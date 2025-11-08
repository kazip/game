const canvas = document.getElementById("game");
const ctx = canvas.getContext("2d");

const WORLD_SIZE = 500;
const MIN_BOARD_SIZE = 260;
const gameContainer = document.querySelector(".game-container");

const cat = {
  x: WORLD_SIZE / 2,
  y: WORLD_SIZE / 2,
  size: 36,
  speed: 180, // pixels per second
  facing: 1,
  moving: false,
  walkCycle: 0,
  stepAccumulator: 0
};

const WALK_FREQUENCY = 4; // steps per second while the cat is moving

const fish = {
  x: 0,
  y: 0,
  size: 28,
  alive: false
};

const keys = new Set();
let score = 0;
let remaining = 10;
let lastTimestamp = 0;
let gameOver = false;
const messageEl = document.getElementById("message");
const restartBtn = document.getElementById("restart");
const scoreEl = document.getElementById("score");
const timerEl = document.getElementById("timer");
const controlsContainer = document.querySelector(".controls");
const controlButtons = document.querySelectorAll(".control-btn");

const pointerState = new Map();
const directionToKey = {
  up: "ArrowUp",
  down: "ArrowDown",
  left: "ArrowLeft",
  right: "ArrowRight"
};
const directionKeys = new Set(Object.values(directionToKey));

class SoundManager {
  constructor() {
    this.context = null;
    this.masterGain = null;
    this.musicGain = null;
    this.sfxGain = null;
    this.enabled = false;
    this.padOscillators = [];
    this.musicLoopId = null;
    this.lastStepTime = 0;
    this.loopDuration = 12;
    this.rhythmGain = null;
    this.noiseBuffer = null;
  }

  init() {
    if (this.enabled) {
      return;
    }

    const AudioContext = window.AudioContext || window.webkitAudioContext;
    if (typeof AudioContext !== "function") {
      return;
    }

    this.context = new AudioContext();
    this.masterGain = this.context.createGain();
    this.masterGain.gain.value = 0.7;
    this.masterGain.connect(this.context.destination);

    this.musicGain = this.context.createGain();
    this.musicGain.gain.value = 0;
    this.musicGain.connect(this.masterGain);

    this.sfxGain = this.context.createGain();
    this.sfxGain.gain.value = 1;
    this.sfxGain.connect(this.masterGain);

    this.enabled = true;
    this.startMusic();
  }

  startMusic() {
    if (!this.enabled || this.musicLoopId) {
      return;
    }

    const ctx = this.context;
    const now = ctx.currentTime;
    this.createPadLayer(now);
    this.musicGain.gain.linearRampToValueAtTime(0.45, now + 3.5);
    this.playMusicSegment(now);
    this.musicLoopId = window.setInterval(() => {
      this.playMusicSegment(ctx.currentTime);
    }, this.loopDuration * 1000);
  }

  createPadLayer(startTime) {
    if (this.padOscillators.length > 0) {
      return;
    }

    const ctx = this.context;
    const padGain = ctx.createGain();
    padGain.gain.setValueAtTime(0.0001, startTime);
    padGain.gain.exponentialRampToValueAtTime(0.3, startTime + 5);
    padGain.connect(this.musicGain);

    const padNotes = [261.63, 329.63, 392.0, 523.25];
    padNotes.forEach((frequency, index) => {
      const osc = ctx.createOscillator();
      osc.type = "triangle";
      osc.frequency.setValueAtTime(frequency, startTime);
      const gain = ctx.createGain();
      gain.gain.value = 0.12 / padNotes.length;
      osc.connect(gain);
      gain.connect(padGain);
      osc.start(startTime + index * 0.08);
      this.padOscillators.push({ osc, gain });
    });
  }

  ensureNoiseBuffer(duration = 0.35) {
    if (this.noiseBuffer) {
      return this.noiseBuffer;
    }
    const ctx = this.context;
    const length = Math.floor(ctx.sampleRate * duration);
    const buffer = ctx.createBuffer(1, length, ctx.sampleRate);
    const data = buffer.getChannelData(0);
    for (let i = 0; i < length; i++) {
      const fade = 1 - i / length;
      data[i] = (Math.random() * 2 - 1) * fade * 0.8;
    }
    this.noiseBuffer = buffer;
    return buffer;
  }

  playMusicSegment(startTime) {
    if (!this.enabled) {
      return;
    }

    this.playBassline(startTime);
    this.playLead(startTime + 0.25);
    this.playPercussion(startTime);
  }

  playBassline(startTime) {
    const ctx = this.context;
    const baseFrequency = 130.81; // C3
    const pattern = [0, -5, -3, -7, -2, -7, -9, -12];
    const noteDuration = 0.9;

    pattern.forEach((interval, index) => {
      const time = startTime + index * noteDuration;
      const osc = ctx.createOscillator();
      osc.type = "sawtooth";
      const freq = baseFrequency * Math.pow(2, interval / 12);
      osc.frequency.setValueAtTime(freq, time);
      osc.frequency.exponentialRampToValueAtTime(freq * 0.98, time + noteDuration * 0.9);

      const gain = ctx.createGain();
      gain.gain.setValueAtTime(0.0001, time);
      gain.gain.exponentialRampToValueAtTime(0.16, time + 0.05);
      gain.gain.exponentialRampToValueAtTime(0.0001, time + noteDuration * 0.95);

      osc.connect(gain);
      gain.connect(this.musicGain);
      osc.start(time);
      osc.stop(time + noteDuration);
    });
  }

  playLead(startTime) {
    const ctx = this.context;
    const melody = [0, 4, 7, 12, 14, 12, 7, 4, 2, 4, 7, 9];
    const spacing = 0.4;

    melody.forEach((interval, index) => {
      const time = startTime + index * spacing;
      const osc = ctx.createOscillator();
      osc.type = "triangle";
      const frequency = 261.63 * Math.pow(2, interval / 12);
      osc.frequency.setValueAtTime(frequency, time);

      const gain = ctx.createGain();
      gain.gain.setValueAtTime(0.0001, time);
      gain.gain.exponentialRampToValueAtTime(0.08, time + 0.05);
      gain.gain.exponentialRampToValueAtTime(0.0001, time + spacing * 0.9);

      const vibrato = ctx.createOscillator();
      vibrato.type = "sine";
      vibrato.frequency.setValueAtTime(6, time);
      const vibratoGain = ctx.createGain();
      vibratoGain.gain.setValueAtTime(3, time);
      vibrato.connect(vibratoGain);
      vibratoGain.connect(osc.frequency);

      osc.connect(gain);
      gain.connect(this.musicGain);
      osc.start(time);
      osc.stop(time + spacing * 1.1);
      vibrato.start(time);
      vibrato.stop(time + spacing * 1.1);
    });
  }

  playPercussion(startTime) {
    const ctx = this.context;
    const beatSpacing = 0.6;
    const hits = 8;

    if (!this.rhythmGain) {
      this.rhythmGain = ctx.createGain();
      this.rhythmGain.gain.value = 0.6;
      this.rhythmGain.connect(this.musicGain);
    }

    for (let i = 0; i < hits; i++) {
      const time = startTime + i * beatSpacing;
      const bufferSource = ctx.createBufferSource();
      bufferSource.buffer = this.ensureNoiseBuffer();

      const filter = ctx.createBiquadFilter();
      filter.type = "bandpass";
      filter.frequency.setValueAtTime(i % 2 === 0 ? 1400 : 900, time);
      filter.Q.setValueAtTime(6, time);

      const gain = ctx.createGain();
      gain.gain.setValueAtTime(0.0001, time);
      gain.gain.exponentialRampToValueAtTime(i % 2 === 0 ? 0.45 : 0.25, time + 0.02);
      gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.25);

      bufferSource.connect(filter);
      filter.connect(gain);
      gain.connect(this.rhythmGain);
      bufferSource.start(time);
      bufferSource.stop(time + 0.3);
    }
  }

  playCatch() {
    if (!this.enabled) {
      return;
    }
    const ctx = this.context;
    const now = ctx.currentTime;
    const clickOsc = ctx.createOscillator();
    clickOsc.type = "sine";
    clickOsc.frequency.setValueAtTime(820, now);
    clickOsc.frequency.exponentialRampToValueAtTime(420, now + 0.18);

    const clickGain = ctx.createGain();
    clickGain.gain.setValueAtTime(0.0001, now);
    clickGain.gain.exponentialRampToValueAtTime(0.32, now + 0.04);
    clickGain.gain.exponentialRampToValueAtTime(0.0001, now + 0.3);

    clickOsc.connect(clickGain);
    clickGain.connect(this.sfxGain);

    const splash = ctx.createBufferSource();
    splash.buffer = this.ensureNoiseBuffer(0.25);
    const splashFilter = ctx.createBiquadFilter();
    splashFilter.type = "bandpass";
    splashFilter.frequency.setValueAtTime(900, now);
    splashFilter.Q.setValueAtTime(5.5, now);

    const splashGain = ctx.createGain();
    splashGain.gain.setValueAtTime(0.0001, now);
    splashGain.gain.exponentialRampToValueAtTime(0.6, now + 0.03);
    splashGain.gain.exponentialRampToValueAtTime(0.0001, now + 0.35);

    splash.connect(splashFilter);
    splashFilter.connect(splashGain);
    splashGain.connect(this.sfxGain);

    clickOsc.start(now);
    clickOsc.stop(now + 0.35);
    splash.start(now);
    splash.stop(now + 0.35);
  }

  playStep() {
    if (!this.enabled || !this.sfxGain) {
      return;
    }

    const ctx = this.context;
    const now = ctx.currentTime;
    if (now - this.lastStepTime < 0.18) {
      return;
    }
    this.lastStepTime = now;

    const osc = ctx.createOscillator();
    osc.type = "sine";
    osc.frequency.setValueAtTime(180, now);
    osc.frequency.exponentialRampToValueAtTime(120, now + 0.25);

    const gain = ctx.createGain();
    gain.gain.setValueAtTime(0.0001, now);
    gain.gain.exponentialRampToValueAtTime(0.22, now + 0.04);
    gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.32);

    osc.connect(gain);
    gain.connect(this.sfxGain);
    osc.start(now);
    osc.stop(now + 0.35);
  }
}

const soundManager = new SoundManager();

function ensureAudioActive() {
  soundManager.init();
  if (soundManager.context && soundManager.context.state === "suspended") {
    soundManager.context.resume();
  }
}

function normalizeKey(key) {
  if (key.startsWith("Arrow")) {
    return key;
  }
  return key.toLowerCase();
}

function clamp(value, min, max) {
  return Math.min(Math.max(value, min), max);
}

function updateBoardSize() {
  if (!gameContainer) {
    return;
  }

  const viewportWidth = window.innerWidth || WORLD_SIZE;
  const containerWidth = gameContainer.clientWidth || viewportWidth;
  const paddedViewportWidth = Math.max(viewportWidth - 32, 160);
  const widthLimit = Math.min(containerWidth, paddedViewportWidth, WORLD_SIZE);

  if (widthLimit <= 0) {
    document.documentElement.style.setProperty("--board-size", `${MIN_BOARD_SIZE}px`);
    return;
  }

  const viewportHeight = window.innerHeight || WORLD_SIZE;
  const containerTop = gameContainer.getBoundingClientRect().top;
  const controlsVisible =
    controlsContainer && window.getComputedStyle(controlsContainer).display !== "none";
  const controlsHeight = controlsVisible ? controlsContainer.offsetHeight : 0;
  const verticalPadding = 180 + controlsHeight;
  const heightLimitRaw = viewportHeight - containerTop - verticalPadding;
  const minimumHeightAllowance = Math.min(MIN_BOARD_SIZE, widthLimit);
  const heightLimit = Math.min(
    Math.max(heightLimitRaw, minimumHeightAllowance),
    WORLD_SIZE
  );

  let boardSize = Math.min(widthLimit, heightLimit);
  if (widthLimit >= MIN_BOARD_SIZE && heightLimit >= MIN_BOARD_SIZE) {
    boardSize = Math.max(boardSize, MIN_BOARD_SIZE);
  }

  document.documentElement.style.setProperty("--board-size", `${boardSize}px`);
}

function spawnFish() {
  const margin = 30;
  fish.x = margin + Math.random() * (WORLD_SIZE - margin * 2);
  fish.y = margin + Math.random() * (WORLD_SIZE - margin * 2);
  fish.alive = true;
  remaining = 10;
}

function resetGame() {
  updateBoardSize();
  score = 0;
  scoreEl.textContent = score;
  gameOver = false;
  messageEl.textContent = "";
  restartBtn.disabled = true;
  cat.x = WORLD_SIZE / 2;
  cat.y = WORLD_SIZE / 2;
  cat.moving = false;
  cat.walkCycle = 0;
  cat.stepAccumulator = 0;
  cat.facing = 1;
  spawnFish();
  lastTimestamp = performance.now();
  requestAnimationFrame(loop);
}

function endGame(reason) {
  gameOver = true;
  fish.alive = false;
  restartBtn.disabled = false;
  messageEl.textContent = reason + ` Результат: ${score}.`;
  updateBoardSize();
}

function update(delta) {
  const horizontalInput =
    (keys.has("ArrowRight") || keys.has("d") ? 1 : 0) -
    (keys.has("ArrowLeft") || keys.has("a") ? 1 : 0);
  const verticalInput =
    (keys.has("ArrowDown") || keys.has("s") ? 1 : 0) -
    (keys.has("ArrowUp") || keys.has("w") ? 1 : 0);

  const hasInput = horizontalInput !== 0 || verticalInput !== 0;
  if (hasInput) {
    const length = Math.hypot(horizontalInput, verticalInput) || 1;
    const normalizedX = horizontalInput / length;
    const normalizedY = verticalInput / length;
    const distance = cat.speed * delta;
    cat.x += normalizedX * distance;
    cat.y += normalizedY * distance;
    cat.moving = true;
    if (Math.abs(normalizedX) > 0.1) {
      cat.facing = normalizedX >= 0 ? 1 : -1;
    }
    const walkIncrement = delta * WALK_FREQUENCY;
    cat.walkCycle = (cat.walkCycle + walkIncrement) % 1;
    cat.stepAccumulator += walkIncrement;
    while (cat.stepAccumulator >= 0.5) {
      cat.stepAccumulator -= 0.5;
      soundManager.playStep();
    }
  } else {
    cat.moving = false;
    cat.walkCycle = 0;
    cat.stepAccumulator = 0;
  }

  cat.x = clamp(cat.x, cat.size / 2, WORLD_SIZE - cat.size / 2);
  cat.y = clamp(cat.y, cat.size / 2, WORLD_SIZE - cat.size / 2);

  if (fish.alive) {
    const dx = cat.x - fish.x;
    const dy = cat.y - fish.y;
    const distanceToFish = Math.hypot(dx, dy);
    if (distanceToFish < (cat.size + fish.size) / 2) {
      score += 1;
      scoreEl.textContent = score;
      spawnFish();
      soundManager.playCatch();
    }
  }

  remaining -= delta;
  const displayTime = Math.max(remaining, 0);
  timerEl.textContent = displayTime.toFixed(1);
  if (remaining <= 0) {
    endGame("Котик не успел поймать рыбку!");
  }
}

function drawCat() {
  ctx.save();
  ctx.translate(cat.x, cat.y);
  ctx.scale(cat.facing, 1);

  const cycle = cat.walkCycle * Math.PI * 2;
  const bobbing = cat.moving ? Math.cos(cycle) * 2 : 0;

  // Tail
  ctx.save();
  ctx.translate(-cat.size * 0.45, -cat.size * 0.1 + bobbing * 0.2);
  const tailSwing = cat.moving ? Math.sin(cycle + Math.PI / 2) * 8 : 0;
  ctx.rotate((tailSwing * Math.PI) / 180);
  ctx.fillStyle = "#ffb347";
  ctx.beginPath();
  ctx.ellipse(0, 0, cat.size * 0.35, cat.size * 0.12, 0, 0, Math.PI * 2);
  ctx.fill();
  ctx.restore();

  ctx.translate(0, bobbing);

  // Back legs
  ctx.fillStyle = "#f2a73a";
  for (let i = -1; i <= 1; i += 2) {
    const swing = cat.moving ? Math.sin(cycle + (i < 0 ? 0 : Math.PI)) * 4 : 0;
    ctx.beginPath();
    ctx.ellipse(i * cat.size * 0.23 + swing * 0.2, cat.size * 0.42, cat.size * 0.18, cat.size * 0.14, 0, 0, Math.PI * 2);
    ctx.fill();
  }

  // Body
  ctx.fillStyle = "#ffb347";
  ctx.beginPath();
  ctx.ellipse(0, 0, cat.size / 2, cat.size / 2.3, 0, 0, Math.PI * 2);
  ctx.fill();

  // Belly
  ctx.fillStyle = "#ffd59c";
  ctx.beginPath();
  ctx.ellipse(0, cat.size * 0.1, cat.size * 0.32, cat.size * 0.28, 0, 0, Math.PI * 2);
  ctx.fill();

  // Front legs
  ctx.fillStyle = "#ffb347";
  for (let i = -1; i <= 1; i += 2) {
    const phase = i < 0 ? Math.PI : 0;
    const swing = cat.moving ? Math.sin(cycle + phase) * 4 : 0;
    ctx.beginPath();
    ctx.ellipse(i * cat.size * 0.25 - swing * 0.2, cat.size * 0.44, cat.size * 0.16, cat.size * 0.15, 0, 0, Math.PI * 2);
    ctx.fill();
  }

  // Head
  ctx.save();
  ctx.translate(cat.size * 0.26, -cat.size * 0.1);
  ctx.fillStyle = "#ffb347";
  ctx.beginPath();
  ctx.ellipse(0, 0, cat.size * 0.34, cat.size * 0.3, 0, 0, Math.PI * 2);
  ctx.fill();

  // Ears
  ctx.fillStyle = "#ffb347";
  ctx.beginPath();
  ctx.moveTo(-cat.size * 0.18, -cat.size * 0.22);
  ctx.lineTo(-cat.size * 0.08, -cat.size * 0.42);
  ctx.lineTo(0, -cat.size * 0.18);
  ctx.closePath();
  ctx.fill();

  ctx.beginPath();
  ctx.moveTo(cat.size * 0.05, -cat.size * 0.18);
  ctx.lineTo(cat.size * 0.18, -cat.size * 0.4);
  ctx.lineTo(cat.size * 0.2, -cat.size * 0.12);
  ctx.closePath();
  ctx.fill();

  // Eyes
  ctx.fillStyle = "#14365d";
  ctx.beginPath();
  ctx.ellipse(-cat.size * 0.04, -cat.size * 0.05, cat.size * 0.07, cat.size * 0.09, 0, 0, Math.PI * 2);
  ctx.ellipse(cat.size * 0.14, -cat.size * 0.05, cat.size * 0.07, cat.size * 0.09, 0, 0, Math.PI * 2);
  ctx.fill();

  // Muzzle
  ctx.fillStyle = "#ffe5b4";
  ctx.beginPath();
  ctx.arc(cat.size * 0.05, cat.size * 0.05, cat.size * 0.14, 0, Math.PI * 2);
  ctx.fill();

  // Nose and mouth
  ctx.fillStyle = "#ff6f61";
  ctx.beginPath();
  ctx.moveTo(cat.size * 0.05, cat.size * 0.0);
  ctx.lineTo(cat.size * 0.02, cat.size * 0.05);
  ctx.lineTo(cat.size * 0.08, cat.size * 0.05);
  ctx.closePath();
  ctx.fill();

  ctx.strokeStyle = "#ff6f61";
  ctx.lineWidth = 1.8;
  ctx.beginPath();
  ctx.moveTo(cat.size * 0.05, cat.size * 0.05);
  ctx.lineTo(cat.size * 0.05, cat.size * 0.09);
  ctx.moveTo(cat.size * 0.05, cat.size * 0.09);
  ctx.bezierCurveTo(cat.size * 0.0, cat.size * 0.12, -cat.size * 0.02, cat.size * 0.16, cat.size * 0.02, cat.size * 0.17);
  ctx.moveTo(cat.size * 0.05, cat.size * 0.09);
  ctx.bezierCurveTo(cat.size * 0.11, cat.size * 0.12, cat.size * 0.12, cat.size * 0.16, cat.size * 0.08, cat.size * 0.17);
  ctx.stroke();

  // Whiskers
  ctx.strokeStyle = "rgba(20, 54, 93, 0.7)";
  ctx.lineWidth = 1.4;
  ctx.beginPath();
  ctx.moveTo(-cat.size * 0.05, cat.size * 0.02);
  ctx.lineTo(-cat.size * 0.26, -cat.size * 0.03);
  ctx.moveTo(-cat.size * 0.04, cat.size * 0.07);
  ctx.lineTo(-cat.size * 0.24, cat.size * 0.12);
  ctx.moveTo(cat.size * 0.12, cat.size * 0.02);
  ctx.lineTo(cat.size * 0.32, -cat.size * 0.03);
  ctx.moveTo(cat.size * 0.13, cat.size * 0.07);
  ctx.lineTo(cat.size * 0.3, cat.size * 0.12);
  ctx.stroke();

  ctx.restore();

  ctx.restore();
}

function drawFish() {
  if (!fish.alive) return;
  ctx.save();
  ctx.translate(fish.x, fish.y);
  ctx.fillStyle = "#5cc8ff";
  ctx.beginPath();
  ctx.ellipse(0, 0, fish.size / 2, fish.size / 3, 0, 0, Math.PI * 2);
  ctx.fill();

  ctx.beginPath();
  ctx.moveTo(-fish.size / 2, 0);
  ctx.lineTo(-fish.size / 2 - 10, -fish.size / 3);
  ctx.lineTo(-fish.size / 2 - 10, fish.size / 3);
  ctx.closePath();
  ctx.fill();

  ctx.fillStyle = "#14365d";
  ctx.beginPath();
  ctx.arc(fish.size / 4, -fish.size / 6, 3, 0, Math.PI * 2);
  ctx.fill();
  ctx.restore();
}

function prepareCanvasForFrame() {
  const dpr = window.devicePixelRatio || 1;
  const targetSize = Math.round(WORLD_SIZE * dpr);
  if (canvas.width !== targetSize || canvas.height !== targetSize) {
    canvas.width = targetSize;
    canvas.height = targetSize;
  }
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, WORLD_SIZE, WORLD_SIZE);
}

function loop(timestamp) {
  if (gameOver) return;

  const delta = (timestamp - lastTimestamp) / 1000;
  lastTimestamp = timestamp;
  prepareCanvasForFrame();
  update(delta);
  drawFish();
  drawCat();
  requestAnimationFrame(loop);
}

window.addEventListener("keydown", (event) => {
  const key = normalizeKey(event.key);
  if (directionKeys.has(key)) {
    event.preventDefault();
  }
  ensureAudioActive();
  keys.add(key);
});

window.addEventListener("keyup", (event) => {
  const key = normalizeKey(event.key);
  keys.delete(key);
});

restartBtn.addEventListener("click", () => {
  if (!gameOver) return;
  ensureAudioActive();
  resetGame();
});

function activatePointer(button, key, pointerId) {
  keys.add(key);
  button.classList.add("active");
  pointerState.set(pointerId, { key, button });
}

function deactivatePointer(pointerId) {
  const state = pointerState.get(pointerId);
  if (!state) return;
  keys.delete(state.key);
  state.button.classList.remove("active");
  pointerState.delete(pointerId);
}

controlButtons.forEach((button) => {
  button.addEventListener("pointerdown", (event) => {
    const direction = button.dataset.direction;
    const key = directionToKey[direction];
    if (!key) return;
    event.preventDefault();
    ensureAudioActive();
    if (button.setPointerCapture) {
      button.setPointerCapture(event.pointerId);
    }
    activatePointer(button, key, event.pointerId);
  });

  button.addEventListener("pointerup", (event) => {
    event.preventDefault();
    if (button.releasePointerCapture) {
      button.releasePointerCapture(event.pointerId);
    }
    deactivatePointer(event.pointerId);
  });

  button.addEventListener("pointercancel", (event) => {
    deactivatePointer(event.pointerId);
  });

  button.addEventListener("lostpointercapture", (event) => {
    deactivatePointer(event.pointerId);
  });

  button.addEventListener("contextmenu", (event) => {
    event.preventDefault();
  });
});

window.addEventListener("blur", () => {
  keys.clear();
  pointerState.forEach(({ button }) => {
    button.classList.remove("active");
  });
  pointerState.clear();
});

window.addEventListener("resize", () => {
  updateBoardSize();
});

window.addEventListener(
  "pointerdown",
  () => {
    ensureAudioActive();
  },
  { once: true }
);

const coarsePointerQuery = typeof window.matchMedia === "function"
  ? window.matchMedia("(pointer: coarse)")
  : null;
if (coarsePointerQuery) {
  if (typeof coarsePointerQuery.addEventListener === "function") {
    coarsePointerQuery.addEventListener("change", updateBoardSize);
  } else if (typeof coarsePointerQuery.addListener === "function") {
    coarsePointerQuery.addListener(updateBoardSize);
  }
}

// Запуск игры при загрузке страницы
resetGame();
