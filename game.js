import { decodeStateFromBase64, encodeInputToBuffer } from "./multiplayer-binary.js";

const canvas = document.getElementById("game");
const ctx = canvas.getContext("2d");

const WORLD_SIZE = 500;
const MIN_BOARD_SIZE = 260;
const GRID_SIZE = 10;
const GRID_CELL_SIZE = WORLD_SIZE / GRID_SIZE;
const WALL_THICKNESS = GRID_CELL_SIZE * 0.6;
const MAX_WALL_TOTAL_LENGTH = 10;
const gameContainer = document.querySelector(".game-container");

const DEFAULT_CAT_APPEARANCE = {
  baseColor: "#ffb347",
  bellyColor: "#ffd59c",
  eyeColor: "#14365d",
  accessoryColor: "#3c78d8",
  hat: "none",
  boots: "none"
};

const CAT_APPEARANCE_STORAGE_KEY = "cat-game:appearance";
const ALLOWED_HATS = ["none", "beanie", "cap", "crown"];
const ALLOWED_BOOTS = ["none", "sneakers", "boots"];

const cat = {
  x: WORLD_SIZE / 2,
  y: WORLD_SIZE / 2,
  size: 36,
  speed: 180, // pixels per second
  facing: 1,
  moving: false,
  walkCycle: 0,
  stepAccumulator: 0,
  appearance: { ...DEFAULT_CAT_APPEARANCE }
};

const CAT_BASE_SIZE = cat.size;
const CAT_BASE_SPEED = cat.speed;

const WALK_FREQUENCY = 4; // steps per second while the cat is moving

const fish = {
  x: 0,
  y: 0,
  size: 28,
  alive: false,
  type: "normal",
  direction: 1
};

const FISH_BASE_SIZE = fish.size;
const FISH_SWIM_SPEED = 36;

const powerUp = {
  x: 0,
  y: 0,
  size: 34,
  active: false,
  remaining: 0
};

const POWER_UP_BASE_SIZE = powerUp.size;

let walls = [];

const mines = [];

const MAX_MINES = 3;
const MINE_SIZE = 26;
const MINE_MIN_DISTANCE = 25;
const SURVIVAL_MINE_BASE_COUNT = 1;
const SURVIVAL_MINE_INCREMENT = 1;
const SURVIVAL_MAX_MINES = 12;

const POWER_UP_CHANCE = 0.05;
const POWER_UP_LIFETIME = 5;
const POWER_UP_DURATION = 30;
const TIME_INCREASE_LIMIT = 15;
const TIME_DECREASE_LIMIT = 5;

const STATUS_EFFECTS = {
  speedUp: {
    icon: "👢",
    label: "Ускорение: котик передвигается в два раза быстрее"
  },
  timeIncrease: {
    icon: "🕒⬆️",
    label: "Увеличение времени: на поимку рыбки выделяется 15 секунд"
  },
  speedDown: {
    icon: "🧪",
    label: "Замедление: котик передвигается в полтора раза медленнее"
  },
  timeDecrease: {
    icon: "🕒⬇️",
    label: "Уменьшение времени: на поимку рыбки выделяется 5 секунд"
  }
};

const STATUS_EFFECT_TYPES = Object.keys(STATUS_EFFECTS);

let activeStatusEffect = null;
let displayedStatusEffect = null;
let lastResultReason = null;

const keys = new Set();
const NORMAL_FISH_TIME_LIMIT = 10;
const NORMAL_FISH_POINTS = 1;
const GOLDEN_FISH_CHANCE = 0.05;
const GOLDEN_FISH_TIME_LIMIT = 5;
const GOLDEN_FISH_POINTS = 5;

const TIMER_MODES = {
  PER_FISH: "per-fish",
  SHARED: "shared",
  SURVIVAL: "survival"
};

const SHARED_TIMER_START = 20;
const SHARED_TIMER_NORMAL_BONUS = 2;
const SHARED_TIMER_GOLDEN_BONUS = 1;
const SHARED_TIMER_MINE_PENALTY = 5;

let score = 0;
let remaining = NORMAL_FISH_TIME_LIMIT;
let lastTimestamp = 0;
let gameOver = false;
let goldenChainActive = false;
let goldenChainRemaining = 0;
let singleTimerMode = TIMER_MODES.PER_FISH;
let survivalMineCount = SURVIVAL_MINE_BASE_COUNT;
const messageEl = document.getElementById("message");
const restartBtn = document.getElementById("restart");
const scoreEl = document.getElementById("score");
const timerEl = document.getElementById("timer");
const controlsContainer = document.querySelector(".controls");
const joystickBase = document.querySelector(".joystick-base");
const joystickThumb = document.querySelector(".joystick-thumb");
const statusEffectIconEl = document.getElementById("status-effect-icon");
const leaderboardEl = document.getElementById("leaderboard");
const submitScoreForm = document.getElementById("submit-score");
const playerNameInput = document.getElementById("player-name");
const saveScoreButton = document.getElementById("save-score");
const scoreStatusEl = document.getElementById("score-status");
const resultsSummaryEl = document.getElementById("results-summary");
const resultsScoreValueEl = document.getElementById("results-score-value");
const mainMenuOverlay = document.getElementById("main-menu-overlay");
const menuPlayBtn = document.getElementById("menu-play");
const menuResultsBtn = document.getElementById("menu-results");
const menuAppearanceBtn = document.getElementById("menu-appearance");
const menuSettingsBtn = document.getElementById("menu-settings");
const openMenuBtn = document.getElementById("open-menu");
const resultsOverlay = document.getElementById("results-overlay");
const resultsBackBtn = document.getElementById("results-back");
const settingsOverlay = document.getElementById("settings-overlay");
const settingsBackBtn = document.getElementById("settings-back");
const appearanceOverlay = document.getElementById("appearance-overlay");
const appearanceBackBtn = document.getElementById("appearance-back");
const soundToggleInput = document.getElementById("setting-sound");
const musicToggleInput = document.getElementById("setting-music");
const catColorBaseInput = document.getElementById("cat-color-base");
const catColorBellyInput = document.getElementById("cat-color-belly");
const catColorEyesInput = document.getElementById("cat-color-eyes");
const catColorAccessoryInput = document.getElementById("cat-color-accessory");
const catHatSelect = document.getElementById("cat-hat");
const catBootsSelect = document.getElementById("cat-boots");
const catPreviewCanvas = document.getElementById("cat-preview");
const catPreviewCtx = catPreviewCanvas?.getContext("2d") ?? null;
const modeOverlay = document.getElementById("mode-overlay");
const startSingleBtn = document.getElementById("start-single");
const startSingleSharedBtn = document.getElementById("start-single-shared");
const startSurvivalBtn = document.getElementById("start-survival");
const startMultiplayerBtn = document.getElementById("start-multiplayer");
const multiplayerOverlay = document.getElementById("multiplayer-overlay");
const multiplayerJoinForm = document.getElementById("multiplayer-join-form");
const multiplayerNameInput = document.getElementById("multiplayer-name");
const multiplayerRoomInput = document.getElementById("multiplayer-room");
const multiplayerErrorEl = document.getElementById("multiplayer-error");
const multiplayerCancelBtn = document.getElementById("multiplayer-cancel");
const multiplayerLobbyCard = document.getElementById("multiplayer-lobby");
const multiplayerLobbyShell = document.getElementById("multiplayer-lobby-shell");
const multiplayerPrejoinCard = document.getElementById("multiplayer-prejoin");
const multiplayerRoomLabel = document.getElementById("multiplayer-room-label");
const multiplayerStatusEl = document.getElementById("multiplayer-status");
const multiplayerPlayerList = document.getElementById("multiplayer-player-list");
const multiplayerReadyBtn = document.getElementById("multiplayer-ready");
const multiplayerLeaveBtn = document.getElementById("multiplayer-leave");
const multiplayerHud = document.getElementById("multiplayer-hud");
const multiplayerHudRoom = document.getElementById("multiplayer-hud-room");
const multiplayerCountdownEl = document.getElementById("multiplayer-countdown");
const multiplayerHudPlayers = document.getElementById("multiplayer-hud-players");
const multiplayerGameMessage = document.getElementById("multiplayer-game-message");
const multiplayerRoomList = document.getElementById("multiplayer-room-list");
const multiplayerRoomEmpty = document.getElementById("multiplayer-room-empty");
const multiplayerRoomRefreshBtn = document.getElementById("multiplayer-room-refresh");
const multiplayerChatPanel = document.getElementById("multiplayer-chat");
const multiplayerChatMessages = document.getElementById("multiplayer-chat-messages");
const multiplayerChatForm = document.getElementById("multiplayer-chat-form");
const multiplayerChatInput = document.getElementById("multiplayer-chat-input");
const multiplayerChatRoomLabel = document.getElementById("multiplayer-chat-room");

let gameMode = "menu";
let multiplayerManager = null;
let multiplayerRenderHandle = null;

const directionKeys = new Set([
  "ArrowUp",
  "ArrowDown",
  "ArrowLeft",
  "ArrowRight"
]);
const JOYSTICK_DEADZONE = 0.22;
let joystickPointerId = null;
const joystickVector = { x: 0, y: 0 };

window.CAT_SERVER_URL = 'https://catgame.derium.ru'

const API_BASE_URL =
  typeof window !== "undefined" && window.CAT_SERVER_URL
    ? window.CAT_SERVER_URL
    : window.location.origin;
const WS_BASE_URL = API_BASE_URL.replace(/^http/, "ws");
let multiplayerLobby = null;

const PLAYER_ID_STORAGE_KEY = "cat-game:player-id";
const PLAYER_NAME_STORAGE_KEY = "cat-game:player-name";
const SOUND_ENABLED_STORAGE_KEY = "cat-game:sound-enabled";
const MUSIC_ENABLED_STORAGE_KEY = "cat-game:music-enabled";
const DEFAULT_SCORE_STATUS =
  "Сыграйте раунд и сохраните результат в таблицу лидеров.";

const scoreboardState = {
  hasSavedCurrentScore: false,
  isSaving: false
};

function getOrCreatePlayerId() {
  try {
    const existing = window.localStorage.getItem(PLAYER_ID_STORAGE_KEY);
    if (existing) {
      return existing;
    }
    const generated = crypto.randomUUID();
    window.localStorage.setItem(PLAYER_ID_STORAGE_KEY, generated);
    return generated;
  } catch (error) {
    return crypto.randomUUID();
  }
}

const playerId = getOrCreatePlayerId();

async function apiRequest(path, { method = "GET", body = null } = {}) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method,
    headers: { "Content-Type": "application/json" },
    body: body ? JSON.stringify(body) : null
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `Запрос завершился с ошибкой ${response.status}`);
  }

  if (response.status === 204) {
    return null;
  }

  return response.json();
}

updateStatusEffectIndicator();

function sanitizeColor(value, fallback) {
  if (typeof value === "string" && /^#([0-9a-fA-F]{6})$/.test(value.trim())) {
    return value.trim();
  }
  return fallback;
}

function sanitizeAppearance(rawAppearance) {
  const base = { ...DEFAULT_CAT_APPEARANCE };
  if (!rawAppearance || typeof rawAppearance !== "object") {
    return base;
  }
  const merged = { ...base, ...rawAppearance };
  return {
    baseColor: sanitizeColor(merged.baseColor, base.baseColor),
    bellyColor: sanitizeColor(merged.bellyColor, base.bellyColor),
    eyeColor: sanitizeColor(merged.eyeColor, base.eyeColor),
    accessoryColor: sanitizeColor(merged.accessoryColor, base.accessoryColor),
    hat: ALLOWED_HATS.includes(merged.hat) ? merged.hat : base.hat,
    boots: ALLOWED_BOOTS.includes(merged.boots) ? merged.boots : base.boots
  };
}

function safeGetStoredName() {
  try {
    return window.localStorage.getItem(PLAYER_NAME_STORAGE_KEY) || "";
  } catch (error) {
    return "";
  }
}

function safeStoreName(name) {
  try {
    window.localStorage.setItem(PLAYER_NAME_STORAGE_KEY, name);
  } catch (error) {
    // Storage access might be blocked; ignore errors silently.
  }
}

function safeGetStoredBoolean(key, defaultValue) {
  try {
    const rawValue = window.localStorage.getItem(key);
    if (rawValue === null) {
      return defaultValue;
    }
    return rawValue === "true";
  } catch (error) {
    return defaultValue;
  }
}

function safeStoreBoolean(key, value) {
  try {
    window.localStorage.setItem(key, value ? "true" : "false");
  } catch (error) {
    // Ignore storage errors silently.
  }
}

function safeGetStoredObject(key, defaultValue) {
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) {
      return defaultValue;
    }
    const parsed = JSON.parse(raw);
    return parsed ?? defaultValue;
  } catch (error) {
    return defaultValue;
  }
}

function safeStoreObject(key, value) {
  try {
    window.localStorage.setItem(key, JSON.stringify(value));
  } catch (error) {
    // Ignore storage errors silently.
  }
}

function loadCatAppearanceFromStorage() {
  const stored = safeGetStoredObject(CAT_APPEARANCE_STORAGE_KEY, DEFAULT_CAT_APPEARANCE);
  return sanitizeAppearance(stored);
}

async function loadCatAppearanceFromServer() {
  try {
    const data = await apiRequest(`/api/cats/${encodeURIComponent(playerId)}`);
    if (data?.appearance) {
      const appearance = sanitizeAppearance(data.appearance);
      cat.appearance = appearance;
      safeStoreObject(CAT_APPEARANCE_STORAGE_KEY, appearance);
      applyAppearanceToInputs(appearance);
      renderCatPreview();
    }
  } catch (error) {
    console.warn("Не удалось загрузить внешний облик кота", error);
  }
}

function saveCatAppearanceToServer(appearance) {
  const payload = {
    playerId,
    name: playerNameInput?.value?.trim() || "Игрок",
    appearance
  };
  apiRequest(`/api/cats/${encodeURIComponent(playerId)}`, { method: "POST", body: payload }).catch(
    (error) => {
      console.warn("Не удалось сохранить облик кота", error);
    }
  );
}

function applyAppearanceToInputs(appearance) {
  if (!appearance) {
    return;
  }
  if (catColorBaseInput) {
    catColorBaseInput.value = appearance.baseColor;
  }
  if (catColorBellyInput) {
    catColorBellyInput.value = appearance.bellyColor;
  }
  if (catColorEyesInput) {
    catColorEyesInput.value = appearance.eyeColor;
  }
  if (catColorAccessoryInput) {
    catColorAccessoryInput.value = appearance.accessoryColor;
  }
  if (catHatSelect) {
    catHatSelect.value = appearance.hat;
  }
  if (catBootsSelect) {
    catBootsSelect.value = appearance.boots;
  }
}

function renderCatPreview() {
  if (!catPreviewCanvas || !catPreviewCtx) {
    return;
  }

  const width = Math.max(Math.floor(catPreviewCanvas.clientWidth || catPreviewCanvas.width || 240), 180);
  const height = Math.max(Math.floor(catPreviewCanvas.clientHeight || catPreviewCanvas.height || 180), 140);

  if (catPreviewCanvas.width !== width) {
    catPreviewCanvas.width = width;
  }
  if (catPreviewCanvas.height !== height) {
    catPreviewCanvas.height = height;
  }

  catPreviewCtx.clearRect(0, 0, width, height);

  const previewCat = {
    x: width / 2,
    y: height * 0.65,
    size: Math.min(width, height) * 0.32,
    facing: 1,
    moving: false,
    walkCycle: 0,
    appearance: { ...cat.appearance }
  };

  drawCatSprite(previewCat, catPreviewCtx);
}

function updateCatAppearance(changes = {}) {
  const nextAppearance = sanitizeAppearance({ ...cat.appearance, ...changes });
  cat.appearance = nextAppearance;
  safeStoreObject(CAT_APPEARANCE_STORAGE_KEY, nextAppearance);
  saveCatAppearanceToServer(nextAppearance);
  applyAppearanceToInputs(nextAppearance);
  renderCatPreview();
  if (gameMode === "multiplayer" && multiplayerManager) {
    multiplayerManager.updateAppearance(nextAppearance);
  }
}

cat.appearance = loadCatAppearanceFromStorage();
applyAppearanceToInputs(cat.appearance);
renderCatPreview();
loadCatAppearanceFromServer();

function setScoreStatus(text = "") {
  if (scoreStatusEl) {
    scoreStatusEl.textContent = text;
  }
}

function showMainMenu() {
  hideModeSelection();
  hideMultiplayerOverlay();
  hideResultsOverlay();
  hideSettingsOverlay();
  hideAppearanceOverlay();
  if (mainMenuOverlay) {
    mainMenuOverlay.classList.remove("hidden");
  }
  gameMode = "menu";
}

function hideMainMenu() {
  if (mainMenuOverlay) {
    mainMenuOverlay.classList.add("hidden");
  }
}

function showResultsOverlay() {
  hideMainMenu();
  hideModeSelection();
  hideMultiplayerOverlay();
  hideAppearanceOverlay();
  updateResultsSummary();
  if (resultsOverlay) {
    resultsOverlay.classList.remove("hidden");
  }
}

function hideResultsOverlay() {
  if (resultsOverlay) {
    resultsOverlay.classList.add("hidden");
  }
}

function showSettingsOverlay() {
  hideMainMenu();
  hideModeSelection();
  hideMultiplayerOverlay();
  hideAppearanceOverlay();
  if (settingsOverlay) {
    settingsOverlay.classList.remove("hidden");
  }
}

function hideSettingsOverlay() {
  if (settingsOverlay) {
    settingsOverlay.classList.add("hidden");
  }
}

function showAppearanceOverlay() {
  hideMainMenu();
  hideModeSelection();
  hideMultiplayerOverlay();
  hideSettingsOverlay();
  if (appearanceOverlay) {
    appearanceOverlay.classList.remove("hidden");
  }
  renderCatPreview();
}

function hideAppearanceOverlay() {
  if (appearanceOverlay) {
    appearanceOverlay.classList.add("hidden");
  }
}

function showModeSelection() {
  hideMainMenu();
  hideResultsOverlay();
  hideSettingsOverlay();
  hideAppearanceOverlay();
  if (modeOverlay) {
    modeOverlay.classList.remove("hidden");
  }
  if (multiplayerOverlay) {
    multiplayerOverlay.classList.add("hidden");
  }
}

function hideModeSelection() {
  if (modeOverlay) {
    modeOverlay.classList.add("hidden");
  }
}

function showMultiplayerJoinForm() {
  if (multiplayerOverlay) {
    multiplayerOverlay.classList.remove("hidden");
  }
  ensureLobbyConnected();
  if (multiplayerPrejoinCard) {
    multiplayerPrejoinCard.classList.remove("hidden");
  }
  if (multiplayerLobbyShell) {
    multiplayerLobbyShell.classList.add("hidden");
  }
  if (multiplayerLobbyCard) {
    multiplayerLobbyCard.classList.add("hidden");
  }
  hideMultiplayerChat();
  if (multiplayerErrorEl) {
    multiplayerErrorEl.textContent = "";
  }
  if (multiplayerNameInput && !multiplayerNameInput.value && playerNameInput) {
    multiplayerNameInput.value = playerNameInput.value;
  }
  if (multiplayerNameInput) {
    multiplayerNameInput.focus();
  }
}

function showMultiplayerLobby() {
  if (multiplayerOverlay) {
    multiplayerOverlay.classList.remove("hidden");
  }
  if (multiplayerPrejoinCard) {
    multiplayerPrejoinCard.classList.add("hidden");
  }
  if (multiplayerLobbyShell) {
    multiplayerLobbyShell.classList.remove("hidden");
  }
  if (multiplayerLobbyCard) {
    multiplayerLobbyCard.classList.remove("hidden");
  }
}

function showMultiplayerOverlay() {
  if (multiplayerOverlay) {
    multiplayerOverlay.classList.remove("hidden");
  }
}

function hideMultiplayerOverlay() {
  if (multiplayerOverlay) {
    multiplayerOverlay.classList.add("hidden");
  }
}

function showMultiplayerChat(roomName = "") {
  if (!multiplayerChatPanel) {
    return;
  }
  multiplayerChatPanel.classList.remove("hidden");
  if (multiplayerChatRoomLabel) {
    multiplayerChatRoomLabel.textContent = roomName ? `Комната ${roomName}` : "";
  }
}

function hideMultiplayerChat({ reset = true } = {}) {
  if (multiplayerChatPanel) {
    multiplayerChatPanel.classList.add("hidden");
  }
  if (reset) {
    renderChatMessages([]);
  }
  if (multiplayerChatRoomLabel) {
    multiplayerChatRoomLabel.textContent = "";
  }
}

function renderChatMessages(messages) {
  if (!multiplayerChatMessages) {
    return;
  }
  multiplayerChatMessages.innerHTML = "";
  if (!messages || messages.length === 0) {
    const empty = document.createElement("li");
    empty.textContent = "Сообщений пока нет";
    multiplayerChatMessages.appendChild(empty);
    return;
  }

  messages.slice(-50).forEach((message) => {
    const item = document.createElement("li");
    const timeLabel = message.at
      ? new Date(message.at).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" })
      : "";
    item.innerHTML = `<span class="multiplayer-chat-author">${escapeHtml(
      message.name || "Игрок"
    )}</span>: ${escapeHtml(message.text)} <span class="multiplayer-chat-time">${escapeHtml(
      timeLabel
    )}</span>`;
    multiplayerChatMessages.appendChild(item);
  });
  multiplayerChatMessages.scrollTop = multiplayerChatMessages.scrollHeight;
}

function renderMultiplayerRoomList() {
  if (!multiplayerRoomList || !multiplayerRoomEmpty) {
    return;
  }
  const rooms = multiplayerLobby?.getRooms() || [];
  multiplayerRoomList.innerHTML = "";
  multiplayerRoomEmpty.classList.toggle("hidden", rooms.length > 0);

  rooms.forEach((room) => {
    const item = document.createElement("li");
    const isJoinable = room.phase !== "playing" && room.phase !== "countdown";
    item.classList.toggle("unavailable", !isJoinable);
    const meta = document.createElement("div");
    meta.className = "multiplayer-room-meta";
    meta.innerHTML = `
      <span class="multiplayer-room-name">${escapeHtml(room.roomName)}</span>
      <span class="multiplayer-room-status">Игроков: ${room.playerCount} · Статус: ${
        room.phase === "playing" ? "Идёт игра" : room.phase === "countdown" ? "Скоро старт" : "Ожидание"
      }</span>
    `;

    const joinButton = document.createElement("button");
    joinButton.type = "button";
    joinButton.textContent = isJoinable ? "Подключиться" : "Недоступно";
    joinButton.disabled = !isJoinable;
    joinButton.dataset.roomName = room.roomName;

    item.appendChild(meta);
    item.appendChild(joinButton);
    multiplayerRoomList.appendChild(item);
  });
}

async function ensureLobbyConnected() {
  if (!multiplayerLobby) {
    return;
  }
  try {
    await multiplayerLobby.connect();
    multiplayerLobby.requestSync();
    renderMultiplayerRoomList();
  } catch (error) {
    console.warn("Не удалось подключиться к лобби", error);
  }
}

function showRestartButton() {
  if (restartBtn) {
    restartBtn.classList.remove("hidden");
  }
}

function hideRestartButton() {
  if (restartBtn) {
    restartBtn.classList.add("hidden");
  }
}

async function leaveMultiplayerRoom({ backToMenu = false } = {}) {
  if (multiplayerManager) {
    try {
      await multiplayerManager.leave();
    } catch (error) {
      console.warn("Не удалось корректно выйти из комнаты", error);
    }
    multiplayerManager = null;
  }
  multiplayerHud?.classList.add("hidden");
  hideMultiplayerChat();
  showRestartButton();
  soundManager.stopMusic(0.6);
  if (backToMenu) {
    showMainMenu();
  }
  setDisplayedStatusEffect(null);
  messageEl.textContent = "";
  updateTimerDisplay(NORMAL_FISH_TIME_LIMIT);
  scoreEl.textContent = "0";
  gameMode = backToMenu ? "menu" : gameMode;
}

async function joinMultiplayerRoom(roomName, playerName) {
  if (multiplayerManager) {
    await leaveMultiplayerRoom();
  }
  clearStatusEffect();
  multiplayerManager = new MultiplayerManager(multiplayerLobby);
  await multiplayerManager.join(roomName, playerName);
  safeStoreName(playerName);
  multiplayerManager.updateInputFromControls();
  hideModeSelection();
  showMultiplayerLobby();
  showMultiplayerChat(roomName);
  hideRestartButton();
  messageEl.textContent = "";
  timerEl.textContent = "0.0";
  scoreEl.textContent = "0";
  gameMode = "multiplayer";
  updateBoardSize();
}

function startSingleMode(timerModeOverride = TIMER_MODES.PER_FISH) {
  leaveMultiplayerRoom().catch(() => {});
  hideMultiplayerOverlay();
  showRestartButton();
  singleTimerMode = timerModeOverride;
  survivalMineCount = SURVIVAL_MINE_BASE_COUNT;
  gameMode = "single";
  gameOver = true;
  hideModeSelection();
  setTimeout(() => {
    gameOver = false;
    resetGame();
  }, 0);
}

function openModeSelection() {
  showModeSelection();
  gameMode = "menu";
}

function updateScoreFormControls() {
  if (!submitScoreForm) {
    return;
  }
  if (playerNameInput) {
    playerNameInput.disabled = scoreboardState.isSaving;
  }
  if (saveScoreButton) {
    const shouldEnable =
      gameOver &&
      !scoreboardState.hasSavedCurrentScore &&
      !scoreboardState.isSaving;
    saveScoreButton.disabled = !shouldEnable;
  }
}

function updateResultsSummary(reasonOverride = null) {
  if (!resultsSummaryEl) {
    return;
  }

  const isRoundFinished = Boolean(gameOver);
  const reasonText = reasonOverride ?? lastResultReason;
  if (!isRoundFinished) {
    resultsSummaryEl.textContent =
      "Сыграйте раунд, чтобы увидеть ваши результаты.";
  } else {
    const reasonPrefix = reasonText ? `${reasonText} ` : "";
    resultsSummaryEl.textContent = `${reasonPrefix}Итоговый счёт: ${score}.`;
  }

  if (resultsScoreValueEl) {
    resultsScoreValueEl.textContent = isRoundFinished ? score : 0;
  }
}

function renderLeaderboard(items) {
  if (!leaderboardEl) {
    return;
  }
  leaderboardEl.innerHTML = "";
  if (!items || items.length === 0) {
    const emptyItem = document.createElement("li");
    emptyItem.textContent = "Пока нет результатов.";
    leaderboardEl.appendChild(emptyItem);
    return;
  }

  items.forEach(({ name, score: scoreValue }) => {
    const item = document.createElement("li");
    item.innerHTML = `<span>${escapeHtml(name)}</span><span>${scoreValue}</span>`;
    leaderboardEl.appendChild(item);
  });
}

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function updateStatusEffectIndicator() {
  if (!statusEffectIconEl) {
    return;
  }
  if (!displayedStatusEffect) {
    statusEffectIconEl.textContent = "—";
    statusEffectIconEl.setAttribute("aria-label", "Нет эффектов");
    return;
  }

  const effect = STATUS_EFFECTS[displayedStatusEffect.type];
  if (effect) {
    statusEffectIconEl.textContent = effect.icon;
    statusEffectIconEl.setAttribute("aria-label", effect.label);
  }
}

function clearStatusEffect() {
  activeStatusEffect = null;
  setDisplayedStatusEffect(null);
}

function setStatusEffect(effectType) {
  if (!STATUS_EFFECTS[effectType]) {
    clearStatusEffect();
    return;
  }
  activeStatusEffect = {
    type: effectType,
    remaining: POWER_UP_DURATION
  };
  setDisplayedStatusEffect(activeStatusEffect);

  if (!fish.alive) {
    return;
  }

  if (isSharedTimerMode()) {
    if (effectType === "timeIncrease") {
      remaining += TIME_INCREASE_LIMIT;
    } else if (effectType === "timeDecrease") {
      remaining = Math.max(remaining - TIME_DECREASE_LIMIT, 0);
    }
    updateTimerDisplay();
    if (remaining <= 0) {
      endGame("Время вышло!");
    }
    return;
  }

  const newLimit = getFishTimeLimitForFish(fish.type);
  if (effectType === "timeIncrease") {
    remaining = Math.max(remaining, newLimit);
  } else if (effectType === "timeDecrease") {
    remaining = Math.min(remaining, newLimit);
  }
}

function applyRandomStatusEffect() {
  if (STATUS_EFFECT_TYPES.length === 0) {
    return;
  }
  const randomIndex = Math.floor(Math.random() * STATUS_EFFECT_TYPES.length);
  setStatusEffect(STATUS_EFFECT_TYPES[randomIndex]);
}

function setDisplayedStatusEffect(effect) {
  if (effect && STATUS_EFFECTS[effect.type]) {
    displayedStatusEffect = {
      type: effect.type,
      remaining: effect.remaining ?? 0
    };
  } else {
    displayedStatusEffect = null;
  }
  updateStatusEffectIndicator();
}

function syncMultiplayerStatusEffect(effect) {
  if (effect && STATUS_EFFECTS[effect.type]) {
    setDisplayedStatusEffect(effect);
  } else {
    setDisplayedStatusEffect(null);
  }
}

function getCatSpeedMultiplier() {
  if (!activeStatusEffect) {
    return 1;
  }
  if (activeStatusEffect.type === "speedUp") {
    return 2;
  }
  if (activeStatusEffect.type === "speedDown") {
    return 1 / 1.5;
  }
  return 1;
}

function getFishTimeLimitForFish(fishType) {
  if (activeStatusEffect?.type === "timeIncrease") {
    return TIME_INCREASE_LIMIT;
  }
  if (activeStatusEffect?.type === "timeDecrease") {
    return TIME_DECREASE_LIMIT;
  }
  return fishType === "golden" ? GOLDEN_FISH_TIME_LIMIT : NORMAL_FISH_TIME_LIMIT;
}

function spawnPowerUp() {
  const margin = 36;
  for (let attempt = 0; attempt < 40; attempt++) {
    const candidateX = margin + Math.random() * (WORLD_SIZE - margin * 2);
    const candidateY = margin + Math.random() * (WORLD_SIZE - margin * 2);
    if (!circleIntersectsAnyWall(candidateX, candidateY, powerUp.size / 2 + 2)) {
      powerUp.x = candidateX;
      powerUp.y = candidateY;
      powerUp.active = true;
      powerUp.remaining = POWER_UP_LIFETIME;
      return;
    }
  }
  powerUp.active = false;
  powerUp.remaining = 0;
}

function clearMines() {
  mines.length = 0;
}

function isMinePositionValid(x, y, radius) {
  const safeRadius = radius + MINE_MIN_DISTANCE;

  if (
    x - safeRadius < 0 ||
    y - safeRadius < 0 ||
    x + safeRadius > WORLD_SIZE ||
    y + safeRadius > WORLD_SIZE
  ) {
    return false;
  }

  if (circleIntersectsAnyWall(x, y, safeRadius)) {
    return false;
  }

  const distanceToCat = Math.hypot(x - cat.x, y - cat.y);
  if (distanceToCat <= cat.size / 2 + safeRadius) {
    return false;
  }

  if (fish.alive) {
    const distanceToFish = Math.hypot(x - fish.x, y - fish.y);
    if (distanceToFish <= fish.size / 2 + safeRadius) {
      return false;
    }
  }

  for (const mine of mines) {
    const distanceToMine = Math.hypot(x - mine.x, y - mine.y);
    if (distanceToMine <= mine.size / 2 + radius + MINE_MIN_DISTANCE) {
      return false;
    }
  }

  return true;
}

function spawnMines(overrideCount = null) {
  clearMines();

  const targetCount =
    typeof overrideCount === "number"
      ? clamp(Math.floor(overrideCount), 0, SURVIVAL_MAX_MINES)
      : Math.floor(Math.random() * (MAX_MINES + 1));

  if (targetCount === 0) {
    return;
  }

  const radius = MINE_SIZE / 2;
  const margin = radius + MINE_MIN_DISTANCE + 4;
  let attempts = 0;
  const maxAttempts = 200 + targetCount * 10;

  while (mines.length < targetCount && attempts < maxAttempts) {
    attempts += 1;
    const candidateX = margin + Math.random() * (WORLD_SIZE - margin * 2);
    const candidateY = margin + Math.random() * (WORLD_SIZE - margin * 2);

    if (!isMinePositionValid(candidateX, candidateY, radius)) {
      continue;
    }

    mines.push({ x: candidateX, y: candidateY, size: MINE_SIZE });
  }
}

function maybeSpawnPowerUp() {
  if (Math.random() < POWER_UP_CHANCE) {
    spawnPowerUp();
  } else if (powerUp.active) {
    // Allow existing power-up to continue ticking down.
    powerUp.remaining = Math.min(powerUp.remaining, POWER_UP_LIFETIME);
  }
}

async function fetchLeaderboard() {
  if (leaderboardEl) {
    leaderboardEl.innerHTML = "";
    const loadingItem = document.createElement("li");
    loadingItem.textContent = "Загрузка...";
    leaderboardEl.appendChild(loadingItem);
  }

  try {
    const response = await apiRequest("/api/scores");
    const items = Array.isArray(response?.scores) ? response.scores : response || [];
    renderLeaderboard(items);
  } catch (error) {
    console.error("Не удалось получить таблицу лидеров", error);
    if (leaderboardEl) {
      leaderboardEl.innerHTML = "";
      const item = document.createElement("li");
      item.textContent = "Не удалось загрузить таблицу лидеров.";
      leaderboardEl.appendChild(item);
    }
    const fallbackMessage = scoreboardState.hasSavedCurrentScore
      ? "Результат сохранён, но не удалось обновить таблицу лидеров."
      : "Не удалось загрузить таблицу лидеров.";
    setScoreStatus(fallbackMessage);
  }
}

const storedName = safeGetStoredName();
if (playerNameInput && storedName) {
  playerNameInput.value = storedName;
}

setScoreStatus(DEFAULT_SCORE_STATUS);
fetchLeaderboard();

updateScoreFormControls();

class SoundManager {
  constructor() {
    this.context = null;
    this.masterGain = null;
    this.musicGain = null;
    this.sfxGain = null;
    this.enabled = false;
    this.musicEnabled = true;
    this.sfxEnabled = true;
    this.padOscillators = [];
    this.musicLoopId = null;
    this.lastStepTime = 0;
    this.loopDuration = 16;
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
    this.sfxGain.gain.value = this.sfxEnabled ? 1 : 0;
    this.sfxGain.connect(this.masterGain);

    this.enabled = true;
    if (this.musicEnabled) {
      this.startMusic();
    }
  }

  startMusic() {
    if (!this.enabled || !this.musicEnabled) {
      return;
    }

    const ctx = this.context;
    const now = ctx.currentTime;
    if (this.musicLoopId) {
      window.clearInterval(this.musicLoopId);
      this.musicLoopId = null;
    }
    this.musicGain.gain.cancelScheduledValues(now);
    const currentGain = Math.max(this.musicGain.gain.value, 0.0001);
    this.musicGain.gain.setValueAtTime(currentGain, now);
    this.musicGain.gain.linearRampToValueAtTime(0.48, now + 2.5);
    this.createPadLayer(now);
    this.playMusicSegment(now);
    this.musicLoopId = window.setInterval(() => {
      this.playMusicSegment(ctx.currentTime);
    }, this.loopDuration * 1000);
  }

  stopMusic(fadeDuration = 0.6) {
    if (!this.enabled || !this.context) {
      return;
    }

    if (this.musicLoopId) {
      window.clearInterval(this.musicLoopId);
      this.musicLoopId = null;
    }

    const ctx = this.context;
    const now = ctx.currentTime;
    this.musicGain.gain.cancelScheduledValues(now);
    this.musicGain.gain.setValueAtTime(
      Math.max(this.musicGain.gain.value, 0.0001),
      now
    );
    this.musicGain.gain.linearRampToValueAtTime(0.0001, now + fadeDuration);

    if (this.rhythmGain) {
      this.rhythmGain.gain.cancelScheduledValues(now);
      this.rhythmGain.gain.setValueAtTime(
        Math.max(this.rhythmGain.gain.value, 0.0001),
        now
      );
      this.rhythmGain.gain.linearRampToValueAtTime(
        0.0001,
        now + fadeDuration * 0.8
      );
    }

    this.padOscillators.forEach(({ osc, gain }) => {
      try {
        gain.gain.cancelScheduledValues(now);
        gain.gain.setValueAtTime(Math.max(gain.gain.value, 0.0001), now);
        gain.gain.linearRampToValueAtTime(0.0001, now + fadeDuration);
        osc.stop(now + fadeDuration + 0.2);
      } catch (error) {
        // Ignored
      }
    });
    this.padOscillators = [];
  }

  setSfxEnabled(enabled) {
    this.sfxEnabled = Boolean(enabled);
    if (this.sfxGain) {
      this.sfxGain.gain.value = this.sfxEnabled ? 1 : 0;
    }
  }

  setMusicEnabled(enabled) {
    this.musicEnabled = Boolean(enabled);
    if (!this.enabled) {
      return;
    }
    if (this.musicEnabled) {
      this.startMusic();
    } else {
      this.stopMusic(0.3);
    }
  }

  createPadLayer(startTime) {
    if (this.padOscillators.length > 0) {
      return;
    }

    const ctx = this.context;
    const padGain = ctx.createGain();
    padGain.gain.setValueAtTime(0.0001, startTime);
    padGain.gain.exponentialRampToValueAtTime(0.32, startTime + 4.2);
    padGain.connect(this.musicGain);

    const padNotes = [220.0, 329.63, 440.0];
    padNotes.forEach((frequency, index) => {
      const osc = ctx.createOscillator();
      osc.type = index % 2 === 0 ? "triangle" : "sawtooth";
      osc.frequency.setValueAtTime(frequency, startTime);
      const gain = ctx.createGain();
      gain.gain.value = 0.14 / padNotes.length;
      osc.connect(gain);
      gain.connect(padGain);
      osc.start(startTime + index * 0.12);
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

    this.playProgression(startTime);
    this.playLead(startTime + 0.25);
    this.playPulseLayer(startTime);
    this.playPercussion(startTime);
  }

  playProgression(startTime) {
    const ctx = this.context;
    const chords = [
      { root: 220.0, type: "minor" },
      { root: 261.63, type: "major" },
      { root: 196.0, type: "major" },
      { root: 293.66, type: "minor" }
    ];
    const chordDuration = this.loopDuration / chords.length;

    chords.forEach((chord, chordIndex) => {
      const chordStart = startTime + chordIndex * chordDuration;
      const chordIntervals = chord.type === "minor" ? [0, 3, 7, 12] : [0, 4, 7, 12];
      chordIntervals.forEach((interval, voiceIndex) => {
        const osc = ctx.createOscillator();
        osc.type = voiceIndex % 2 === 0 ? "triangle" : "sine";
        const frequency = chord.root * Math.pow(2, interval / 12);
        osc.frequency.setValueAtTime(frequency, chordStart);

        const gain = ctx.createGain();
        gain.gain.setValueAtTime(0.0001, chordStart);
        gain.gain.linearRampToValueAtTime(
          0.18 / chordIntervals.length,
          chordStart + 0.6
        );
        gain.gain.linearRampToValueAtTime(
          0.0001,
          chordStart + chordDuration * 0.95
        );

        osc.connect(gain);
        gain.connect(this.musicGain);
        osc.start(chordStart);
        osc.stop(chordStart + chordDuration + 1.2);
      });
    });
  }

  playPulseLayer(startTime) {
    const ctx = this.context;
    const chords = [
      440.0,
      523.25,
      392.0,
      587.33
    ];
    const chordDuration = this.loopDuration / chords.length;
    const pulsesPerChord = 8;

    chords.forEach((baseFrequency, chordIndex) => {
      const stepDuration = chordDuration / pulsesPerChord;
      for (let i = 0; i < pulsesPerChord; i += 1) {
        const time = startTime + chordIndex * chordDuration + i * stepDuration;
        const osc = ctx.createOscillator();
        osc.type = "sawtooth";
        const interval = i % 2 === 0 ? 0 : 7;
        const frequency = baseFrequency * Math.pow(2, interval / 12);
        osc.frequency.setValueAtTime(frequency, time);

        const gain = ctx.createGain();
        gain.gain.setValueAtTime(0.0001, time);
        gain.gain.exponentialRampToValueAtTime(0.16, time + 0.06);
        gain.gain.exponentialRampToValueAtTime(0.0001, time + stepDuration * 0.9);

        const filter = ctx.createBiquadFilter();
        filter.type = "lowpass";
        filter.frequency.setValueAtTime(1600, time);
        filter.Q.setValueAtTime(3.5, time);

        osc.connect(filter);
        filter.connect(gain);
        gain.connect(this.musicGain);
        osc.start(time);
        osc.stop(time + stepDuration * 1.2);
      }
    });
  }

  playLead(startTime) {
    const ctx = this.context;
    const baseFrequency = 440.0; // A4
    const melody = [
      { interval: 0, duration: 0.5 },
      { interval: 3, duration: 0.5 },
      { interval: 7, duration: 0.5 },
      { interval: 10, duration: 0.5 },
      { interval: 12, duration: 0.75 },
      { interval: 10, duration: 0.25 },
      { interval: 7, duration: 0.5 },
      { interval: 3, duration: 0.5 },
      { interval: 0, duration: 0.75 },
      { interval: 3, duration: 0.25 },
      { interval: -2, duration: 0.5 },
      { interval: 0, duration: 0.5 },
      { interval: 3, duration: 0.5 },
      { interval: 7, duration: 0.5 },
      { interval: 10, duration: 1.0 },
      { interval: 7, duration: 1.0 }
    ];

    let currentTime = startTime;
    melody.forEach(({ interval, duration }) => {
      const osc = ctx.createOscillator();
      osc.type = "triangle";
      const frequency = baseFrequency * Math.pow(2, interval / 12);
      osc.frequency.setValueAtTime(frequency, currentTime);

      const gain = ctx.createGain();
      gain.gain.setValueAtTime(0.0001, currentTime);
      gain.gain.exponentialRampToValueAtTime(0.09, currentTime + 0.07);
      gain.gain.exponentialRampToValueAtTime(0.0001, currentTime + duration * 0.92);

      const vibrato = ctx.createOscillator();
      vibrato.type = "sine";
      vibrato.frequency.setValueAtTime(5.8, currentTime);
      const vibratoGain = ctx.createGain();
      vibratoGain.gain.setValueAtTime(4.2, currentTime);
      vibrato.connect(vibratoGain);
      vibratoGain.connect(osc.frequency);

      osc.connect(gain);
      gain.connect(this.musicGain);
      osc.start(currentTime);
      osc.stop(currentTime + duration + 0.4);
      vibrato.start(currentTime);
      vibrato.stop(currentTime + duration + 0.4);

      currentTime += duration;
    });
  }

  playPercussion(startTime) {
    const ctx = this.context;
    const beatSpacing = 0.5;
    const hits = Math.floor(this.loopDuration / beatSpacing);

    if (!this.rhythmGain) {
      this.rhythmGain = ctx.createGain();
      this.rhythmGain.gain.value = 0.5;
      this.rhythmGain.connect(this.musicGain);
    }

    for (let i = 0; i < hits; i++) {
      const time = startTime + i * beatSpacing;
      const bufferSource = ctx.createBufferSource();
      bufferSource.buffer = this.ensureNoiseBuffer();

      const filter = ctx.createBiquadFilter();
      filter.type = "bandpass";
      filter.frequency.setValueAtTime(i % 2 === 0 ? 1200 : 900, time);
      filter.Q.setValueAtTime(5.5, time);

      const gain = ctx.createGain();
      gain.gain.setValueAtTime(0.0001, time);
      const accent = i % 4 === 0 ? 0.55 : i % 2 === 0 ? 0.32 : 0.2;
      gain.gain.exponentialRampToValueAtTime(accent, time + 0.025);
      gain.gain.exponentialRampToValueAtTime(0.0001, time + 0.24);

      bufferSource.connect(filter);
      filter.connect(gain);
      gain.connect(this.rhythmGain);
      bufferSource.start(time);
      bufferSource.stop(time + 0.3);

      if (i % 4 === 0) {
        const lowOsc = ctx.createOscillator();
        lowOsc.type = "sine";
        lowOsc.frequency.setValueAtTime(90, time);
        lowOsc.frequency.linearRampToValueAtTime(70, time + 0.25);
        const lowGain = ctx.createGain();
        lowGain.gain.setValueAtTime(0.0001, time);
        lowGain.gain.exponentialRampToValueAtTime(0.22, time + 0.04);
        lowGain.gain.exponentialRampToValueAtTime(0.0001, time + 0.4);
        lowOsc.connect(lowGain);
        lowGain.connect(this.rhythmGain);
        lowOsc.start(time);
        lowOsc.stop(time + 0.5);
      }
    }
  }

  playCatch() {
    if (!this.enabled || !this.sfxEnabled || !this.sfxGain) {
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
    if (!this.enabled || !this.sfxEnabled || !this.sfxGain) {
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
const initialSoundEnabled = safeGetStoredBoolean(SOUND_ENABLED_STORAGE_KEY, true);
const initialMusicEnabled = safeGetStoredBoolean(MUSIC_ENABLED_STORAGE_KEY, true);
soundManager.setSfxEnabled(initialSoundEnabled);
soundManager.setMusicEnabled(initialMusicEnabled);

function ensureAudioActive() {
  soundManager.init();
  if (soundManager.context && soundManager.context.state === "suspended") {
    soundManager.context.resume();
  }
}

function normalizeKey(key) {
  if (typeof key !== "string") {
    return "";
  }
  if (key.startsWith("Arrow")) {
    return key;
  }
  return key.toLowerCase();
}

function clamp(value, min, max) {
  return Math.min(Math.max(value, min), max);
}

function randomInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

function createSeededRng(seed) {
  let state = seed >>> 0;
  return () => {
    state += 0x6d2b79f5;
    let t = state;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function clampGridIndex(value) {
  return clamp(value, 0, GRID_SIZE - 1);
}

function positionToGridCell(x, y) {
  const col = clampGridIndex(Math.floor(x / GRID_CELL_SIZE));
  const row = clampGridIndex(Math.floor(y / GRID_CELL_SIZE));
  return { row, col };
}

function getCellsForSegment(row, col, length, orientation) {
  const cells = [];
  for (let offset = 0; offset < length; offset++) {
    const currentRow = orientation === "horizontal" ? row : row + offset;
    const currentCol = orientation === "horizontal" ? col + offset : col;
    cells.push({ row: currentRow, col: currentCol });
  }
  return cells;
}

function buildBlockedGridFromSegments(segments) {
  const grid = Array.from({ length: GRID_SIZE }, () =>
    Array(GRID_SIZE).fill(false)
  );
  segments.forEach(({ row, col, length, orientation }) => {
    for (let offset = 0; offset < length; offset++) {
      const currentRow = orientation === "horizontal" ? row : row + offset;
      const currentCol = orientation === "horizontal" ? col + offset : col;
      if (
        currentRow >= 0 &&
        currentRow < GRID_SIZE &&
        currentCol >= 0 &&
        currentCol < GRID_SIZE
      ) {
        grid[currentRow][currentCol] = true;
      }
    }
  });
  return grid;
}

function isPathAvailable(catCell, fishCell, blockedGrid) {
  const startKey = `${catCell.row},${catCell.col}`;
  const targetKey = `${fishCell.row},${fishCell.col}`;
  const visited = new Set();
  const queue = [{ row: catCell.row, col: catCell.col }];
  const deltas = [
    [1, 0],
    [-1, 0],
    [0, 1],
    [0, -1]
  ];

  visited.add(startKey);

  while (queue.length > 0) {
    const current = queue.shift();
    const key = `${current.row},${current.col}`;
    if (key === targetKey) {
      return true;
    }

    for (const [dRow, dCol] of deltas) {
      const nextRow = current.row + dRow;
      const nextCol = current.col + dCol;
      if (
        nextRow < 0 ||
        nextRow >= GRID_SIZE ||
        nextCol < 0 ||
        nextCol >= GRID_SIZE
      ) {
        continue;
      }
      if (blockedGrid[nextRow][nextCol]) {
        continue;
      }
      const nextKey = `${nextRow},${nextCol}`;
      if (visited.has(nextKey)) {
        continue;
      }
      visited.add(nextKey);
      queue.push({ row: nextRow, col: nextCol });
    }
  }

  return false;
}

function convertSegmentsToWalls(segments) {
  return segments.map(({ row, col, length, orientation }) => {
    if (orientation === "horizontal") {
      return {
        x: col * GRID_CELL_SIZE,
        y: row * GRID_CELL_SIZE + (GRID_CELL_SIZE - WALL_THICKNESS) / 2,
        width: length * GRID_CELL_SIZE,
        height: WALL_THICKNESS
      };
    }
    return {
      x: col * GRID_CELL_SIZE + (GRID_CELL_SIZE - WALL_THICKNESS) / 2,
      y: row * GRID_CELL_SIZE,
      width: WALL_THICKNESS,
      height: length * GRID_CELL_SIZE
    };
  });
}

function circleIntersectsRect(cx, cy, radius, rect) {
  const closestX = clamp(cx, rect.x, rect.x + rect.width);
  const closestY = clamp(cy, rect.y, rect.y + rect.height);
  const dx = cx - closestX;
  const dy = cy - closestY;
  return dx * dx + dy * dy < radius * radius;
}

function circleIntersectsAnyWall(cx, cy, radius, candidateWalls = walls) {
  return candidateWalls.some((wall) => circleIntersectsRect(cx, cy, radius, wall));
}

function doesFishCollideAt(xPosition) {
  const radius = fish.size / 2;
  if (xPosition - radius < 0 || xPosition + radius > WORLD_SIZE) {
    return true;
  }

  if (circleIntersectsAnyWall(xPosition, fish.y, radius)) {
    return true;
  }

  if (powerUp.active) {
    const half = powerUp.size / 2;
    const powerUpRect = {
      x: powerUp.x - half,
      y: powerUp.y - half,
      width: powerUp.size,
      height: powerUp.size
    };
    if (circleIntersectsRect(xPosition, fish.y, radius, powerUpRect)) {
      return true;
    }
  }

  for (const mine of mines) {
    const distanceToMine = Math.hypot(xPosition - mine.x, fish.y - mine.y);
    if (distanceToMine < radius + mine.size / 2) {
      return true;
    }
  }

  return false;
}

function updateFishMovement(delta) {
  if (!fish.alive) {
    return;
  }

  const swimStep = fish.direction * FISH_SWIM_SPEED * delta;
  let nextX = fish.x + swimStep;

  if (doesFishCollideAt(nextX)) {
    fish.direction *= -1;
    nextX = fish.x + fish.direction * FISH_SWIM_SPEED * delta;
    if (doesFishCollideAt(nextX)) {
      return;
    }
  }

  fish.x = clamp(nextX, fish.size / 2, WORLD_SIZE - fish.size / 2);
}

function buildRandomWallSegments(catCell, fishCell) {
  const segments = [];
  const occupied = new Set();
  let totalLength = 0;
  let attempts = 0;

  while (totalLength < MAX_WALL_TOTAL_LENGTH && attempts < 80) {
    attempts += 1;
    if (segments.length >= 2 && Math.random() < 0.35) {
      break;
    }

    const remaining = MAX_WALL_TOTAL_LENGTH - totalLength;
    const maxSegmentLength = Math.min(remaining, 3);
    if (maxSegmentLength <= 0) {
      continue;
    }
    const length = Math.min(
      remaining,
      1 + Math.floor(Math.random() * maxSegmentLength)
    );
    const orientation = Math.random() < 0.5 ? "horizontal" : "vertical";
    const maxRow =
      orientation === "horizontal" ? GRID_SIZE - 1 : GRID_SIZE - length;
    const maxCol =
      orientation === "horizontal" ? GRID_SIZE - length : GRID_SIZE - 1;

    if (maxRow < 0 || maxCol < 0) {
      continue;
    }

    const row = randomInt(0, maxRow);
    const col = randomInt(0, maxCol);
    const cells = getCellsForSegment(row, col, length, orientation);

    let invalid = false;
    for (const cell of cells) {
      const key = `${cell.row},${cell.col}`;
      if (occupied.has(key)) {
        invalid = true;
        break;
      }
      if (
        (cell.row === catCell.row && cell.col === catCell.col) ||
        (cell.row === fishCell.row && cell.col === fishCell.col)
      ) {
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

  if (segments.length < 2) {
    return null;
  }

  return segments;
}

function generateWallsLayout(catCell, fishCell) {
  for (let attempt = 0; attempt < 160; attempt++) {
    const segments = buildRandomWallSegments(catCell, fishCell);
    if (!segments) {
      continue;
    }
    const blockedGrid = buildBlockedGridFromSegments(segments);
    if (!isPathAvailable(catCell, fishCell, blockedGrid)) {
      continue;
    }
    const candidateWalls = convertSegmentsToWalls(segments);
    if (circleIntersectsAnyWall(cat.x, cat.y, cat.size / 2 + 2, candidateWalls)) {
      continue;
    }
    return candidateWalls;
  }
  return null;
}

function resolveEntityWallCollisions(entity, candidateWalls) {
  if (!entity || !candidateWalls || candidateWalls.length === 0) {
    return;
  }

  const radius = entity.size / 2;
  let iterations = 0;
  let moved = true;

  while (moved && iterations < 4) {
    moved = false;
    iterations += 1;
    for (const wall of candidateWalls) {
      const closestX = clamp(entity.x, wall.x, wall.x + wall.width);
      const closestY = clamp(entity.y, wall.y, wall.y + wall.height);
      let dx = entity.x - closestX;
      let dy = entity.y - closestY;
      let distanceSquared = dx * dx + dy * dy;
      if (distanceSquared < radius * radius) {
        if (distanceSquared === 0) {
          if (wall.width < wall.height) {
            dx = entity.x < wall.x + wall.width / 2 ? -1 : 1;
            dy = 0;
          } else {
            dx = 0;
            dy = entity.y < wall.y + wall.height / 2 ? -1 : 1;
          }
          distanceSquared = 1;
        }
        const distance = Math.sqrt(distanceSquared);
        const overlap = radius - distance;
        const nx = dx / distance;
        const ny = dy / distance;
        entity.x += nx * overlap;
        entity.y += ny * overlap;
        moved = true;
      }
    }
  }
}

function resolveCatWallCollisions() {
  resolveEntityWallCollisions(cat, walls);
}

function updateJoystickThumbPosition() {
  if (!joystickBase || !joystickThumb) {
    return;
  }
  const maxOffsetX = (joystickBase.clientWidth - joystickThumb.clientWidth) / 2;
  const maxOffsetY = (joystickBase.clientHeight - joystickThumb.clientHeight) / 2;
  const offsetX = joystickVector.x * maxOffsetX;
  const offsetY = joystickVector.y * maxOffsetY;
  joystickThumb.style.setProperty("--offset-x", `${offsetX}px`);
  joystickThumb.style.setProperty("--offset-y", `${offsetY}px`);
}

function setJoystickVector(rawX, rawY, { bypassDeadzone = false } = {}) {
  if (!joystickBase) {
    joystickVector.x = 0;
    joystickVector.y = 0;
    return;
  }

  const x = clamp(rawX, -1, 1);
  const y = clamp(rawY, -1, 1);

  if (bypassDeadzone) {
    joystickVector.x = x;
    joystickVector.y = y;
  } else {
    const magnitude = Math.hypot(x, y);
    if (magnitude < JOYSTICK_DEADZONE) {
      joystickVector.x = 0;
      joystickVector.y = 0;
    } else {
      const normalizedX = x / (magnitude || 1);
      const normalizedY = y / (magnitude || 1);
      const clampedMagnitude = Math.min(magnitude, 1);
      const scaledMagnitude =
        (clampedMagnitude - JOYSTICK_DEADZONE) / (1 - JOYSTICK_DEADZONE);
      const finalMagnitude = clamp(scaledMagnitude, 0, 1);
      joystickVector.x = normalizedX * finalMagnitude;
      joystickVector.y = normalizedY * finalMagnitude;
    }
  }

  updateJoystickThumbPosition();
  if (gameMode === "multiplayer" && multiplayerManager) {
    multiplayerManager.updateInputFromControls();
  }
}

function updateJoystickFromEvent(event) {
  if (!joystickBase) {
    return;
  }
  const rect = joystickBase.getBoundingClientRect();
  if (rect.width === 0 || rect.height === 0) {
    setJoystickVector(0, 0, { bypassDeadzone: true });
    return;
  }
  const relativeX = (event.clientX - (rect.left + rect.width / 2)) / (rect.width / 2);
  const relativeY = (event.clientY - (rect.top + rect.height / 2)) / (rect.height / 2);
  setJoystickVector(relativeX, relativeY);
}

function resetJoystick() {
  joystickPointerId = null;
  if (joystickBase) {
    joystickBase.classList.remove("dragging");
  }
  setJoystickVector(0, 0, { bypassDeadzone: true });
  if (gameMode === "multiplayer" && multiplayerManager) {
    multiplayerManager.updateInputFromControls();
  }
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
  updateJoystickThumbPosition();
}

function isSharedTimerMode() {
  return gameMode === "single" && singleTimerMode === TIMER_MODES.SHARED;
}

function isSurvivalMode() {
  return gameMode === "single" && singleTimerMode === TIMER_MODES.SURVIVAL;
}

function updateTimerDisplay(value = remaining) {
  if (timerEl) {
    timerEl.textContent = Math.max(value, 0).toFixed(1);
  }
}

function spawnFish() {
  const margin = 30;
  let shouldSpawnGolden = false;

  if (isSharedTimerMode()) {
    if (goldenChainActive && goldenChainRemaining > 0) {
      shouldSpawnGolden = true;
      goldenChainRemaining -= 1;
    } else {
      goldenChainActive = false;
      goldenChainRemaining = 0;
      if (Math.random() < GOLDEN_FISH_CHANCE) {
        shouldSpawnGolden = true;
        goldenChainActive = true;
        goldenChainRemaining = Math.floor(Math.random() * 5);
      }
    }
  } else {
    shouldSpawnGolden = Math.random() < GOLDEN_FISH_CHANCE;
  }

  fish.type = shouldSpawnGolden ? "golden" : "normal";
  fish.direction = Math.random() < 0.5 ? -1 : 1;
  const catCell = positionToGridCell(cat.x, cat.y);
  let placed = false;
  let attempts = 0;
  while (!placed && attempts < 180) {
    attempts += 1;
    const candidateX = margin + Math.random() * (WORLD_SIZE - margin * 2);
    const candidateY = margin + Math.random() * (WORLD_SIZE - margin * 2);
    const fishCell = positionToGridCell(candidateX, candidateY);
    if (fishCell.row === catCell.row && fishCell.col === catCell.col) {
      continue;
    }
    const candidateWalls = generateWallsLayout(catCell, fishCell);
    if (!candidateWalls) {
      continue;
    }
    if (circleIntersectsAnyWall(candidateX, candidateY, fish.size / 2 + 2, candidateWalls)) {
      continue;
    }
    walls = candidateWalls;
    if (
      powerUp.active &&
      circleIntersectsAnyWall(powerUp.x, powerUp.y, powerUp.size / 2 + 2)
    ) {
      powerUp.active = false;
      powerUp.remaining = 0;
    }
    resolveCatWallCollisions();
    cat.x = clamp(cat.x, cat.size / 2, WORLD_SIZE - cat.size / 2);
    cat.y = clamp(cat.y, cat.size / 2, WORLD_SIZE - cat.size / 2);
    fish.x = candidateX;
    fish.y = candidateY;
    placed = true;
  }

  if (!placed) {
    walls = [];
    fish.x = margin + Math.random() * (WORLD_SIZE - margin * 2);
    fish.y = margin + Math.random() * (WORLD_SIZE - margin * 2);
  }

  fish.alive = true;
  if (!isSharedTimerMode()) {
    remaining = getFishTimeLimitForFish(fish.type);
    updateTimerDisplay();
  }
  const mineCount = isSurvivalMode() ? survivalMineCount : null;
  spawnMines(mineCount);
  maybeSpawnPowerUp();
}

function resetGame() {
  updateBoardSize();
  score = 0;
  scoreEl.textContent = score;
  gameOver = false;
  scoreboardState.hasSavedCurrentScore = false;
  scoreboardState.isSaving = false;
  lastResultReason = null;
  messageEl.textContent = "";
  restartBtn.disabled = true;
  cat.x = WORLD_SIZE / 2;
  cat.y = WORLD_SIZE / 2;
  cat.moving = false;
  cat.walkCycle = 0;
  cat.stepAccumulator = 0;
  cat.facing = 1;
  goldenChainActive = false;
  goldenChainRemaining = 0;
  survivalMineCount = SURVIVAL_MINE_BASE_COUNT;
  powerUp.active = false;
  powerUp.remaining = 0;
  clearStatusEffect();
  clearMines();
  remaining = isSharedTimerMode() ? SHARED_TIMER_START : NORMAL_FISH_TIME_LIMIT;
  updateTimerDisplay();
  spawnFish();
  if (soundManager.enabled) {
    soundManager.startMusic();
  }
  lastTimestamp = performance.now();
  setScoreStatus(DEFAULT_SCORE_STATUS);
  updateResultsSummary();
  updateScoreFormControls();
  requestAnimationFrame(loop);
}

function endGame(reason) {
  gameOver = true;
  fish.alive = false;
  restartBtn.disabled = false;
  messageEl.textContent = reason + ` Результат: ${score}.`;
  powerUp.active = false;
  lastResultReason = reason;
  clearStatusEffect();
  updateBoardSize();
  soundManager.stopMusic(0.8);
  updateResultsSummary(reason);
  updateScoreFormControls();
  showResultsOverlay();
  if (playerNameInput) {
    playerNameInput.focus();
  }
  if (!scoreboardState.hasSavedCurrentScore) {
    setScoreStatus("Введите имя и сохраните результат.");
  }
}

function getRawInputVector() {
  let horizontalInput =
    (keys.has("ArrowRight") || keys.has("d") ? 1 : 0) -
    (keys.has("ArrowLeft") || keys.has("a") ? 1 : 0);
  let verticalInput =
    (keys.has("ArrowDown") || keys.has("s") ? 1 : 0) -
    (keys.has("ArrowUp") || keys.has("w") ? 1 : 0);

  horizontalInput += joystickVector.x;
  verticalInput += joystickVector.y;

  return { x: horizontalInput, y: verticalInput };
}

function update(delta) {
  const inputVector = getRawInputVector();
  let horizontalInput = inputVector.x;
  let verticalInput = inputVector.y;

  const length = Math.hypot(horizontalInput, verticalInput);
  const hasInput = length > 0.001;
  const speedMultiplier = getCatSpeedMultiplier();
  if (hasInput) {
    const cappedMagnitude = Math.min(length, 1);
    const normalizedX = horizontalInput / (length || 1);
    const normalizedY = verticalInput / (length || 1);
    const distance = cat.speed * speedMultiplier * delta * cappedMagnitude;
    const deltaX = normalizedX * distance;
    const deltaY = normalizedY * distance;

    if (deltaX !== 0) {
      cat.x += deltaX;
      resolveCatWallCollisions();
    }

    if (deltaY !== 0) {
      cat.y += deltaY;
      resolveCatWallCollisions();
    }
    cat.moving = true;
    if (Math.abs(normalizedX) > 0.1) {
      cat.facing = normalizedX >= 0 ? 1 : -1;
    }
    const walkIncrement =
      delta * WALK_FREQUENCY * cappedMagnitude * speedMultiplier;
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

  resolveCatWallCollisions();
  cat.x = clamp(cat.x, cat.size / 2, WORLD_SIZE - cat.size / 2);
  cat.y = clamp(cat.y, cat.size / 2, WORLD_SIZE - cat.size / 2);

  if (powerUp.active) {
    powerUp.remaining -= delta;
    if (powerUp.remaining <= 0) {
      powerUp.active = false;
      powerUp.remaining = 0;
    } else {
      const dxPower = cat.x - powerUp.x;
      const dyPower = cat.y - powerUp.y;
      const distanceToPower = Math.hypot(dxPower, dyPower);
      if (distanceToPower < (cat.size + powerUp.size) / 2) {
        powerUp.active = false;
        powerUp.remaining = 0;
        applyRandomStatusEffect();
        soundManager.playCatch();
      }
    }
  }

  if (fish.alive) {
    updateFishMovement(delta);

    const dx = cat.x - fish.x;
    const dy = cat.y - fish.y;
    const distanceToFish = Math.hypot(dx, dy);
    if (distanceToFish < (cat.size + fish.size) / 2) {
      const isGoldenFish = fish.type === "golden";
      score += isGoldenFish ? GOLDEN_FISH_POINTS : NORMAL_FISH_POINTS;
      scoreEl.textContent = score;
      if (isSharedTimerMode()) {
        if (isGoldenFish) {
          goldenChainActive = goldenChainRemaining > 0;
        } else {
          goldenChainActive = false;
          goldenChainRemaining = 0;
        }
      } else {
        goldenChainActive = false;
        goldenChainRemaining = 0;
      }
      if (isSharedTimerMode()) {
        remaining += isGoldenFish ? SHARED_TIMER_GOLDEN_BONUS : SHARED_TIMER_NORMAL_BONUS;
        updateTimerDisplay();
      }
      if (isSurvivalMode()) {
        survivalMineCount = Math.min(
          survivalMineCount + SURVIVAL_MINE_INCREMENT,
          SURVIVAL_MAX_MINES
        );
      }
      spawnFish();
      soundManager.playCatch();
    }
  }

  for (let i = mines.length - 1; i >= 0; i -= 1) {
    const mine = mines[i];
    const distanceToMine = Math.hypot(cat.x - mine.x, cat.y - mine.y);
    if (distanceToMine < (cat.size + mine.size) / 2) {
      if (isSharedTimerMode()) {
        mines.splice(i, 1);
        remaining = Math.max(remaining - SHARED_TIMER_MINE_PENALTY, 0);
        updateTimerDisplay();
        if (remaining <= 0) {
          endGame("Время вышло!");
          return;
        }
      } else {
        endGame("Котик подорвался на мине!");
        return;
      }
    }
  }

  if (activeStatusEffect) {
    activeStatusEffect.remaining -= delta;
    if (activeStatusEffect.remaining <= 0) {
      const endedEffect = activeStatusEffect.type;
      clearStatusEffect();
      if (
        !isSharedTimerMode() &&
        fish.alive &&
        (endedEffect === "timeIncrease" || endedEffect === "timeDecrease")
      ) {
        const baseLimit = getFishTimeLimitForFish(fish.type);
        if (endedEffect === "timeIncrease") {
          remaining = Math.min(remaining, baseLimit);
        } else {
          remaining = Math.max(remaining, baseLimit);
        }
      }
    }
  }

  remaining -= delta;
  const displayTime = Math.max(remaining, 0);
  updateTimerDisplay(displayTime);
  if (remaining <= 0) {
    if (isSharedTimerMode()) {
      endGame("Время вышло!");
      return;
    }
    const missedFishType = fish.type;
    if (missedFishType === "golden") {
      fish.alive = false;
      goldenChainActive = false;
      goldenChainRemaining = 0;
      spawnFish();
    } else {
      endGame("Котик не успел поймать рыбку!");
    }
  }
}

function drawBoot(x, y, color, style, size = CAT_BASE_SIZE, renderCtx = ctx) {
  const ctxRef = renderCtx;
  if (!ctxRef) return;
  ctxRef.save();
  ctxRef.translate(x, y);
  ctxRef.fillStyle = color;
  ctxRef.strokeStyle = "rgba(0, 0, 0, 0.12)";
  ctxRef.lineWidth = 1.2;
  const width = size * 0.18;
  const height = size * 0.1;
  ctxRef.beginPath();
  ctxRef.roundRect(-width / 2, -height / 2, width, height, 4);
  ctxRef.fill();
  ctxRef.stroke();

  if (style === "sneakers") {
    ctxRef.fillStyle = "rgba(255, 255, 255, 0.8)";
    ctxRef.fillRect(-width / 2 + 2, -2, width - 4, 3);
    ctxRef.fillRect(-width / 2 + 2, 2, width - 4, 2);
  } else if (style === "boots") {
    ctxRef.fillStyle = "rgba(0, 0, 0, 0.1)";
    ctxRef.fillRect(-width / 2, height / 2 - 2, width, 3);
  }
  ctxRef.restore();
}

function drawHat(style, color, size, renderCtx = ctx) {
  const ctxRef = renderCtx;
  if (!ctxRef) return;
  ctxRef.save();
  ctxRef.translate(size * 0.05, -size * 0.35);
  ctxRef.fillStyle = color;
  ctxRef.strokeStyle = "rgba(0, 0, 0, 0.14)";
  ctxRef.lineWidth = 1.2;

  if (style === "beanie") {
    ctxRef.beginPath();
    ctxRef.arc(0, 0, size * 0.22, Math.PI, 0);
    ctxRef.lineTo(size * 0.22, size * 0.04);
    ctxRef.lineTo(-size * 0.22, size * 0.04);
    ctxRef.closePath();
    ctxRef.fill();
    ctxRef.stroke();
    ctxRef.beginPath();
    ctxRef.arc(0, -size * 0.2, size * 0.05, 0, Math.PI * 2);
    ctxRef.fillStyle = "#fff";
    ctxRef.fill();
  } else if (style === "cap") {
    ctxRef.beginPath();
    ctxRef.arc(0, 0, size * 0.2, Math.PI, 0);
    ctxRef.fill();
    ctxRef.stroke();
    ctxRef.beginPath();
    ctxRef.ellipse(size * 0.08, size * 0.03, size * 0.16, size * 0.06, 0, 0, Math.PI * 2);
    ctxRef.fill();
  } else if (style === "crown") {
    ctxRef.beginPath();
    ctxRef.moveTo(-size * 0.16, size * 0.02);
    ctxRef.lineTo(-size * 0.08, -size * 0.18);
    ctxRef.lineTo(0, size * 0.02);
    ctxRef.lineTo(size * 0.08, -size * 0.18);
    ctxRef.lineTo(size * 0.16, size * 0.02);
    ctxRef.closePath();
    ctxRef.fill();
    ctxRef.stroke();
    ctxRef.fillStyle = "#ffe95b";
    ctxRef.beginPath();
    ctxRef.arc(-size * 0.08, -size * 0.18, size * 0.025, 0, Math.PI * 2);
    ctxRef.arc(size * 0.08, -size * 0.18, size * 0.025, 0, Math.PI * 2);
    ctxRef.fill();
  }

  ctxRef.restore();
}

function drawCatSprite(catState, targetCtx = ctx) {
  const renderCtx = targetCtx;
  if (!catState || !renderCtx) {
    return;
  }
  renderCtx.save();
  renderCtx.translate(catState.x, catState.y);
  renderCtx.scale(catState.facing ?? 1, 1);

  const appearance = sanitizeAppearance(catState.appearance);
  const baseColor = appearance.baseColor;
  const bellyColor = appearance.bellyColor;
  const eyeColor = appearance.eyeColor;
  const accessoryColor = appearance.accessoryColor;

  const cycle = (catState.walkCycle || 0) * Math.PI * 2;
  const isMoving = Boolean(catState.moving);
  const bobbing = isMoving ? Math.cos(cycle) * 2 : 0;

  // Tail
  renderCtx.save();
  renderCtx.translate(-catState.size * 0.45, -catState.size * 0.1 + bobbing * 0.2);
  const tailSwing = isMoving ? Math.sin(cycle + Math.PI / 2) * 8 : 0;
  renderCtx.rotate((tailSwing * Math.PI) / 180);
  renderCtx.fillStyle = baseColor;
  renderCtx.beginPath();
  renderCtx.ellipse(0, 0, catState.size * 0.35, catState.size * 0.12, 0, 0, Math.PI * 2);
  renderCtx.fill();
  renderCtx.restore();

  renderCtx.translate(0, bobbing);

  // Back legs
  renderCtx.fillStyle = baseColor;
  for (let i = -1; i <= 1; i += 2) {
    const swing = isMoving ? Math.sin(cycle + (i < 0 ? 0 : Math.PI)) * 4 : 0;
    const legX = i * catState.size * 0.23 + swing * 0.2;
    const legY = catState.size * 0.42;
    renderCtx.beginPath();
    renderCtx.ellipse(legX, legY, catState.size * 0.18, catState.size * 0.14, 0, 0, Math.PI * 2);
    renderCtx.fill();
    if (appearance.boots !== "none") {
      drawBoot(
        legX,
        legY + catState.size * 0.07,
        accessoryColor,
        appearance.boots,
        catState.size,
        renderCtx
      );
    }
  }

  // Body
  renderCtx.fillStyle = baseColor;
  renderCtx.beginPath();
  renderCtx.ellipse(0, 0, catState.size / 2, catState.size / 2.3, 0, 0, Math.PI * 2);
  renderCtx.fill();

  // Belly
  renderCtx.fillStyle = bellyColor;
  renderCtx.beginPath();
  renderCtx.ellipse(0, catState.size * 0.1, catState.size * 0.32, catState.size * 0.28, 0, 0, Math.PI * 2);
  renderCtx.fill();

  // Front legs
  renderCtx.fillStyle = baseColor;
  for (let i = -1; i <= 1; i += 2) {
    const phase = i < 0 ? Math.PI : 0;
    const swing = isMoving ? Math.sin(cycle + phase) * 4 : 0;
    const legX = i * catState.size * 0.25 - swing * 0.2;
    const legY = catState.size * 0.44;
    renderCtx.beginPath();
    renderCtx.ellipse(legX, legY, catState.size * 0.16, catState.size * 0.15, 0, 0, Math.PI * 2);
    renderCtx.fill();
    if (appearance.boots !== "none") {
      drawBoot(
        legX,
        legY + catState.size * 0.08,
        accessoryColor,
        appearance.boots,
        catState.size,
        renderCtx
      );
    }
  }

  // Head
  renderCtx.save();
  renderCtx.translate(catState.size * 0.26, -catState.size * 0.1);
  renderCtx.fillStyle = baseColor;
  renderCtx.beginPath();
  renderCtx.ellipse(0, 0, catState.size * 0.34, catState.size * 0.3, 0, 0, Math.PI * 2);
  renderCtx.fill();

  // Ears
  renderCtx.fillStyle = baseColor;
  renderCtx.beginPath();
  renderCtx.moveTo(-catState.size * 0.18, -catState.size * 0.22);
  renderCtx.lineTo(-catState.size * 0.08, -catState.size * 0.42);
  renderCtx.lineTo(0, -catState.size * 0.18);
  renderCtx.closePath();
  renderCtx.fill();

  renderCtx.beginPath();
  renderCtx.moveTo(catState.size * 0.05, -catState.size * 0.18);
  renderCtx.lineTo(catState.size * 0.18, -catState.size * 0.4);
  renderCtx.lineTo(catState.size * 0.2, -catState.size * 0.12);
  renderCtx.closePath();
  renderCtx.fill();

  // Eyes
  renderCtx.fillStyle = eyeColor;
  renderCtx.beginPath();
  renderCtx.ellipse(
    -catState.size * 0.04,
    -catState.size * 0.05,
    catState.size * 0.07,
    catState.size * 0.09,
    0,
    0,
    Math.PI * 2
  );
  renderCtx.ellipse(
    catState.size * 0.14,
    -catState.size * 0.05,
    catState.size * 0.07,
    catState.size * 0.09,
    0,
    0,
    Math.PI * 2
  );
  renderCtx.fill();

  // Muzzle
  renderCtx.fillStyle = "#ffe5b4";
  renderCtx.beginPath();
  renderCtx.arc(catState.size * 0.05, catState.size * 0.05, catState.size * 0.14, 0, Math.PI * 2);
  renderCtx.fill();

  // Nose and mouth
  renderCtx.fillStyle = accessoryColor;
  renderCtx.beginPath();
  renderCtx.moveTo(catState.size * 0.05, catState.size * 0.0);
  renderCtx.lineTo(catState.size * 0.02, catState.size * 0.05);
  renderCtx.lineTo(catState.size * 0.08, catState.size * 0.05);
  renderCtx.closePath();
  renderCtx.fill();

  renderCtx.strokeStyle = accessoryColor;
  renderCtx.lineWidth = 1.8;
  renderCtx.beginPath();
  renderCtx.moveTo(catState.size * 0.05, catState.size * 0.05);
  renderCtx.lineTo(catState.size * 0.05, catState.size * 0.09);
  renderCtx.moveTo(catState.size * 0.05, catState.size * 0.09);
  renderCtx.bezierCurveTo(
    catState.size * 0.0,
    catState.size * 0.12,
    -catState.size * 0.02,
    catState.size * 0.16,
    catState.size * 0.02,
    catState.size * 0.17
  );
  renderCtx.moveTo(catState.size * 0.05, catState.size * 0.09);
  renderCtx.bezierCurveTo(
    catState.size * 0.11,
    catState.size * 0.12,
    catState.size * 0.12,
    catState.size * 0.16,
    catState.size * 0.08,
    catState.size * 0.17
  );
  renderCtx.stroke();

  // Whiskers
  renderCtx.strokeStyle = "rgba(20, 54, 93, 0.7)";
  renderCtx.lineWidth = 1.4;
  renderCtx.beginPath();
  renderCtx.moveTo(-catState.size * 0.05, catState.size * 0.02);
  renderCtx.lineTo(-catState.size * 0.26, -catState.size * 0.03);
  renderCtx.moveTo(-catState.size * 0.04, catState.size * 0.07);
  renderCtx.lineTo(-catState.size * 0.24, catState.size * 0.12);
  renderCtx.moveTo(catState.size * 0.12, catState.size * 0.02);
  renderCtx.lineTo(catState.size * 0.32, -catState.size * 0.03);
  renderCtx.moveTo(catState.size * 0.13, catState.size * 0.07);
  renderCtx.lineTo(catState.size * 0.3, catState.size * 0.12);
  renderCtx.stroke();

  if (appearance.hat && appearance.hat !== "none") {
    drawHat(appearance.hat, accessoryColor, catState.size, renderCtx);
  }

  renderCtx.restore();

  renderCtx.restore();
}

function drawCat() {
  drawCatSprite(cat);
}

function drawFishSprite(fishState) {
  if (!fishState || !fishState.alive) return;
  ctx.save();
  ctx.translate(fishState.x, fishState.y);
  const facing = Math.sign(fishState.direction || 1) || 1;
  ctx.scale(facing, 1);
  const bodyColor = fishState.type === "golden" ? "#ffd700" : "#5cc8ff";
  const finColor = fishState.type === "golden" ? "#ffae00" : "#5cc8ff";
  ctx.fillStyle = bodyColor;
  ctx.beginPath();
  ctx.ellipse(0, 0, fishState.size / 2, fishState.size / 3, 0, 0, Math.PI * 2);
  ctx.fill();

  ctx.beginPath();
  ctx.fillStyle = finColor;
  ctx.moveTo(-fishState.size / 2, 0);
  ctx.lineTo(-fishState.size / 2 - 10, -fishState.size / 3);
  ctx.lineTo(-fishState.size / 2 - 10, fishState.size / 3);
  ctx.closePath();
  ctx.fill();

  ctx.fillStyle = "#14365d";
  ctx.beginPath();
  ctx.arc(fishState.size / 4, -fishState.size / 6, 3, 0, Math.PI * 2);
  ctx.fill();
  ctx.restore();
}

function drawFish() {
  drawFishSprite(fish);
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

function drawWallsCollection(targetWalls) {
  if (!targetWalls || targetWalls.length === 0) {
    return;
  }

  ctx.save();
  ctx.fillStyle = "#4a5a6a";
  ctx.strokeStyle = "#1f2a33";
  ctx.lineWidth = 4;
  targetWalls.forEach((wall) => {
    ctx.fillRect(wall.x, wall.y, wall.width, wall.height);
    ctx.strokeRect(wall.x, wall.y, wall.width, wall.height);
  });
  ctx.restore();
}

function drawWalls() {
  drawWallsCollection(walls);
}

function drawPowerUpSprite(powerUpState) {
  if (!powerUpState || !powerUpState.active) return;
  ctx.save();
  ctx.translate(powerUpState.x, powerUpState.y);
  const size = powerUpState.size;
  const half = size / 2;
  ctx.fillStyle = "#dba15d";
  ctx.strokeStyle = "#8b5a2b";
  ctx.lineWidth = 3;
  ctx.beginPath();
  ctx.rect(-half, -half, size, size);
  ctx.fill();
  ctx.stroke();

  ctx.strokeStyle = "rgba(99, 55, 15, 0.6)";
  ctx.lineWidth = 2;
  ctx.beginPath();
  ctx.moveTo(-half, -half);
  ctx.lineTo(half, half);
  ctx.moveTo(-half, half);
  ctx.lineTo(half, -half);
  ctx.moveTo(-half, 0);
  ctx.lineTo(half, 0);
  ctx.moveTo(0, -half);
  ctx.lineTo(0, half);
  ctx.stroke();
  ctx.restore();
}

function drawPowerUp() {
  drawPowerUpSprite(powerUp);
}

function drawMinesCollection(targetMines) {
  if (!targetMines || targetMines.length === 0) {
    return;
  }

  ctx.save();
  targetMines.forEach((mine) => {
    ctx.save();
    ctx.translate(mine.x, mine.y);
    const radius = mine.size / 2;

    const gradient = ctx.createRadialGradient(0, 0, radius * 0.3, 0, 0, radius);
    gradient.addColorStop(0, "#ff5252");
    gradient.addColorStop(1, "#5a1b1b");

    ctx.fillStyle = gradient;
    ctx.beginPath();
    ctx.arc(0, 0, radius, 0, Math.PI * 2);
    ctx.fill();

    ctx.strokeStyle = "#2b0d0d";
    ctx.lineWidth = 3;
    ctx.beginPath();
    for (let i = 0; i < 8; i += 1) {
      const angle = (i / 8) * Math.PI * 2;
      const inner = radius * 0.3;
      const outer = radius * 1.1;
      ctx.moveTo(Math.cos(angle) * inner, Math.sin(angle) * inner);
      ctx.lineTo(Math.cos(angle) * outer, Math.sin(angle) * outer);
    }
    ctx.stroke();

    ctx.restore();
  });
  ctx.restore();
}

function drawMines() {
  drawMinesCollection(mines);
}

function generateClientId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `player-${Math.random().toString(36).slice(2, 10)}`;
}

function createMultiplayerServerDependencies() {
  return {
    WORLD_SIZE,
    FISH_BASE_SIZE,
    FISH_SWIM_SPEED,
    POWER_UP_BASE_SIZE,
    NORMAL_FISH_TIME_LIMIT,
    createSeededRng,
    CAT_BASE_SIZE,
    GRID_CELL_SIZE,
    GRID_SIZE,
    WALL_THICKNESS,
    circleIntersectsRect,
    circleIntersectsAnyWall,
    MAX_MINES,
    MINE_SIZE,
    MINE_MIN_DISTANCE,
    MAX_WALL_TOTAL_LENGTH,
    getCellsForSegment,
    buildBlockedGridFromSegments,
    isPathAvailable,
    convertSegmentsToWalls,
    positionToGridCell,
    clamp,
    CAT_BASE_SPEED,
    WALK_FREQUENCY,
    resolveEntityWallCollisions,
    GOLDEN_FISH_POINTS,
    NORMAL_FISH_POINTS,
    GOLDEN_FISH_CHANCE,
    POWER_UP_CHANCE,
    POWER_UP_LIFETIME,
    STATUS_EFFECT_TYPES,
    POWER_UP_DURATION,
    TIME_INCREASE_LIMIT,
    TIME_DECREASE_LIMIT,
    GOLDEN_FISH_TIME_LIMIT,
    sanitizeAppearance
  };
}

class MultiplayerLobby {
  constructor() {
    this.rooms = new Map();
    this.listeners = new Set();
    this.pollInterval = null;
  }

  addListener(listener) {
    if (listener) {
      this.listeners.add(listener);
    }
  }

  notifyListeners() {
    this.listeners.forEach((listener) => {
      try {
        listener(this.getRooms());
      } catch (error) {
        console.warn("Ошибка обработчика лобби", error);
      }
    });
  }

  getRooms() {
    return Array.from(this.rooms.values()).sort((a, b) => (b.updatedAt || 0) - (a.updatedAt || 0));
  }

  async connect() {
    if (this.pollInterval) {
      return;
    }
    await this.requestSync();
    this.pollInterval = setInterval(() => this.requestSync(), 5000);
  }

  async disconnect() {
    if (this.pollInterval) {
      clearInterval(this.pollInterval);
      this.pollInterval = null;
    }
  }

  async requestSync() {
    try {
      const response = await apiRequest("/api/rooms");
      const rooms = Array.isArray(response?.rooms) ? response.rooms : response || [];
      this.rooms.clear();
      rooms.forEach((room) => {
        if (room?.roomName) {
          this.rooms.set(room.roomName, room);
        }
      });
      this.notifyListeners();
    } catch (error) {
      console.warn("Не удалось обновить список комнат", error);
    }
  }

  registerActiveRoom() {}

  unregisterActiveRoom() {}
}

function applyMultiplayerStatePatch(previousState, patch) {
  if (!previousState || !patch) {
    return previousState;
  }
  const nextState = { ...previousState };
  if (patch.serverTime !== undefined) {
    nextState.serverTime = patch.serverTime;
  }
  if (patch.tickIndex !== undefined) {
    nextState.tickIndex = patch.tickIndex;
  }
  if (patch.phase !== undefined) {
    nextState.phase = patch.phase;
  }
  if (patch.countdown !== undefined) {
    nextState.countdown = patch.countdown;
  }
  if (patch.remaining !== undefined) {
    nextState.remaining = patch.remaining;
  }
  if (patch.goldenChainActive !== undefined) {
    nextState.goldenChainActive = patch.goldenChainActive;
  }
  if (patch.message !== undefined) {
    nextState.message = patch.message;
  }
  if (patch.winnerId !== undefined) {
    nextState.winnerId = patch.winnerId;
  }
  if (patch.statusEffect !== undefined) {
    nextState.statusEffect = patch.statusEffect || null;
  }
  if (patch.fish) {
    nextState.fish = { ...previousState.fish, ...patch.fish };
  }
  if (patch.powerUp) {
    nextState.powerUp = { ...previousState.powerUp, ...patch.powerUp };
  }
  if (Array.isArray(patch.walls)) {
    nextState.walls = patch.walls.map((wall) => ({ ...wall }));
  }
  if (Array.isArray(patch.mines)) {
    nextState.mines = patch.mines.map((mine) => ({ ...mine }));
  }

  const playerMap = new Map((previousState.players || []).map((player) => [player.id, { ...player }]));
  if (Array.isArray(patch.removedPlayers)) {
    patch.removedPlayers.forEach((id) => playerMap.delete(id));
  }
  if (Array.isArray(patch.players)) {
    patch.players.forEach((update) => {
      if (!update?.id) {
        return;
      }
      const existing = playerMap.get(update.id) || { id: update.id };
      const merged = { ...existing };
      Object.entries(update).forEach(([key, value]) => {
        if (key === "id") {
          return;
        }
        if (value !== undefined) {
          merged[key] = value;
        }
      });
      playerMap.set(update.id, merged);
    });
  }
  nextState.players = Array.from(playerMap.values());

  return nextState;
}

class MultiplayerManager {
  constructor(lobby) {
    this.playerId = playerId;
    this.playerName = "";
    this.roomName = "";
    this.socket = null;
    this.state = null;
    this.previousRenderState = null;
    this.smoothingStartTime = 0;
    this.smoothingDuration = 1000 / 15;
    this.ready = false;
    this.inputVector = { x: 0, y: 0 };
    this.lastInputSentAt = 0;
    this.rendering = false;
    this.renderFrameBound = (timestamp) => this.renderFrame(timestamp);
    this.chatMessages = [];
    this.lobby = lobby;
    this.lastLocalStepAccumulator = 0;
  }

  async join(roomName, playerName) {
    this.roomName = roomName;
    this.playerName = playerName;
    this.ready = false;
    this.resetChat();

    const params = new URLSearchParams({
      room: roomName,
      playerId: this.playerId,
      name: playerName
    });
    const socketUrl = `${WS_BASE_URL}/ws?${params.toString()}`;
    await this.leave();

    this.socket = new WebSocket(socketUrl);
    this.socket.addEventListener("open", () => {
      this.sendMessage({ type: "appearance", appearance: sanitizeAppearance(cat.appearance) });
      this.sendMessage({ type: "ready", ready: this.ready });
      this.updateReadyButton();
    });
    this.socket.addEventListener("message", async (event) => {
      try {
        if (typeof event.data === "string") {
          const payload = JSON.parse(event.data);
          this.handleSocketMessage(payload);
        } else if (event.data instanceof Blob) {
          const buffer = await event.data.arrayBuffer();
          this.handleBinaryMessage(buffer);
        } else if (event.data instanceof ArrayBuffer) {
          this.handleBinaryMessage(event.data);
        }
      } catch (error) {
        console.warn("Ошибка обработки сообщения сервера", error);
      }
    });
    this.socket.addEventListener("close", () => {
      this.socket = null;
      this.updateReadyButton();
      this.updateHud();
    });
    await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error("Подключение заняло слишком много времени")), 4000);
      const handleOpen = () => {
        clearTimeout(timer);
        resolve();
      };
      const handleError = () => {
        clearTimeout(timer);
        reject(new Error("Не удалось подключиться к серверу"));
      };
      this.socket?.addEventListener("open", handleOpen, { once: true });
      this.socket?.addEventListener("error", handleError, { once: true });
    });
  }

  async leave() {
    this.stopRenderLoop();
    if (this.socket) {
      this.socket.close();
    }
    this.socket = null;
    this.state = null;
    this.previousRenderState = null;
    this.smoothingStartTime = 0;
    this.ready = false;
    this.updateReadyButton();
    this.updateHud();
  }

  sendMessage(payload) {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      return;
    }
    this.socket.send(JSON.stringify(payload));
  }

  sendBinary(buffer) {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN || !buffer) {
      return;
    }
    this.socket.send(buffer);
  }

  handleSocketMessage(message) {
    switch (message?.type) {
      case "state":
        this.handleServerState(message);
        break;
      case "chat":
        this.handleChatMessage(message.message || message);
        break;
      case "error":
        if (multiplayerErrorEl) {
          multiplayerErrorEl.textContent = message.message || "Не удалось подключиться.";
        }
        break;
      default:
        break;
    }
  }

  handleBinaryMessage(buffer) {
    if (!buffer) {
      return;
    }
    const payload = decodeStateFromBase64(buffer);
    if (payload?.state || payload?.patch) {
      this.handleServerState(payload);
    }
  }

  handleServerState(payload) {
    if (!payload) {
      return;
    }
    const previousState = this.state;
    let nextState = null;
    if (payload.patch && this.state) {
      nextState = applyMultiplayerStatePatch(this.state, payload.patch);
    } else if (payload.state) {
      nextState = payload.state;
    } else if (!payload.patch) {
      nextState = payload;
    }
    if (!nextState) {
      return;
    }
    const now = performance.now();
    this.previousRenderState = previousState || nextState;
    this.smoothingStartTime = now;
    this.state = nextState;
    this.handleStateAudio(previousState, nextState);
    syncMultiplayerStatusEffect(nextState.statusEffect);
    if (nextState.phase === "playing" || nextState.phase === "countdown") {
      hideMultiplayerOverlay();
      hideMultiplayerChat({ reset: false });
      hideRestartButton();
    } else if (nextState.phase === "ended") {
      showMultiplayerOverlay();
      showMultiplayerLobby();
      showMultiplayerChat(this.roomName);
      showRestartButton();
    }
    this.ready = Boolean(nextState.players?.find((player) => player.id === this.playerId)?.ready);
    this.updateReadyButton();
    this.updateHud();
    this.updateLobbyUI();
    this.ensureRender();
  }

  handleStateAudio(previousState, nextState) {
    const wasActivePhase =
      previousState?.phase === "playing" || previousState?.phase === "countdown";
    const isActivePhase =
      nextState?.phase === "playing" || nextState?.phase === "countdown";

    if (isActivePhase && !wasActivePhase) {
      ensureAudioActive();
      soundManager.startMusic();
    } else if (!isActivePhase && wasActivePhase) {
      soundManager.stopMusic(0.6);
    }

    const selfId = this.playerId;
    const previousPlayer = previousState?.players?.find((player) => player.id === selfId);
    const currentPlayer = nextState?.players?.find((player) => player.id === selfId);

    if (currentPlayer) {
      const previousScore = previousPlayer?.score ?? 0;
      if (currentPlayer.score > previousScore) {
        soundManager.playCatch();
      }

      const prevStep = previousPlayer?.stepAccumulator ?? this.lastLocalStepAccumulator ?? 0;
      const nextStep = currentPlayer.stepAccumulator ?? 0;
      const stepWrapped =
        currentPlayer.moving &&
        (nextStep < prevStep - 0.25 || (prevStep > 0.25 && nextStep < prevStep));
      if (stepWrapped) {
        soundManager.playStep();
      }
      this.lastLocalStepAccumulator = nextStep;
    } else {
      this.lastLocalStepAccumulator = 0;
    }

    const prevEffectType = previousState?.statusEffect?.type;
    const nextEffectType = nextState?.statusEffect?.type;
    if (nextEffectType && nextEffectType !== prevEffectType) {
      soundManager.playCatch();
    }
  }

  ensureRender() {
    if (this.rendering) {
      return;
    }
    this.rendering = true;
    multiplayerRenderHandle = requestAnimationFrame(this.renderFrameBound);
  }

  stopRenderLoop() {
    this.rendering = false;
    if (multiplayerRenderHandle) {
      cancelAnimationFrame(multiplayerRenderHandle);
      multiplayerRenderHandle = null;
    }
  }

  renderFrame() {
    if (!this.rendering) {
      return;
    }
    if (gameMode !== "multiplayer") {
      this.stopRenderLoop();
      return;
    }
    const elapsed = performance.now() - this.smoothingStartTime;
    const progress = this.smoothingDuration > 0 ? clamp(elapsed / this.smoothingDuration, 0, 1) : 1;
    prepareCanvasForFrame();
    if (this.state) {
      const players = this.getInterpolatedPlayers(progress);
      drawWallsCollection(this.state.walls || []);
      drawMinesCollection(this.state.mines || []);
      drawPowerUpSprite(this.state.powerUp);
      drawFishSprite(this.state.fish);
      players.forEach((player) => {
        drawCatSprite(player);
      });
    }
    multiplayerRenderHandle = requestAnimationFrame(this.renderFrameBound);
  }

  getInterpolatedPlayers(progress) {
    if (!this.state) {
      return [];
    }
    const clampedProgress = clamp(progress ?? 1, 0, 1);
    const currentPlayers = this.state.players || [];
    const previousPlayers = this.previousRenderState?.players || [];
    if (previousPlayers.length === 0 || currentPlayers.length === 0) {
      return currentPlayers;
    }
    const previousById = new Map(previousPlayers.map((player) => [player.id, player]));
    return currentPlayers.map((player) => {
      const previous = previousById.get(player.id);
      if (!previous) {
        return player;
      }
      const interpolateValue = (from, to) => from + (to - from) * clampedProgress;
      let stepAccumulator = player.stepAccumulator;
      if (typeof previous.stepAccumulator === "number" && typeof player.stepAccumulator === "number") {
        let diff = player.stepAccumulator - previous.stepAccumulator;
        if (diff > 0.5) {
          diff -= 1;
        } else if (diff < -0.5) {
          diff += 1;
        }
        stepAccumulator = previous.stepAccumulator + diff * clampedProgress;
      }

      return {
        ...player,
        x: interpolateValue(previous.x ?? player.x, player.x ?? previous.x),
        y: interpolateValue(previous.y ?? player.y, player.y ?? previous.y),
        size: interpolateValue(previous.size ?? player.size, player.size ?? previous.size),
        walkCycle: interpolateValue(previous.walkCycle ?? player.walkCycle, player.walkCycle ?? previous.walkCycle),
        stepAccumulator
      };
    });
  }

  toggleReady() {
    this.ready = !this.ready;
    this.updateReadyButton();
    this.sendMessage({ type: "ready", ready: this.ready });
  }

  updateAppearance(appearance) {
    if (!appearance) {
      return;
    }
    const sanitized = sanitizeAppearance(appearance);
    this.sendMessage({ type: "appearance", appearance: sanitized });
  }

  updateReadyButton() {
    if (multiplayerReadyBtn) {
      multiplayerReadyBtn.textContent = this.ready ? "Не готов" : "Готов";
      multiplayerReadyBtn.disabled = !this.socket || this.socket.readyState !== WebSocket.OPEN;
    }
  }

  resetChat() {
    this.chatMessages = [];
    renderChatMessages(this.chatMessages);
  }

  handleChatMessage(payload) {
    if (!payload || !payload.text) {
      return;
    }
    const entry = {
      playerId: payload.playerId,
      name: payload.name || "Игрок",
      text: String(payload.text).slice(0, 240),
      at: payload.at || Date.now()
    };
    this.chatMessages.push(entry);
    if (this.chatMessages.length > 60) {
      this.chatMessages = this.chatMessages.slice(-60);
    }
    renderChatMessages(this.chatMessages);
  }

  sendChatMessage(text) {
    const trimmed = text.trim();
    if (!trimmed) {
      return;
    }
    const payload = {
      playerId: this.playerId,
      name: this.playerName || "Игрок",
      text: trimmed.slice(0, 240),
      at: Date.now()
    };
    this.handleChatMessage(payload);
    this.sendMessage({ type: "chat", message: payload });
  }

  sendInput(vector) {
    this.sendBinary(encodeInputToBuffer(this.playerId, vector));
  }

  updateInputFromControls() {
    const raw = getRawInputVector();
    const vector = { x: raw.x, y: raw.y };
    const changed =
      Math.abs(vector.x - this.inputVector.x) > 0.02 ||
      Math.abs(vector.y - this.inputVector.y) > 0.02;
    const now = performance.now();
    if (changed || now - this.lastInputSentAt > 120) {
      this.inputVector = vector;
      this.lastInputSentAt = now;
      this.sendInput(vector);
    }
  }

  updateLobbyUI() {
    if (!multiplayerLobbyCard || multiplayerLobbyCard.classList.contains("hidden")) {
      return;
    }
    if (multiplayerRoomLabel) {
      multiplayerRoomLabel.textContent = this.roomName;
    }
    const players = this.state?.players || [];
    if (multiplayerPlayerList) {
      multiplayerPlayerList.innerHTML = "";
      players.forEach((player) => {
        const statusText = player.ready ? "Готов" : "Не готов";
        const item = document.createElement("li");
        item.innerHTML = `<span>${escapeHtml(player.name)}</span><span>${statusText}</span>`;
        multiplayerPlayerList.appendChild(item);
      });
    }
    if (multiplayerStatusEl) {
      let message = this.state?.message || "Подождите немного, идёт подключение";
      if (this.state?.phase === "ended" && players.length > 0) {
        const sorted = [...players].sort((a, b) => b.score - a.score);
        const winner = this.state.winnerId
          ? sorted.find((player) => player.id === this.state.winnerId)
          : sorted[0];
        const winnerText = winner
          ? `Победитель: ${winner.name} (${winner.score})`
          : "Победитель не определён";
        const scoresText = sorted.map((player) => `${player.name}: ${player.score}`).join(", ");
        message = `${message}. ${winnerText}. Итоги — ${scoresText}`;
      }
      multiplayerStatusEl.textContent = message;
    }
  }

  updateHud() {
    if (!multiplayerHud) {
      return;
    }
    if (!this.state) {
      multiplayerHud.classList.add("hidden");
      return;
    }
    if (multiplayerHudRoom) {
      multiplayerHudRoom.textContent = this.roomName;
    }
    const phase = this.state.phase;
    const shouldShowHud = phase === "countdown" || phase === "playing" || phase === "ended";
    multiplayerHud.classList.toggle("hidden", !shouldShowHud);
    if (shouldShowHud && multiplayerHudPlayers) {
      multiplayerHudPlayers.innerHTML = "";
      const sorted = [...(this.state.players || [])].sort((a, b) => b.score - a.score);
      sorted.forEach((player) => {
        const item = document.createElement("li");
        let status = "Ждёт";
        if (phase === "playing") {
          status = player.alive ? "В игре" : "Выбыл";
        } else if (phase === "countdown") {
          status = "Готовится";
        } else if (phase === "ended") {
          status = this.state.winnerId === player.id ? "Победитель" : "Итог";
        }
        item.innerHTML = `<span>${escapeHtml(player.name)}</span><span>${player.score} · ${status}</span>`;
        multiplayerHudPlayers.appendChild(item);
      });
    }
    if (multiplayerCountdownEl) {
      if (phase === "countdown") {
        multiplayerCountdownEl.textContent = `Старт через ${Math.ceil(this.state.countdown || 0)} с`;
      } else if (phase === "playing") {
        multiplayerCountdownEl.textContent = `Время: ${(this.state.remaining || 0).toFixed(1)} c`;
      } else if (phase === "ended") {
        multiplayerCountdownEl.textContent = "Раунд завершён";
      } else {
        multiplayerCountdownEl.textContent = "";
      }
    }
    if (multiplayerGameMessage) {
      multiplayerGameMessage.textContent = this.state.message || "";
    }

    const localPlayer = (this.state.players || []).find((player) => player.id === this.playerId);
    if (scoreEl && localPlayer) {
      scoreEl.textContent = localPlayer.score;
    } else if (scoreEl && !localPlayer) {
      scoreEl.textContent = "0";
    }
    if (timerEl) {
      const timerValue = phase === "countdown" ? this.state.countdown : this.state.remaining;
      timerEl.textContent = timerValue != null ? Math.max(timerValue, 0).toFixed(1) : "0.0";
    }
  }
}

multiplayerLobby = new MultiplayerLobby();
multiplayerLobby.addListener(() => renderMultiplayerRoomList());


function loop(timestamp) {
  if (gameMode !== "single" || gameOver) return;

  const delta = (timestamp - lastTimestamp) / 1000;
  lastTimestamp = timestamp;
  prepareCanvasForFrame();
  update(delta);
  drawWalls();
  drawMines();
  drawPowerUp();
  drawFish();
  drawCat();
  requestAnimationFrame(loop);
}

window.addEventListener("keydown", (event) => {
  const key = normalizeKey(event.key);
  if (!key) {
    return;
  }
  if (directionKeys.has(key)) {
    event.preventDefault();
  }
  ensureAudioActive();
  keys.add(key);
  if (gameMode === "multiplayer" && multiplayerManager) {
    multiplayerManager.updateInputFromControls();
  }
});

window.addEventListener("keyup", (event) => {
  const key = normalizeKey(event.key);
  if (!key) {
    return;
  }
  keys.delete(key);
  if (gameMode === "multiplayer" && multiplayerManager) {
    multiplayerManager.updateInputFromControls();
  }
});

if (openMenuBtn) {
  openMenuBtn.addEventListener("click", () => {
    showMainMenu();
  });
}

if (menuPlayBtn) {
  menuPlayBtn.addEventListener("click", () => {
    showModeSelection();
  });
}

if (menuResultsBtn) {
  menuResultsBtn.addEventListener("click", () => {
    showResultsOverlay();
  });
}

if (menuAppearanceBtn) {
  menuAppearanceBtn.addEventListener("click", () => {
    showAppearanceOverlay();
  });
}

if (menuSettingsBtn) {
  menuSettingsBtn.addEventListener("click", () => {
    showSettingsOverlay();
  });
}

if (resultsBackBtn) {
  resultsBackBtn.addEventListener("click", () => {
    hideResultsOverlay();
    showMainMenu();
  });
}

if (settingsBackBtn) {
  settingsBackBtn.addEventListener("click", () => {
    hideSettingsOverlay();
    showMainMenu();
  });
}

if (appearanceBackBtn) {
  appearanceBackBtn.addEventListener("click", () => {
    hideAppearanceOverlay();
    showMainMenu();
  });
}

if (soundToggleInput) {
  soundToggleInput.checked = initialSoundEnabled;
  soundToggleInput.addEventListener("change", () => {
    const enabled = Boolean(soundToggleInput.checked);
    soundManager.setSfxEnabled(enabled);
    safeStoreBoolean(SOUND_ENABLED_STORAGE_KEY, enabled);
    if (enabled) {
      ensureAudioActive();
    }
  });
}

if (musicToggleInput) {
  musicToggleInput.checked = initialMusicEnabled;
  musicToggleInput.addEventListener("change", () => {
    const enabled = Boolean(musicToggleInput.checked);
    soundManager.setMusicEnabled(enabled);
    safeStoreBoolean(MUSIC_ENABLED_STORAGE_KEY, enabled);
    if (enabled) {
      ensureAudioActive();
    }
  });
}

if (catColorBaseInput) {
  catColorBaseInput.addEventListener("input", () => {
    updateCatAppearance({ baseColor: catColorBaseInput.value });
  });
}

if (catColorBellyInput) {
  catColorBellyInput.addEventListener("input", () => {
    updateCatAppearance({ bellyColor: catColorBellyInput.value });
  });
}

if (catColorEyesInput) {
  catColorEyesInput.addEventListener("input", () => {
    updateCatAppearance({ eyeColor: catColorEyesInput.value });
  });
}

if (catColorAccessoryInput) {
  catColorAccessoryInput.addEventListener("input", () => {
    updateCatAppearance({ accessoryColor: catColorAccessoryInput.value });
  });
}

if (catHatSelect) {
  catHatSelect.addEventListener("change", () => {
    updateCatAppearance({ hat: catHatSelect.value });
  });
}

if (catBootsSelect) {
  catBootsSelect.addEventListener("change", () => {
    updateCatAppearance({ boots: catBootsSelect.value });
  });
}

restartBtn.addEventListener("click", () => {
  ensureAudioActive();
  if (gameMode === "single" && gameOver) {
    openModeSelection();
    return;
  }

  if (
    gameMode === "multiplayer" &&
    multiplayerManager &&
    multiplayerManager.state?.phase === "ended"
  ) {
    showMultiplayerOverlay();
    showMultiplayerLobby();
    multiplayerManager.updateLobbyUI();
    return;
  }

  openModeSelection();
});

if (startSingleBtn) {
  startSingleBtn.addEventListener("click", () => {
    startSingleMode();
  });
}

if (startSingleSharedBtn) {
  startSingleSharedBtn.addEventListener("click", () => {
    startSingleMode(TIMER_MODES.SHARED);
  });
}

if (startSurvivalBtn) {
  startSurvivalBtn.addEventListener("click", () => {
    startSingleMode(TIMER_MODES.SURVIVAL);
  });
}

if (startMultiplayerBtn) {
  startMultiplayerBtn.addEventListener("click", () => {
    hideModeSelection();
    showMultiplayerJoinForm();
  });
}

if (multiplayerRoomRefreshBtn) {
  multiplayerRoomRefreshBtn.addEventListener("click", () => {
    ensureLobbyConnected();
  });
}

if (multiplayerRoomList) {
  multiplayerRoomList.addEventListener("click", (event) => {
    const target = event.target;
    if (!(target instanceof HTMLElement)) {
      return;
    }
    const roomName = target.dataset?.roomName;
    if (!roomName) {
      return;
    }
    if (multiplayerRoomInput) {
      multiplayerRoomInput.value = roomName;
    }
    if (multiplayerNameInput && !multiplayerNameInput.value && playerNameInput?.value) {
      multiplayerNameInput.value = playerNameInput.value;
    }
    if (multiplayerJoinForm?.requestSubmit) {
      multiplayerJoinForm.requestSubmit();
    } else if (multiplayerJoinForm) {
      multiplayerJoinForm.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    }
  });
}

if (multiplayerCancelBtn) {
  multiplayerCancelBtn.addEventListener("click", async () => {
    await leaveMultiplayerRoom({ backToMenu: true });
    hideMultiplayerOverlay();
    showMainMenu();
  });
}

let multiplayerJoinInProgress = false;
if (multiplayerJoinForm) {
  multiplayerJoinForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (multiplayerJoinInProgress) {
      return;
    }
    const rawName = multiplayerNameInput ? multiplayerNameInput.value.trim() : "";
    const rawRoom = multiplayerRoomInput ? multiplayerRoomInput.value.trim() : "";
    if (!rawName) {
      multiplayerErrorEl.textContent = "Введите имя игрока.";
      if (multiplayerNameInput) {
        multiplayerNameInput.focus();
      }
      return;
    }
    if (!rawRoom) {
      multiplayerErrorEl.textContent = "Введите название комнаты.";
      if (multiplayerRoomInput) {
        multiplayerRoomInput.focus();
      }
      return;
    }

    const normalizedName = rawName.slice(0, 32);
    const normalizedRoom = rawRoom.slice(0, 32);

    const existingRoom = multiplayerLobby
      ?.getRooms()
      .find((room) => room.roomName.toLowerCase() === normalizedRoom.toLowerCase());
    if (existingRoom && (existingRoom.phase === "playing" || existingRoom.phase === "countdown")) {
      multiplayerErrorEl.textContent = "К этой комнате нельзя подключиться во время игры.";
      return;
    }

    multiplayerJoinInProgress = true;
    multiplayerErrorEl.textContent = "";
    try {
      await joinMultiplayerRoom(normalizedRoom, normalizedName);
      if (playerNameInput) {
        playerNameInput.value = normalizedName;
      }
    } catch (error) {
      console.error("Не удалось подключиться к комнате", error);
      multiplayerErrorEl.textContent = "Не удалось подключиться. Попробуйте ещё раз.";
    } finally {
      multiplayerJoinInProgress = false;
    }
  });
}

if (multiplayerReadyBtn) {
  multiplayerReadyBtn.addEventListener("click", () => {
    if (gameMode !== "multiplayer" || !multiplayerManager) {
      return;
    }
    multiplayerManager.toggleReady();
  });
}

if (multiplayerLeaveBtn) {
  multiplayerLeaveBtn.addEventListener("click", async () => {
    await leaveMultiplayerRoom({ backToMenu: true });
    hideMultiplayerOverlay();
    showModeSelection();
  });
}

if (multiplayerChatForm) {
  multiplayerChatForm.addEventListener("submit", (event) => {
    event.preventDefault();
    if (gameMode !== "multiplayer" || !multiplayerManager) {
      return;
    }
    const text = multiplayerChatInput ? multiplayerChatInput.value.trim() : "";
    if (!text) {
      return;
    }
    multiplayerManager.sendChatMessage(text);
    if (multiplayerChatInput) {
      multiplayerChatInput.value = "";
      multiplayerChatInput.focus();
    }
  });
}

if (submitScoreForm) {
  submitScoreForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!gameOver) {
      setScoreStatus("Завершите игру, чтобы сохранить результат.");
      return;
    }

    if (scoreboardState.hasSavedCurrentScore || scoreboardState.isSaving) {
      return;
    }

    const rawName = playerNameInput ? playerNameInput.value.trim() : "";
    if (!rawName) {
      setScoreStatus("Введите имя перед сохранением.");
      if (playerNameInput) {
        playerNameInput.focus();
      }
      return;
    }

    const normalizedName = rawName.slice(0, 32);

    scoreboardState.isSaving = true;
    updateScoreFormControls();
    setScoreStatus("Сохраняем результат...");

    try {
      const safeScore = Math.max(0, Math.floor(score));
      const payload = { name: normalizedName, score: safeScore, playerId };
      await apiRequest("/api/scores", { method: "POST", body: payload });
      scoreboardState.hasSavedCurrentScore = true;
      setScoreStatus("Результат сохранён!");
      safeStoreName(normalizedName);
      fetchLeaderboard();
    } catch (error) {
      console.error("Не удалось сохранить результат", error);
      setScoreStatus("Не удалось сохранить результат. Попробуйте ещё раз.");
    } finally {
      scoreboardState.isSaving = false;
      updateScoreFormControls();
    }
  });
}

if (joystickBase) {
  const handlePointerMove = (event) => {
    if (joystickPointerId !== event.pointerId) {
      return;
    }
    event.preventDefault();
    updateJoystickFromEvent(event);
  };

  joystickBase.addEventListener("pointerdown", (event) => {
    if (joystickPointerId !== null && joystickPointerId !== event.pointerId) {
      return;
    }
    joystickPointerId = event.pointerId;
    ensureAudioActive();
    joystickBase.classList.add("dragging");
    if (joystickBase.setPointerCapture) {
      joystickBase.setPointerCapture(event.pointerId);
    }
    updateJoystickFromEvent(event);
    event.preventDefault();
  });

  joystickBase.addEventListener("pointermove", handlePointerMove);

  const finishPointerInteraction = (event) => {
    if (joystickPointerId !== event.pointerId) {
      return;
    }
    if (joystickBase.releasePointerCapture) {
      joystickBase.releasePointerCapture(event.pointerId);
    }
    resetJoystick();
  };

  joystickBase.addEventListener("pointerup", (event) => {
    event.preventDefault();
    finishPointerInteraction(event);
  });

  joystickBase.addEventListener("pointercancel", (event) => {
    event.preventDefault();
    finishPointerInteraction(event);
  });

  joystickBase.addEventListener("lostpointercapture", () => {
    resetJoystick();
  });

  joystickBase.addEventListener("contextmenu", (event) => {
    event.preventDefault();
  });
}

window.addEventListener("blur", () => {
  keys.clear();
  resetJoystick();
  if (gameMode === "multiplayer" && multiplayerManager) {
    multiplayerManager.updateInputFromControls();
  }
});

window.addEventListener("resize", () => {
  updateBoardSize();
  updateJoystickThumbPosition();
});

document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "hidden") {
    keys.clear();
    resetJoystick();
    soundManager.stopMusic(0.4);
    if (gameMode === "multiplayer" && multiplayerManager) {
      multiplayerManager.updateInputFromControls();
    }
  } else if (gameMode === "single" && !gameOver && soundManager.enabled) {
    soundManager.startMusic();
  }
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
resetJoystick();
hideRestartButton();
showMainMenu();
updateBoardSize();
scoreEl.textContent = "0";
updateTimerDisplay(NORMAL_FISH_TIME_LIMIT);
