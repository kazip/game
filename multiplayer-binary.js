const encoder = new TextEncoder();
const decoder = new TextDecoder();

const PHASE_CODES = { lobby: 0, countdown: 1, playing: 2, ended: 3 };
const FISH_TYPE_CODES = { normal: 0, golden: 1, timeIncrease: 2, timeDecrease: 3 };
const POWER_UP_TYPE_CODES = {
  none: 0,
  fast: 1,
  slow: 2,
  invert: 3,
  memory: 4,
  chair: 5,
  table: 6,
  fish: 7,
  duck: 8,
  goose: 9,
  goldfish: 10,
  mine: 11,
  alarm: 12
};
const MESSAGE_TYPES = { full: 0, patch: 1 };

class BinaryWriter {
  constructor() {
    this.parts = [];
    this.length = 0;
  }

  writeUint8(value) {
    const buffer = new Uint8Array(1);
    buffer[0] = value & 0xff;
    this.push(buffer);
  }

  writeInt16(value) {
    const view = new DataView(new ArrayBuffer(2));
    view.setInt16(0, value);
    this.push(new Uint8Array(view.buffer));
  }

  writeUint16(value) {
    const view = new DataView(new ArrayBuffer(2));
    view.setUint16(0, value);
    this.push(new Uint8Array(view.buffer));
  }

  writeUint32(value) {
    const view = new DataView(new ArrayBuffer(4));
    view.setUint32(0, value >>> 0);
    this.push(new Uint8Array(view.buffer));
  }

  writeFloat32(value) {
    const view = new DataView(new ArrayBuffer(4));
    view.setFloat32(0, value);
    this.push(new Uint8Array(view.buffer));
  }

  writeBool(value) {
    this.writeUint8(value ? 1 : 0);
  }

  writeString(value) {
    const bytes = encoder.encode(value || "");
    this.writeUint16(bytes.length);
    this.push(bytes);
  }

  push(buffer) {
    this.parts.push(buffer);
    this.length += buffer.length;
  }

  toUint8Array() {
    const result = new Uint8Array(this.length);
    let offset = 0;
    for (const part of this.parts) {
      result.set(part, offset);
      offset += part.length;
    }
    return result;
  }
}

class BinaryReader {
  constructor(buffer) {
    this.view = new DataView(buffer.buffer, buffer.byteOffset, buffer.byteLength);
    this.offset = 0;
  }

  ensureAvailable(size) {
    const remaining = this.view.byteLength - this.offset;
    if (remaining < size) {
      console.warn(`Attempted to read ${size} bytes but only ${remaining} available; advancing to end of buffer.`);
      this.offset = this.view.byteLength;
      return false;
    }
    return true;
  }

  readUint8() {
    if (!this.ensureAvailable(1)) return 0;
    const value = this.view.getUint8(this.offset);
    this.offset += 1;
    return value;
  }

  readInt16() {
    if (!this.ensureAvailable(2)) return 0;
    const value = this.view.getInt16(this.offset);
    this.offset += 2;
    return value;
  }

  readUint16() {
    if (!this.ensureAvailable(2)) return 0;
    const value = this.view.getUint16(this.offset);
    this.offset += 2;
    return value;
  }

  readUint32() {
    if (!this.ensureAvailable(4)) return 0;
    const value = this.view.getUint32(this.offset);
    this.offset += 4;
    return value;
  }

  readFloat32() {
    if (!this.ensureAvailable(4)) return 0;
    const value = this.view.getFloat32(this.offset);
    this.offset += 4;
    return value;
  }

  readBool() {
    return this.readUint8() === 1;
  }

  readString() {
    const length = this.readUint16();
    const remaining = this.view.byteLength - this.offset;
    const safeLength = Math.max(0, Math.min(length, remaining));

    if (length > remaining) {
      console.warn(`Truncating oversized string read: requested ${length} bytes, only ${remaining} available.`);
    }

    const bytes = new Uint8Array(this.view.buffer, this.view.byteOffset + this.offset, safeLength);
    this.offset = Math.min(this.view.byteLength, this.offset + safeLength);
    return decoder.decode(bytes);
  }
}

function toBase64(uint8) {
  if (typeof btoa === "function") {
    let binary = "";
    const chunkSize = 0x8000;
    for (let i = 0; i < uint8.length; i += chunkSize) {
      binary += String.fromCharCode(...uint8.subarray(i, i + chunkSize));
    }
    return btoa(binary);
  }
  return Buffer.from(uint8).toString("base64");
}

function fromBase64(base64) {
  if (typeof atob === "function") {
    const binary = atob(base64 || "");
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) {
      bytes[i] = binary.charCodeAt(i);
    }
    return bytes;
  }
  return new Uint8Array(Buffer.from(base64 || "", "base64"));
}

function encodeWalls(walls = [], writer) {
  writer.writeUint16(Math.min(walls.length, 1024));
  walls.slice(0, 1024).forEach((wall) => {
    writer.writeFloat32(wall.x || 0);
    writer.writeFloat32(wall.y || 0);
    writer.writeFloat32(wall.width || 0);
    writer.writeFloat32(wall.height || 0);
  });
}

function decodeWalls(reader) {
  const count = reader.readUint16();
  const available = Math.floor((reader.view.byteLength - reader.offset) / 16);
  const safeCount = Math.max(0, Math.min(count, available));
  if (safeCount !== count) {
    console.warn(`Truncating wall decode: requested ${count}, only ${safeCount} fit in buffer.`);
  }
  const walls = [];
  for (let i = 0; i < safeCount; i += 1) {
    walls.push({ x: reader.readFloat32(), y: reader.readFloat32(), width: reader.readFloat32(), height: reader.readFloat32() });
  }
  return walls;
}

function encodeMines(mines = [], writer) {
  writer.writeUint8(Math.min(mines.length, 32));
  mines.slice(0, 32).forEach((mine) => {
    writer.writeFloat32(mine.x || 0);
    writer.writeFloat32(mine.y || 0);
    writer.writeFloat32(mine.size || 0);
  });
}

function decodeMines(reader) {
  const count = reader.readUint8();
  const available = Math.floor((reader.view.byteLength - reader.offset) / 12);
  const safeCount = Math.max(0, Math.min(count, available));
  if (safeCount !== count) {
    console.warn(`Truncating mine decode: requested ${count}, only ${safeCount} fit in buffer.`);
  }
  const mines = [];
  for (let i = 0; i < safeCount; i += 1) {
    mines.push({ x: reader.readFloat32(), y: reader.readFloat32(), size: reader.readFloat32() });
  }
  return mines;
}

function encodePowerUps(powerUps = [], writer) {
  writer.writeUint8(Math.min(powerUps.length, 255));
  powerUps.slice(0, 255).forEach((powerUp) => {
    writer.writeBool(Boolean(powerUp.active));
    writer.writeFloat32(powerUp.x || 0);
    writer.writeFloat32(powerUp.y || 0);
    writer.writeFloat32(powerUp.size || 0);
    writer.writeFloat32(powerUp.remaining || 0);
    writer.writeUint8(POWER_UP_TYPE_CODES[powerUp.type] ?? 0);
  });
}

function decodePowerUps(reader) {
  const count = reader.readUint8();
  const available = Math.floor((reader.view.byteLength - reader.offset) / 18);
  const safeCount = Math.max(0, Math.min(count, available));
  if (safeCount !== count) {
    console.warn(`Truncating power-up decode: requested ${count}, only ${safeCount} fit in buffer.`);
  }
  const powerUps = [];
  for (let i = 0; i < safeCount; i += 1) {
    powerUps.push({
      active: reader.readBool(),
      x: reader.readFloat32(),
      y: reader.readFloat32(),
      size: reader.readFloat32(),
      remaining: reader.readFloat32(),
      type:
        Object.keys(POWER_UP_TYPE_CODES).find((key) => POWER_UP_TYPE_CODES[key] === reader.readUint8()) || null
    });
  }
  return powerUps;
}

function encodePlayers(players = [], writer) {
  writer.writeUint8(Math.min(players.length, 32));
  players.slice(0, 32).forEach((player) => {
    writer.writeString(player.id || "");
    writer.writeString(player.name || "");
    writer.writeUint32(player.score >>> 0);
    writer.writeBool(Boolean(player.ready));
    writer.writeBool(Boolean(player.alive));
    writer.writeFloat32(player.x || 0);
    writer.writeFloat32(player.y || 0);
    writer.writeFloat32(player.size || 0);
    writer.writeInt16(Number(player.facing) || 0);
    writer.writeBool(Boolean(player.moving));
    writer.writeFloat32(player.walkCycle || 0);
    writer.writeFloat32(player.stepAccumulator || 0);
    const appearance = player.appearance ? JSON.stringify(player.appearance).slice(0, 300) : "";
    writer.writeString(appearance || "");
    writer.writeString(player.disguise || "");
  });
}

function decodePlayers(reader) {
  const count = reader.readUint8();
  const players = [];
  for (let i = 0; i < count; i += 1) {
    const player = {
      id: reader.readString(),
      name: reader.readString(),
      score: reader.readUint32(),
      ready: reader.readBool(),
      alive: reader.readBool(),
      x: reader.readFloat32(),
      y: reader.readFloat32(),
      size: reader.readFloat32(),
      facing: reader.readInt16(),
      moving: reader.readBool(),
      walkCycle: reader.readFloat32(),
      stepAccumulator: reader.readFloat32(),
      appearance: {}
    };
    const appearanceString = reader.readString();
    if (appearanceString) {
      try {
        player.appearance = JSON.parse(appearanceString);
      } catch (error) {
        player.appearance = {};
      }
    }
    player.disguise = reader.readString();
    players.push(player);
  }
  return players;
}

export function encodeStateToBase64(state) {
  const writer = new BinaryWriter();
  writer.writeUint8(MESSAGE_TYPES.full);
  writer.writeUint32(Math.floor(state.serverTime || 0));
  writer.writeUint32(state.tickIndex >>> 0 || 0);
  writer.writeString(state.roomName || "");
  writer.writeString(state.mode || "");
  writer.writeUint8(PHASE_CODES[state.phase] ?? 0);
  writer.writeFloat32(state.countdown || 0);
  writer.writeFloat32(state.remaining || 0);
  writer.writeString(state.hidePhase || "");
  writer.writeBool(Boolean(state.goldenChainActive));
  writer.writeString(state.winnerId || "");
  writer.writeString(state.message || "");
  writer.writeString(state.seekerId || "");
  writer.writeString(state.bombHolder || "");
  writer.writeFloat32(state.bombTimer || 0);

  const statusType = state.statusEffect?.type || "";
  writer.writeString(statusType);
  writer.writeFloat32(state.statusEffect?.remaining || 0);
  writer.writeString(state.statusEffect?.playerId || "");

  const fishTypeCode = FISH_TYPE_CODES[state.fish?.type] ?? 0;
  writer.writeUint8(fishTypeCode);
  writer.writeFloat32(state.fish?.x || 0);
  writer.writeFloat32(state.fish?.y || 0);
  writer.writeFloat32(state.fish?.size || 0);
  writer.writeBool(Boolean(state.fish?.alive));
  writer.writeBool(Boolean(state.fish?.spawned));
  writer.writeInt16(Number(state.fish?.direction) || 0);

  writer.writeBool(Boolean(state.powerUp?.active));
  writer.writeFloat32(state.powerUp?.x || 0);
  writer.writeFloat32(state.powerUp?.y || 0);
  writer.writeFloat32(state.powerUp?.size || 0);
  writer.writeFloat32(state.powerUp?.remaining || 0);
  writer.writeUint8(POWER_UP_TYPE_CODES[state.powerUp?.type] ?? 0);

  encodePowerUps(state.powerUps, writer);

  encodeWalls(state.walls, writer);
  encodeMines(state.mines, writer);

  encodePlayers(state.players || [], writer);
  return toBase64(writer.toUint8Array());
}

function toReader(payload) {
  if (!payload) {
    return null;
  }
  if (payload instanceof ArrayBuffer) {
    return new BinaryReader(new Uint8Array(payload));
  }
  if (payload instanceof Uint8Array) {
    return new BinaryReader(payload);
  }
  const base64 = typeof payload === "string" ? payload : payload?.binary || payload?.b;
  if (!base64) {
    return null;
  }
  return new BinaryReader(fromBase64(base64));
}

function decodePlayerPatch(reader) {
  const id = reader.readString();
  const flags1 = reader.readUint8();
  const flags2 = reader.readUint8();
  const patch = { id };
  if (flags1 & (1 << 0)) patch.name = reader.readString();
  if (flags1 & (1 << 1)) patch.ready = reader.readBool();
  if (flags1 & (1 << 2)) patch.alive = reader.readBool();
  if (flags1 & (1 << 3)) patch.x = reader.readFloat32();
  if (flags1 & (1 << 4)) patch.y = reader.readFloat32();
  if (flags1 & (1 << 5)) patch.size = reader.readFloat32();
  if (flags1 & (1 << 6)) patch.facing = reader.readInt16();
  if (flags1 & (1 << 7)) patch.moving = reader.readBool();
  if (flags2 & (1 << 0)) patch.walkCycle = reader.readFloat32();
  if (flags2 & (1 << 1)) patch.stepAccumulator = reader.readFloat32();
  if (flags2 & (1 << 2)) patch.score = reader.readUint32();
  if (flags2 & (1 << 3)) {
    const appearanceString = reader.readString();
    try {
      patch.appearance = appearanceString ? JSON.parse(appearanceString) : {};
    } catch (error) {
      patch.appearance = {};
    }
  }
  if (flags2 & (1 << 4)) patch.disguise = reader.readString();
  return patch;
}

function decodeFullState(reader) {
  const serverTime = reader.readUint32();
  const tickIndex = reader.readUint32();
  const roomName = reader.readString();
  const mode = reader.readString();
  const phaseCode = reader.readUint8();
  const countdown = reader.readFloat32();
  const remaining = reader.readFloat32();
  const hidePhase = reader.readString();
  const goldenChainActive = reader.readBool();
  const winnerId = reader.readString();
  const message = reader.readString();
  const seekerId = reader.readString();
  const bombHolder = reader.readString();
  const bombTimer = reader.readFloat32();
  const statusType = reader.readString();
  const statusRemaining = reader.readFloat32();
  const statusPlayerId = reader.readString();
  const fishType = reader.readUint8();
  const fish = {
    type: Object.keys(FISH_TYPE_CODES).find((key) => FISH_TYPE_CODES[key] === fishType) || "normal",
    x: reader.readFloat32(),
    y: reader.readFloat32(),
    size: reader.readFloat32(),
    alive: reader.readBool(),
    spawned: reader.readBool(),
    direction: reader.readInt16()
  };

  const powerUp = {
    active: reader.readBool(),
    x: reader.readFloat32(),
    y: reader.readFloat32(),
    size: reader.readFloat32(),
    remaining: reader.readFloat32(),
    type: null
  };
  const powerUpType = reader.readUint8();
  powerUp.type =
    Object.keys(POWER_UP_TYPE_CODES).find((key) => POWER_UP_TYPE_CODES[key] === powerUpType) || null;

  const powerUps = decodePowerUps(reader);

  const walls = decodeWalls(reader);
  const mines = decodeMines(reader);
  const players = decodePlayers(reader);

  return {
    state: {
      phase: Object.keys(PHASE_CODES).find((key) => PHASE_CODES[key] === phaseCode) || "lobby",
      mode,
      countdown,
      remaining,
      hidePhase,
      message,
      seekerId,
      bombHolder,
      bombTimer,
      winnerId: winnerId || null,
      goldenChainActive,
      statusEffect: statusType
        ? { type: statusType, remaining: statusRemaining, playerId: statusPlayerId || undefined }
        : null,
      fish,
      powerUp,
      powerUps,
      walls,
      mines,
      players,
      serverTime,
      tickIndex,
      roomName
    }
  };
}

function decodePatch(reader) {
  const serverTime = reader.readUint32();
  const tickIndex = reader.readUint32();
  const flags1 = reader.readUint8();
  const flags2 = reader.readUint8();
  const flags3 = reader.readUint8();
  const patch = { serverTime, tickIndex };

  if (flags1 & (1 << 0)) patch.phase = reader.readString();
  if (flags1 & (1 << 1)) patch.countdown = reader.readFloat32();
  if (flags1 & (1 << 2)) patch.remaining = reader.readFloat32();
  if (flags1 & (1 << 3)) patch.message = reader.readString();
  if (flags1 & (1 << 4)) patch.winnerId = reader.readString();
  if (flags1 & (1 << 5)) patch.goldenChainActive = reader.readBool();
  if (flags1 & (1 << 6)) {
    const statusType = reader.readString();
    const statusRemaining = reader.readFloat32();
    const statusPlayerId = reader.readString();
    patch.statusEffect = statusType
      ? { type: statusType, remaining: statusRemaining, playerId: statusPlayerId || undefined }
      : null;
  }
  if (flags1 & (1 << 7)) {
    const fishType = reader.readUint8();
    patch.fish = {
      type: Object.keys(FISH_TYPE_CODES).find((key) => FISH_TYPE_CODES[key] === fishType) || "normal",
      x: reader.readFloat32(),
      y: reader.readFloat32(),
      size: reader.readFloat32(),
      alive: reader.readBool(),
      spawned: reader.readBool(),
      direction: reader.readInt16()
    };
  }

  if (flags2 & (1 << 0)) {
    patch.powerUp = {
      active: reader.readBool(),
      x: reader.readFloat32(),
      y: reader.readFloat32(),
      size: reader.readFloat32(),
      remaining: reader.readFloat32(),
      type: Object.keys(POWER_UP_TYPE_CODES).find((key) => POWER_UP_TYPE_CODES[key] === reader.readUint8()) || null
    };
  }
  if (flags3 & (1 << 0)) {
    patch.powerUps = decodePowerUps(reader);
  }
  if (flags3 & (1 << 1)) {
    patch.seekerId = reader.readString();
  }
  if (flags3 & (1 << 2)) {
    patch.hidePhase = reader.readString();
  }
  if (flags2 & (1 << 1)) {
    patch.walls = decodeWalls(reader);
  }
  if (flags2 & (1 << 2)) {
    patch.mines = decodeMines(reader);
  }
  if (flags2 & (1 << 3)) {
    const count = reader.readUint8();
    patch.players = [];
    for (let i = 0; i < count; i += 1) {
      patch.players.push(decodePlayerPatch(reader));
    }
  }
  if (flags2 & (1 << 4)) {
    const count = reader.readUint8();
    patch.removedPlayers = [];
    for (let i = 0; i < count; i += 1) {
      patch.removedPlayers.push(reader.readString());
    }
  }
  if (flags2 & (1 << 5)) {
    patch.mode = reader.readString();
  }
  if (flags2 & (1 << 6)) {
    patch.bombHolder = reader.readString();
  }
  if (flags2 & (1 << 7)) {
    patch.bombTimer = reader.readFloat32();
  }
  return { patch };
}

export function decodeStateFromBase64(payload) {
  const reader = toReader(payload);
  if (!reader) {
    return null;
  }
  const messageType = reader.readUint8();
  if (messageType === MESSAGE_TYPES.patch) {
    return decodePatch(reader);
  }
  return decodeFullState(reader);
}

export function encodeInputToBase64(playerId, vector) {
  const writer = new BinaryWriter();
  writer.writeString(playerId || "");
  writer.writeFloat32(vector?.x || 0);
  writer.writeFloat32(vector?.y || 0);
  return toBase64(writer.toUint8Array());
}

export function encodeInputToBuffer(playerId, vector) {
  const writer = new BinaryWriter();
  writer.writeString(playerId || "");
  writer.writeFloat32(vector?.x || 0);
  writer.writeFloat32(vector?.y || 0);
  return writer.toUint8Array();
}

export function decodeInputFromBase64(payload) {
  const reader = toReader(payload);
  if (!reader) {
    return null;
  }
  return { playerId: reader.readString(), vector: { x: reader.readFloat32(), y: reader.readFloat32() } };
}
