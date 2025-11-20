const MULTIPLAYER_TICK_INTERVAL = 1 / 60;
const MULTIPLAYER_BROADCAST_INTERVAL = 1 / 15;
const MULTIPLAYER_COUNTDOWN_DURATION = 3;

export class MultiplayerServer {
  constructor(manager, dependencies) {
    this.manager = manager;
    this.deps = dependencies;
    const {
      NORMAL_FISH_TIME_LIMIT,
      WORLD_SIZE,
      FISH_BASE_SIZE,
      FISH_SWIM_SPEED,
      POWER_UP_BASE_SIZE,
      createSeededRng
    } = this.deps;

    this.state = {
      roomName: manager.roomName,
      phase: "lobby",
      countdown: 0,
      remaining: NORMAL_FISH_TIME_LIMIT,
      message: "Ожидаем игроков",
      players: [],
      walls: [],
      mines: [],
      fish: {
        x: WORLD_SIZE / 2,
        y: WORLD_SIZE / 2,
        size: FISH_BASE_SIZE,
        alive: false,
        type: "normal",
        direction: 1
      },
      powerUp: {
        x: 0,
        y: 0,
        size: POWER_UP_BASE_SIZE,
        active: false,
        remaining: 0
      },
      goldenChainActive: false,
      statusEffect: null,
      winnerId: null,
      serverTime: Date.now()
    };
    this.inputs = {};
    this.randomSeed = Math.floor(Math.random() * 1_000_000_000);
    this.random = createSeededRng(this.randomSeed);
    this.intervalId = null;
    this.lastUpdate = performance.now();
    this.broadcastAccumulator = 0;
  }

  destroy() {
    this.stopTicking();
  }

  startTicking() {
    this.stopTicking();
    this.lastUpdate = performance.now();
    this.intervalId = setInterval(() => this.tick(), MULTIPLAYER_TICK_INTERVAL * 1000);
  }

  stopTicking() {
    if (this.intervalId) {
      clearInterval(this.intervalId);
      this.intervalId = null;
    }
  }

  syncPresence(presenceState) {
    const entries = [];
    Object.values(presenceState || {}).forEach((states) => {
      states.forEach((meta) => {
        if (!meta || !meta.playerId) {
          return;
        }
        entries.push(meta);
      });
    });

    entries.sort((a, b) => (a.joinedAt || 0) - (b.joinedAt || 0));
    const presentIds = new Set(entries.map((entry) => entry.playerId));

    entries.forEach((meta) => {
      const sanitizeAppearance = this.deps?.sanitizeAppearance;
      let player = this.state.players.find((item) => item.id === meta.playerId);
      if (!player) {
        player = this.createPlayer(meta);
        this.state.players.push(player);
      }
      player.name = meta.name || player.name;
      if (sanitizeAppearance) {
        player.appearance = sanitizeAppearance(meta.appearance || player.appearance);
      }
    });

    if (this.state.phase === "playing" || this.state.phase === "countdown") {
      this.state.players.forEach((player) => {
        if (!presentIds.has(player.id)) {
          player.alive = false;
        }
      });
    } else {
      this.state.players = this.state.players.filter((player) => presentIds.has(player.id));
    }

    this.updateLobbyMessage();
    this.broadcastState(true);
  }

  createPlayer(meta) {
    const { WORLD_SIZE, CAT_BASE_SIZE, sanitizeAppearance } = this.deps;
    const appearance = sanitizeAppearance ? sanitizeAppearance(meta.appearance) : meta.appearance;
    return {
      id: meta.playerId,
      name: meta.name || "Игрок",
      score: 0,
      ready: false,
      alive: false,
      x: WORLD_SIZE / 2,
      y: WORLD_SIZE / 2,
      size: CAT_BASE_SIZE,
      facing: 1,
      moving: false,
      walkCycle: 0,
      stepAccumulator: 0,
      appearance
    };
  }

  updateLobbyMessage() {
    if (this.state.phase !== "lobby") {
      return;
    }
    if (this.state.players.length === 0) {
      this.state.message = "Ожидаем игроков";
      return;
    }
    const readyCount = this.state.players.filter((player) => player.ready).length;
    if (readyCount === this.state.players.length) {
      this.state.message = "Все игроки готовы";
    } else {
      this.state.message = `Готовы ${readyCount} из ${this.state.players.length}`;
    }
  }

  handlePlayerReady(playerId, ready) {
    const player = this.state.players.find((item) => item.id === playerId);
    if (!player) {
      return;
    }

    if (this.state.phase === "ended") {
      this.state.phase = "lobby";
      this.state.countdown = 0;
      this.state.winnerId = null;
      this.state.fish.alive = false;
      this.state.message = "Ожидаем игроков";
      this.state.statusEffect = null;
      this.state.players.forEach((p) => {
        p.ready = false;
        p.alive = false;
      });
      this.inputs = {};
    }

    if (this.state.phase === "countdown" && !ready) {
      this.state.phase = "lobby";
      this.state.countdown = 0;
      this.state.message = "Ожидаем игроков";
      this.state.statusEffect = null;
      this.state.players.forEach((p) => {
        p.ready = p.id === playerId ? false : p.ready;
      });
      this.broadcastState(true);
      return;
    }

    if (this.state.phase !== "lobby") {
      return;
    }

    player.ready = ready;
    this.updateLobbyMessage();
    const readyCount = this.state.players.filter((p) => p.ready).length;
    if (readyCount === this.state.players.length && this.state.players.length > 0) {
      this.startCountdown();
    } else {
      this.broadcastState(true);
    }
  }

  handlePlayerAppearance(playerId, appearance) {
    const { sanitizeAppearance } = this.deps;
    const player = this.state.players.find((item) => item.id === playerId);
    if (!player || typeof sanitizeAppearance !== "function") {
      return;
    }
    player.appearance = sanitizeAppearance(appearance || player.appearance);
    this.broadcastState(true);
  }

  handlePlayerInput(playerId, vector) {
    const { clamp } = this.deps;
    if (!vector || typeof vector.x !== "number" || typeof vector.y !== "number") {
      return;
    }
    const cappedX = clamp(vector.x, -1.5, 1.5);
    const cappedY = clamp(vector.y, -1.5, 1.5);
    this.inputs[playerId] = { x: cappedX, y: cappedY };
  }

  startCountdown() {
    this.state.phase = "countdown";
    this.state.countdown = MULTIPLAYER_COUNTDOWN_DURATION;
    this.state.message = "Игра скоро начнётся";
    this.broadcastState(true);
  }

  startRound() {
    const { createSeededRng } = this.deps;
    this.assignSpawnPositions();
    this.randomSeed = Math.floor(Math.random() * 1_000_000_000);
    this.random = createSeededRng(this.randomSeed);
    this.resetWorldState();
    this.state.goldenChainActive = false;
    this.spawnFish(true);
    this.state.phase = "playing";
    this.state.message = "Игра началась";
    this.state.winnerId = null;
    this.inputs = {};
    this.broadcastState(true);
  }

  assignSpawnPositions() {
    const { WORLD_SIZE } = this.deps;
    const total = this.state.players.length;
    if (total === 0) {
      return;
    }
    const radius = Math.min(WORLD_SIZE / 2 - 40, 140);
    this.state.players.forEach((player, index) => {
      const angle = (index / total) * Math.PI * 2;
      player.x = WORLD_SIZE / 2 + Math.cos(angle) * radius;
      player.y = WORLD_SIZE / 2 + Math.sin(angle) * radius;
      player.score = 0;
      player.ready = false;
      player.alive = true;
      player.walkCycle = 0;
      player.stepAccumulator = 0;
      player.moving = false;
      player.facing = 1;
    });
  }

  resetWorldState() {
    const { POWER_UP_BASE_SIZE } = this.deps;
    this.state.walls = [];
    this.state.mines = [];
    this.state.statusEffect = null;
    this.state.powerUp = {
      x: 0,
      y: 0,
      size: POWER_UP_BASE_SIZE,
      active: false,
      remaining: 0
    };
  }

  generateWallsLayoutForPlayers(catCells, fishCell) {
    const { buildBlockedGridFromSegments, isPathAvailable, convertSegmentsToWalls } = this.deps;
    if (!catCells || catCells.length === 0 || !fishCell) {
      return null;
    }
    for (let attempt = 0; attempt < 160; attempt += 1) {
      const segments = this.buildRandomWallSegments(catCells, fishCell);
      if (!segments) {
        continue;
      }
      const blockedGrid = buildBlockedGridFromSegments(segments);
      const allReachable = catCells.every((catCell) =>
        isPathAvailable(catCell, fishCell, blockedGrid)
      );
      if (!allReachable) {
        continue;
      }
      const candidateWalls = convertSegmentsToWalls(segments);
      const intersectsAnyPlayer = this.state.players.some((player) =>
        this.entityIntersectsWalls(player, candidateWalls)
      );
      if (intersectsAnyPlayer) {
        continue;
      }
      return candidateWalls;
    }
    return null;
  }

  buildRandomWallSegments(catCells, fishCell) {
    const { MAX_WALL_TOTAL_LENGTH, GRID_SIZE, getCellsForSegment } = this.deps;
    const segments = [];
    const occupied = new Set();
    let totalLength = 0;
    let attempts = 0;
    const fishKey = `${fishCell.row},${fishCell.col}`;
    const catKeySet = new Set(
      (catCells || []).map((cell) => `${cell.row},${cell.col}`)
    );

    while (totalLength < MAX_WALL_TOTAL_LENGTH && attempts < 80) {
      attempts += 1;
      if (segments.length >= 2 && this.random() < 0.35) {
        break;
      }
      const remaining = MAX_WALL_TOTAL_LENGTH - totalLength;
      const maxSegmentLength = Math.min(remaining, 3);
      if (maxSegmentLength <= 0) {
        continue;
      }
      const length = Math.min(
        remaining,
        1 + Math.floor(this.random() * maxSegmentLength)
      );
      const orientation = this.random() < 0.5 ? "horizontal" : "vertical";
      const maxRow = orientation === "horizontal" ? GRID_SIZE - 1 : GRID_SIZE - length;
      const maxCol = orientation === "horizontal" ? GRID_SIZE - length : GRID_SIZE - 1;
      if (maxRow < 0 || maxCol < 0) {
        continue;
      }
      const row = Math.floor(this.random() * (maxRow + 1));
      const col = Math.floor(this.random() * (maxCol + 1));
      const cells = getCellsForSegment(row, col, length, orientation);
      let invalid = false;
      for (const cell of cells) {
        const key = `${cell.row},${cell.col}`;
        if (occupied.has(key) || catKeySet.has(key) || key === fishKey) {
          invalid = true;
          break;
        }
      }
      if (invalid) {
        continue;
      }
      cells.forEach((cell) => occupied.add(`${cell.row},${cell.col}`));
      segments.push({ row, col, length, orientation });
      totalLength += length;
    }

    return segments.length < 2 ? null : segments;
  }

  entityIntersectsWalls(entity, candidateWalls) {
    const { circleIntersectsRect } = this.deps;
    if (!entity || !candidateWalls || candidateWalls.length === 0) {
      return false;
    }
    const radius = (entity.size || 0) / 2 + 2;
    return candidateWalls.some((wall) =>
      circleIntersectsRect(entity.x, entity.y, radius, wall)
    );
  }

  resolvePlayersAfterWallChange() {
    const { resolveEntityWallCollisions } = this.deps;
    this.state.players.forEach((player) => {
      resolveEntityWallCollisions(player, this.state.walls);
      this.clampPlayer(player);
    });
  }

  handlePowerUpAfterWallChange() {
    const { circleIntersectsAnyWall } = this.deps;
    if (
      this.state.powerUp.active &&
      circleIntersectsAnyWall(
        this.state.powerUp.x,
        this.state.powerUp.y,
        this.state.powerUp.size / 2 + 2,
        this.state.walls
      )
    ) {
      this.clearPowerUp();
    }
  }

  generateMines() {
    const { MAX_MINES, MINE_SIZE, MINE_MIN_DISTANCE, WORLD_SIZE } = this.deps;
    const minesResult = [];
    const mineCount = Math.floor(this.random() * (MAX_MINES + 1));
    if (mineCount === 0) {
      return minesResult;
    }
    const radius = MINE_SIZE / 2;
    const margin = radius + MINE_MIN_DISTANCE + 4;
    let attempts = 0;

    while (minesResult.length < mineCount && attempts < 200) {
      attempts += 1;
      const x = margin + this.random() * (WORLD_SIZE - margin * 2);
      const y = margin + this.random() * (WORLD_SIZE - margin * 2);
      if (!this.isMinePositionValid(x, y, radius, minesResult)) {
        continue;
      }
      minesResult.push({ x, y, size: MINE_SIZE });
    }

    return minesResult;
  }

  isMinePositionValid(x, y, radius, existingMines) {
    const { WORLD_SIZE, MINE_MIN_DISTANCE, circleIntersectsAnyWall } = this.deps;
    const safeRadius = radius + MINE_MIN_DISTANCE;

    if (
      x - safeRadius < 0 ||
      y - safeRadius < 0 ||
      x + safeRadius > WORLD_SIZE ||
      y + safeRadius > WORLD_SIZE
    ) {
      return false;
    }

    if (circleIntersectsAnyWall(x, y, safeRadius, this.state.walls)) {
      return false;
    }

    if (this.state.fish.alive) {
      const distanceToFish = Math.hypot(x - this.state.fish.x, y - this.state.fish.y);
      if (distanceToFish <= this.state.fish.size / 2 + safeRadius) {
        return false;
      }
    }

    for (const player of this.state.players) {
      if (!player.alive) {
        continue;
      }
      const distanceToPlayer = Math.hypot(x - player.x, y - player.y);
      if (distanceToPlayer <= player.size / 2 + safeRadius) {
        return false;
      }
    }

    for (const mine of existingMines || []) {
      const distanceToMine = Math.hypot(x - mine.x, y - mine.y);
      if (distanceToMine <= mine.size / 2 + radius + MINE_MIN_DISTANCE) {
        return false;
      }
    }

    return true;
  }

  doesFishCollideAt(xPosition) {
    const { WORLD_SIZE, circleIntersectsAnyWall, circleIntersectsRect } = this.deps;
    const radius = this.state.fish.size / 2;
    if (xPosition - radius < 0 || xPosition + radius > WORLD_SIZE) {
      return true;
    }

    if (circleIntersectsAnyWall(xPosition, this.state.fish.y, radius, this.state.walls)) {
      return true;
    }

    if (this.state.powerUp.active) {
      const half = this.state.powerUp.size / 2;
      const powerUpRect = {
        x: this.state.powerUp.x - half,
        y: this.state.powerUp.y - half,
        width: this.state.powerUp.size,
        height: this.state.powerUp.size
      };
      if (circleIntersectsRect(xPosition, this.state.fish.y, radius, powerUpRect)) {
        return true;
      }
    }

    for (const mine of this.state.mines) {
      const distanceToMine = Math.hypot(xPosition - mine.x, this.state.fish.y - mine.y);
      if (distanceToMine < radius + mine.size / 2) {
        return true;
      }
    }

    return false;
  }

  clearPowerUp() {
    this.state.powerUp.active = false;
    this.state.powerUp.remaining = 0;
    this.state.powerUp.x = 0;
    this.state.powerUp.y = 0;
  }

  updateFishMovement(delta) {
    const { FISH_SWIM_SPEED, clamp } = this.deps;
    if (!this.state.fish.alive) {
      return;
    }

    const swimStep = this.state.fish.direction * FISH_SWIM_SPEED * delta;
    let nextX = this.state.fish.x + swimStep;

    if (this.doesFishCollideAt(nextX)) {
      this.state.fish.direction *= -1;
      nextX = this.state.fish.x + this.state.fish.direction * FISH_SWIM_SPEED * delta;
      if (this.doesFishCollideAt(nextX)) {
        return;
      }
    }

    this.state.fish.x = clamp(
      nextX,
      this.state.fish.size / 2,
      this.deps.WORLD_SIZE - this.state.fish.size / 2
    );
  }

  spawnPowerUp() {
    const { WORLD_SIZE, circleIntersectsAnyWall, POWER_UP_LIFETIME } = this.deps;
    const margin = 36;
    for (let attempt = 0; attempt < 40; attempt += 1) {
      const x = margin + this.random() * (WORLD_SIZE - margin * 2);
      const y = margin + this.random() * (WORLD_SIZE - margin * 2);
      if (!circleIntersectsAnyWall(x, y, this.state.powerUp.size / 2 + 2, this.state.walls)) {
        this.state.powerUp.x = x;
        this.state.powerUp.y = y;
        this.state.powerUp.active = true;
        this.state.powerUp.remaining = POWER_UP_LIFETIME;
        return;
      }
    }
    this.clearPowerUp();
  }

  refreshPowerUp() {
    const { POWER_UP_CHANCE, POWER_UP_LIFETIME } = this.deps;
    if (this.random() < POWER_UP_CHANCE) {
      this.spawnPowerUp();
    } else if (this.state.powerUp.active) {
      this.state.powerUp.remaining = Math.min(this.state.powerUp.remaining, POWER_UP_LIFETIME);
    } else {
      this.clearPowerUp();
    }
  }

  updatePowerUp(delta) {
    if (!this.state.powerUp.active) {
      return false;
    }
    this.state.powerUp.remaining -= delta;
    if (this.state.powerUp.remaining <= 0) {
      this.clearPowerUp();
      return true;
    }
    for (const player of this.state.players) {
      if (!player.alive) {
        continue;
      }
      const distance = Math.hypot(player.x - this.state.powerUp.x, player.y - this.state.powerUp.y);
      if (distance < (player.size + this.state.powerUp.size) / 2) {
        this.clearPowerUp();
        this.applyRandomStatusEffect();
        return true;
      }
    }
    return false;
  }

  getSpeedMultiplier() {
    const effectType = this.state.statusEffect?.type;
    if (effectType === "speedUp") {
      return 2;
    }
    if (effectType === "speedDown") {
      return 1 / 1.5;
    }
    return 1;
  }

  getBaseTimeLimit(fishType) {
    const { GOLDEN_FISH_TIME_LIMIT, NORMAL_FISH_TIME_LIMIT } = this.deps;
    return fishType === "golden" ? GOLDEN_FISH_TIME_LIMIT : NORMAL_FISH_TIME_LIMIT;
  }

  getTimeLimitForFish(fishType) {
    const { TIME_INCREASE_LIMIT, TIME_DECREASE_LIMIT } = this.deps;
    const effectType = this.state.statusEffect?.type;
    if (effectType === "timeIncrease") {
      return TIME_INCREASE_LIMIT;
    }
    if (effectType === "timeDecrease") {
      return TIME_DECREASE_LIMIT;
    }
    return this.getBaseTimeLimit(fishType);
  }

  applyStatusEffect(effectType) {
    const { POWER_UP_DURATION, TIME_INCREASE_LIMIT, TIME_DECREASE_LIMIT } = this.deps;
    if (!effectType) {
      this.state.statusEffect = null;
      return false;
    }
    this.state.statusEffect = {
      type: effectType,
      remaining: POWER_UP_DURATION
    };
    if (effectType === "timeIncrease") {
      this.state.remaining = Math.max(this.state.remaining, TIME_INCREASE_LIMIT);
    } else if (effectType === "timeDecrease") {
      this.state.remaining = Math.min(this.state.remaining, TIME_DECREASE_LIMIT);
    }
    return true;
  }

  applyRandomStatusEffect() {
    const types = this.deps.STATUS_EFFECT_TYPES || [];
    if (!types.length) {
      this.state.statusEffect = null;
      return false;
    }
    const index = Math.floor(this.random() * types.length);
    const type = types[index];
    return this.applyStatusEffect(type);
  }

  updateStatusEffect(delta) {
    if (!this.state.statusEffect) {
      return false;
    }
    this.state.statusEffect.remaining -= delta;
    if (this.state.statusEffect.remaining > 0) {
      return false;
    }
    const endedType = this.state.statusEffect.type;
    this.state.statusEffect = null;
    if (endedType === "timeIncrease" || endedType === "timeDecrease") {
      const baseLimit = this.getBaseTimeLimit(this.state.fish.type);
      if (endedType === "timeIncrease") {
        this.state.remaining = Math.min(this.state.remaining, baseLimit);
      } else {
        this.state.remaining = Math.max(this.state.remaining, baseLimit);
      }
    }
    return true;
  }

  tick() {
    const now = performance.now();
    const delta = (now - this.lastUpdate) / 1000;
    this.lastUpdate = now;

    if (this.state.phase === "countdown") {
      this.updateCountdown(delta);
    } else if (this.state.phase === "playing") {
      this.updatePlaying(delta);
    }

    this.broadcastAccumulator += delta;
    if (this.broadcastAccumulator >= MULTIPLAYER_BROADCAST_INTERVAL) {
      this.broadcastAccumulator = 0;
      this.broadcastState();
    }
  }

  updateCountdown(delta) {
    this.state.countdown = Math.max(0, (this.state.countdown || 0) - delta);
    if (this.state.countdown <= 0) {
      this.startRound();
    }
  }

  updatePlaying(delta) {
    const {
      CAT_BASE_SPEED,
      WALK_FREQUENCY,
      resolveEntityWallCollisions,
      GOLDEN_FISH_POINTS,
      NORMAL_FISH_POINTS
    } = this.deps;
    this.state.remaining = Math.max(0, (this.state.remaining || 0) - delta);
    let stateChanged = false;

    if (this.updateStatusEffect(delta)) {
      stateChanged = true;
    }

    const speedMultiplier = this.getSpeedMultiplier();

    if (this.updatePowerUp(delta)) {
      stateChanged = true;
    }

    this.updateFishMovement(delta);

    this.state.players.forEach((player) => {
      if (!player.alive) {
        return;
      }
      const input = this.inputs[player.id] || { x: 0, y: 0 };
      const length = Math.hypot(input.x, input.y);
      const hasInput = length > 0.001;
      if (hasInput) {
        const cappedMagnitude = Math.min(length, 1);
        const normalizedX = input.x / (length || 1);
        const normalizedY = input.y / (length || 1);
        const distance = CAT_BASE_SPEED * speedMultiplier * delta * cappedMagnitude;
        player.x += normalizedX * distance;
        player.y += normalizedY * distance;
        player.moving = true;
        if (Math.abs(normalizedX) > 0.1) {
          player.facing = normalizedX >= 0 ? 1 : -1;
        }
        const walkIncrement = delta * WALK_FREQUENCY * cappedMagnitude * speedMultiplier;
        player.walkCycle = (player.walkCycle + walkIncrement) % 1;
        player.stepAccumulator += walkIncrement;
        while (player.stepAccumulator >= 0.5) {
          player.stepAccumulator -= 0.5;
        }
      } else {
        player.moving = false;
        player.walkCycle = 0;
        player.stepAccumulator = 0;
      }

      resolveEntityWallCollisions(player, this.state.walls);
      this.clampPlayer(player);

      if (this.state.fish.alive) {
        const distanceToFish = Math.hypot(player.x - this.state.fish.x, player.y - this.state.fish.y);
        if (distanceToFish < (player.size + this.state.fish.size) / 2) {
          const isGolden = this.state.fish.type === "golden";
          player.score += isGolden ? GOLDEN_FISH_POINTS : NORMAL_FISH_POINTS;
          this.state.goldenChainActive = isGolden;
          this.spawnFish();
          stateChanged = true;
        }
      }

      for (const mine of this.state.mines) {
        if (Math.hypot(player.x - mine.x, player.y - mine.y) < (player.size + mine.size) / 2) {
          player.alive = false;
          player.moving = false;
          stateChanged = true;
          break;
        }
      }
    });

    const aliveCount = this.countAlivePlayers();
    if (aliveCount <= 1) {
      const winner = this.determineWinnerId();
      this.endGame(winner ? "Победил последний выживший кот" : "Никто не выжил", winner);
      return;
    }

    if (this.state.remaining <= 0) {
      if (this.state.fish.type === "golden") {
        this.state.fish.alive = false;
        this.state.goldenChainActive = false;
        this.spawnFish(true);
        stateChanged = true;
      } else {
        const winner = this.determineWinnerId();
        this.endGame(winner ? "Рыбка уплыла" : "Рыбка уплыла, победителя нет", winner);
        return;
      }
    }

    if (stateChanged) {
      this.broadcastState(true);
    }
  }

  clampPlayer(player) {
    const { clamp, WORLD_SIZE } = this.deps;
    player.x = clamp(player.x, player.size / 2, WORLD_SIZE - player.size / 2);
    player.y = clamp(player.y, player.size / 2, WORLD_SIZE - player.size / 2);
  }

  spawnFish(forceNormal = false) {
    const {
      WORLD_SIZE,
      GOLDEN_FISH_CHANCE,
      positionToGridCell,
      circleIntersectsAnyWall
    } = this.deps;
    const margin = 30;
    const fishState = this.state.fish;
    const shouldSpawnGolden =
      !forceNormal && (this.state.goldenChainActive || this.random() < GOLDEN_FISH_CHANCE);
    fishState.type = shouldSpawnGolden ? "golden" : "normal";
    this.state.goldenChainActive = shouldSpawnGolden;
    fishState.direction = this.random() < 0.5 ? -1 : 1;

    const alivePlayers = this.state.players.filter((player) => player.alive);
    const referencePlayers =
      alivePlayers.length > 0
        ? alivePlayers
        : this.state.players.length > 0
        ? [this.state.players[0]]
        : [{ x: WORLD_SIZE / 2, y: WORLD_SIZE / 2 }];
    const catCells = referencePlayers.map((player) => positionToGridCell(player.x, player.y));

    let placed = false;
    for (let attempt = 0; attempt < 200; attempt += 1) {
      const x = margin + this.random() * (WORLD_SIZE - margin * 2);
      const y = margin + this.random() * (WORLD_SIZE - margin * 2);
      const fishCell = positionToGridCell(x, y);
      if (catCells.some((cell) => cell.row === fishCell.row && cell.col === fishCell.col)) {
        continue;
      }
      const candidateWalls = this.generateWallsLayoutForPlayers(catCells, fishCell);
      if (!candidateWalls) {
        continue;
      }
      if (circleIntersectsAnyWall(x, y, fishState.size / 2 + 2, candidateWalls)) {
        continue;
      }
      this.state.walls = candidateWalls;
      this.handlePowerUpAfterWallChange();
      this.resolvePlayersAfterWallChange();
      fishState.x = x;
      fishState.y = y;
      fishState.alive = true;
      this.state.mines = this.generateMines();
      this.refreshPowerUp();
      placed = true;
      break;
    }

    if (!placed) {
      this.state.walls = [];
      this.handlePowerUpAfterWallChange();
      this.resolvePlayersAfterWallChange();
      fishState.x = WORLD_SIZE / 2;
      fishState.y = WORLD_SIZE / 2;
      fishState.alive = true;
      this.state.mines = this.generateMines();
      this.refreshPowerUp();
    }

    this.state.remaining = this.getTimeLimitForFish(fishState.type);
  }

  countAlivePlayers() {
    return this.state.players.filter((player) => player.alive).length;
  }

  determineWinnerId() {
    const alivePlayers = this.state.players.filter((player) => player.alive);
    if (alivePlayers.length === 1) {
      return alivePlayers[0].id;
    }
    if (alivePlayers.length === 0) {
      const sorted = [...this.state.players].sort((a, b) => b.score - a.score);
      return sorted[0] ? sorted[0].id : null;
    }
    return null;
  }

  endGame(message, winnerId) {
    this.state.phase = "ended";
    this.state.message = message;
    this.state.winnerId = winnerId || null;
    this.state.countdown = 0;
    this.state.fish.alive = false;
    this.state.statusEffect = null;
    this.broadcastState(true);
  }

  broadcastState(force = false) {
    if (!force && this.state.phase === "lobby") {
      return;
    }
    this.state.serverTime = Date.now();
    this.manager.broadcastState(this.buildStatePayload());
  }

  buildStatePayload() {
    const sanitizeAppearance = this.deps?.sanitizeAppearance;
    return {
      roomName: this.state.roomName,
      phase: this.state.phase,
      countdown: this.state.countdown,
      remaining: this.state.remaining,
      message: this.state.message,
      winnerId: this.state.winnerId,
      goldenChainActive: this.state.goldenChainActive,
      statusEffect: this.state.statusEffect
        ? { type: this.state.statusEffect.type, remaining: this.state.statusEffect.remaining }
        : null,
      fish: { ...this.state.fish },
      powerUp: { ...this.state.powerUp },
      walls: this.state.walls.map((wall) => ({ ...wall })),
      mines: this.state.mines.map((mine) => ({ ...mine })),
      players: this.state.players.map((player) => ({
        id: player.id,
        name: player.name,
        score: player.score,
        ready: player.ready,
        alive: player.alive,
        x: player.x,
        y: player.y,
        size: player.size,
        facing: player.facing,
        moving: player.moving,
        walkCycle: player.walkCycle,
        appearance: sanitizeAppearance
          ? sanitizeAppearance(player.appearance)
          : player.appearance
      })),
      serverTime: Date.now()
    };
  }
}
