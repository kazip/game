import { createClient } from "https://esm.sh/@supabase/supabase-js@2";
import { SUPABASE_URL, SUPABASE_ANON_KEY } from "./supabase-config.js";

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
  alive: false,
  type: "normal"
};

const keys = new Set();
const NORMAL_FISH_TIME_LIMIT = 10;
const NORMAL_FISH_POINTS = 1;
const GOLDEN_FISH_CHANCE = 0.05;
const GOLDEN_FISH_TIME_LIMIT = 5;
const GOLDEN_FISH_POINTS = 5;

let score = 0;
let remaining = NORMAL_FISH_TIME_LIMIT;
let lastTimestamp = 0;
let gameOver = false;
let goldenChainActive = false;
const messageEl = document.getElementById("message");
const restartBtn = document.getElementById("restart");
const scoreEl = document.getElementById("score");
const timerEl = document.getElementById("timer");
const controlsContainer = document.querySelector(".controls");
const joystickBase = document.querySelector(".joystick-base");
const joystickThumb = document.querySelector(".joystick-thumb");
const leaderboardEl = document.getElementById("leaderboard");
const submitScoreForm = document.getElementById("submit-score");
const playerNameInput = document.getElementById("player-name");
const saveScoreButton = document.getElementById("save-score");
const scoreStatusEl = document.getElementById("score-status");

const directionKeys = new Set([
  "ArrowUp",
  "ArrowDown",
  "ArrowLeft",
  "ArrowRight"
]);
const JOYSTICK_DEADZONE = 0.22;
let joystickPointerId = null;
const joystickVector = { x: 0, y: 0 };

const sanitizedSupabaseUrl =
  typeof SUPABASE_URL === "string" ? SUPABASE_URL.trim() : "";
const sanitizedSupabaseKey =
  typeof SUPABASE_ANON_KEY === "string" ? SUPABASE_ANON_KEY.trim() : "";
const supabaseConfigured =
  sanitizedSupabaseUrl &&
  sanitizedSupabaseKey &&
  sanitizedSupabaseUrl !== "https://your-project-ref.supabase.co" &&
  sanitizedSupabaseKey !== "public-anon-key";
const supabaseClient = supabaseConfigured
  ? createClient(sanitizedSupabaseUrl, sanitizedSupabaseKey)
  : null;

const PLAYER_NAME_STORAGE_KEY = "cat-game:player-name";
const DEFAULT_SCORE_STATUS =
  "Сыграйте раунд и сохраните результат в таблицу лидеров.";

const scoreboardState = {
  hasSavedCurrentScore: false,
  isSaving: false
};

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

function setScoreStatus(text = "") {
  if (scoreStatusEl) {
    scoreStatusEl.textContent = text;
  }
}

function updateScoreFormControls() {
  if (!submitScoreForm) {
    return;
  }
  const supabaseReady = Boolean(supabaseClient);
  if (playerNameInput) {
    playerNameInput.disabled = !supabaseReady || scoreboardState.isSaving;
  }
  if (saveScoreButton) {
    const shouldEnable =
      supabaseReady &&
      gameOver &&
      !scoreboardState.hasSavedCurrentScore &&
      !scoreboardState.isSaving;
    saveScoreButton.disabled = !shouldEnable;
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

async function fetchLeaderboard() {
  if (!supabaseClient) {
    if (leaderboardEl && leaderboardEl.childElementCount === 0) {
      const item = document.createElement("li");
      item.textContent = "Supabase не настроен.";
      leaderboardEl.appendChild(item);
    }
    return;
  }

  if (leaderboardEl) {
    leaderboardEl.innerHTML = "";
    const loadingItem = document.createElement("li");
    loadingItem.textContent = "Загрузка...";
    leaderboardEl.appendChild(loadingItem);
  }

  try {
    const { data, error } = await supabaseClient
      .from("scores")
      .select("name, score")
      .order("score", { ascending: false })
      .order("created_at", { ascending: true })
      .limit(10);
    if (error) {
      throw error;
    }
    renderLeaderboard(data || []);
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

if (!supabaseClient) {
  if (leaderboardEl) {
    leaderboardEl.innerHTML = "";
    const item = document.createElement("li");
    item.textContent = "Supabase не настроен.";
    leaderboardEl.appendChild(item);
  }
  setScoreStatus("Укажите Supabase URL и ключ в файле supabase-config.js.");
} else {
  setScoreStatus(DEFAULT_SCORE_STATUS);
  fetchLeaderboard();
}

updateScoreFormControls();

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
    this.sfxGain.gain.value = 1;
    this.sfxGain.connect(this.masterGain);

    this.enabled = true;
    this.startMusic();
  }

  startMusic() {
    if (!this.enabled) {
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

function spawnFish() {
  const margin = 30;
  const shouldSpawnGolden = goldenChainActive || Math.random() < GOLDEN_FISH_CHANCE;
  fish.type = shouldSpawnGolden ? "golden" : "normal";
  goldenChainActive = shouldSpawnGolden;
  fish.x = margin + Math.random() * (WORLD_SIZE - margin * 2);
  fish.y = margin + Math.random() * (WORLD_SIZE - margin * 2);
  fish.alive = true;
  remaining =
    fish.type === "golden" ? GOLDEN_FISH_TIME_LIMIT : NORMAL_FISH_TIME_LIMIT;
}

function resetGame() {
  updateBoardSize();
  score = 0;
  scoreEl.textContent = score;
  gameOver = false;
  scoreboardState.hasSavedCurrentScore = false;
  scoreboardState.isSaving = false;
  messageEl.textContent = "";
  restartBtn.disabled = true;
  cat.x = WORLD_SIZE / 2;
  cat.y = WORLD_SIZE / 2;
  cat.moving = false;
  cat.walkCycle = 0;
  cat.stepAccumulator = 0;
  cat.facing = 1;
  goldenChainActive = false;
  spawnFish();
  if (soundManager.enabled) {
    soundManager.startMusic();
  }
  lastTimestamp = performance.now();
  if (supabaseClient) {
    setScoreStatus(DEFAULT_SCORE_STATUS);
  }
  updateScoreFormControls();
  requestAnimationFrame(loop);
}

function endGame(reason) {
  gameOver = true;
  fish.alive = false;
  restartBtn.disabled = false;
  messageEl.textContent = reason + ` Результат: ${score}.`;
  updateBoardSize();
  soundManager.stopMusic(0.8);
  updateScoreFormControls();
  if (supabaseClient && !scoreboardState.hasSavedCurrentScore) {
    setScoreStatus("Введите имя и сохраните результат.");
  }
}

function update(delta) {
  let horizontalInput =
    (keys.has("ArrowRight") || keys.has("d") ? 1 : 0) -
    (keys.has("ArrowLeft") || keys.has("a") ? 1 : 0);
  let verticalInput =
    (keys.has("ArrowDown") || keys.has("s") ? 1 : 0) -
    (keys.has("ArrowUp") || keys.has("w") ? 1 : 0);

  horizontalInput += joystickVector.x;
  verticalInput += joystickVector.y;

  const length = Math.hypot(horizontalInput, verticalInput);
  const hasInput = length > 0.001;
  if (hasInput) {
    const cappedMagnitude = Math.min(length, 1);
    const normalizedX = horizontalInput / (length || 1);
    const normalizedY = verticalInput / (length || 1);
    const distance = cat.speed * delta * cappedMagnitude;
    cat.x += normalizedX * distance;
    cat.y += normalizedY * distance;
    cat.moving = true;
    if (Math.abs(normalizedX) > 0.1) {
      cat.facing = normalizedX >= 0 ? 1 : -1;
    }
    const walkIncrement = delta * WALK_FREQUENCY * cappedMagnitude;
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
      const isGoldenFish = fish.type === "golden";
      score += isGoldenFish ? GOLDEN_FISH_POINTS : NORMAL_FISH_POINTS;
      scoreEl.textContent = score;
      goldenChainActive = isGoldenFish;
      spawnFish();
      soundManager.playCatch();
    }
  }

  remaining -= delta;
  const displayTime = Math.max(remaining, 0);
  timerEl.textContent = displayTime.toFixed(1);
  if (remaining <= 0) {
    const missedFishType = fish.type;
    if (missedFishType === "golden") {
      fish.alive = false;
      goldenChainActive = false;
      spawnFish();
    } else {
      endGame("Котик не успел поймать рыбку!");
    }
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
  const bodyColor = fish.type === "golden" ? "#ffd700" : "#5cc8ff";
  const finColor = fish.type === "golden" ? "#ffae00" : "#5cc8ff";
  ctx.fillStyle = bodyColor;
  ctx.beginPath();
  ctx.ellipse(0, 0, fish.size / 2, fish.size / 3, 0, 0, Math.PI * 2);
  ctx.fill();

  ctx.beginPath();
  ctx.fillStyle = finColor;
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

if (submitScoreForm) {
  submitScoreForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!supabaseClient) {
      return;
    }

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
      const payload = { name: normalizedName, score: safeScore };
      const { error } = await supabaseClient.from("scores").insert(payload);
      if (error) {
        throw error;
      }
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
  } else if (!gameOver && soundManager.enabled) {
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
resetGame();
