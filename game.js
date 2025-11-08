const canvas = document.getElementById("game");
const ctx = canvas.getContext("2d");

const WORLD_SIZE = 500;
const MIN_BOARD_SIZE = 260;
const gameContainer = document.querySelector(".game-container");

const cat = {
  x: WORLD_SIZE / 2,
  y: WORLD_SIZE / 2,
  size: 36,
  speed: 180 // pixels per second
};

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
  const distance = cat.speed * delta;
  if (keys.has("ArrowUp") || keys.has("w")) {
    cat.y -= distance;
  }
  if (keys.has("ArrowDown") || keys.has("s")) {
    cat.y += distance;
  }
  if (keys.has("ArrowLeft") || keys.has("a")) {
    cat.x -= distance;
  }
  if (keys.has("ArrowRight") || keys.has("d")) {
    cat.x += distance;
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
  ctx.fillStyle = "#ffb347";
  ctx.beginPath();
  ctx.ellipse(0, 0, cat.size / 2, cat.size / 2.4, 0, 0, Math.PI * 2);
  ctx.fill();

  ctx.fillStyle = "#14365d";
  ctx.beginPath();
  ctx.ellipse(-8, -6, 4, 6, 0, 0, Math.PI * 2);
  ctx.ellipse(8, -6, 4, 6, 0, 0, Math.PI * 2);
  ctx.fill();

  ctx.fillStyle = "#ffe5b4";
  ctx.beginPath();
  ctx.arc(0, 6, 8, 0, Math.PI * 2);
  ctx.fill();

  ctx.fillStyle = "#ff6f61";
  ctx.beginPath();
  ctx.arc(0, 10, 5, 0, Math.PI);
  ctx.fill();

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
  keys.add(key);
});

window.addEventListener("keyup", (event) => {
  const key = normalizeKey(event.key);
  keys.delete(key);
});

restartBtn.addEventListener("click", () => {
  if (!gameOver) return;
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
