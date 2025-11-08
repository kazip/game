const canvas = document.getElementById("game");
const ctx = canvas.getContext("2d");

const cat = {
  x: canvas.width / 2,
  y: canvas.height / 2,
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

function clamp(value, min, max) {
  return Math.min(Math.max(value, min), max);
}

function spawnFish() {
  const margin = 30;
  fish.x = margin + Math.random() * (canvas.width - margin * 2);
  fish.y = margin + Math.random() * (canvas.height - margin * 2);
  fish.alive = true;
  remaining = 10;
}

function resetGame() {
  score = 0;
  scoreEl.textContent = score;
  gameOver = false;
  messageEl.textContent = "";
  restartBtn.disabled = true;
  cat.x = canvas.width / 2;
  cat.y = canvas.height / 2;
  spawnFish();
  lastTimestamp = performance.now();
  requestAnimationFrame(loop);
}

function endGame(reason) {
  gameOver = true;
  fish.alive = false;
  restartBtn.disabled = false;
  messageEl.textContent = reason + ` Результат: ${score}.`;
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

  cat.x = clamp(cat.x, cat.size / 2, canvas.width - cat.size / 2);
  cat.y = clamp(cat.y, cat.size / 2, canvas.height - cat.size / 2);

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

function loop(timestamp) {
  if (gameOver) return;

  const delta = (timestamp - lastTimestamp) / 1000;
  lastTimestamp = timestamp;
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  update(delta);
  drawFish();
  drawCat();
  requestAnimationFrame(loop);
}

window.addEventListener("keydown", (event) => {
  keys.add(event.key);
});

window.addEventListener("keyup", (event) => {
  keys.delete(event.key);
});

restartBtn.addEventListener("click", () => {
  if (!gameOver) return;
  resetGame();
});

// Запуск игры при загрузке страницы
resetGame();
