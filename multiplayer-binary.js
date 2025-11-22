const encoder = new TextEncoder();
const decoder = new TextDecoder();

const PHASE_CODES = { lobby: 0, countdown: 1, playing: 2, ended: 3 };
const FISH_TYPE_CODES = { normal: 0, golden: 1, timeIncrease: 2, timeDecrease: 3 };
const POWER_UP_TYPE_CODES = { none: 0, fast: 1, slow: 2, invert: 3 };

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

  readUint8() {
    const value = this.view.getUint8(this.offset);
    this.offset += 1;
    return value;
  }

  readInt16() {
    const value = this.view.getInt16(this.offset);
    this.offset += 2;
    return value;
  }

  readUint16() {
    const value = this.view.getUint16(this.offset);
    this.offset += 2;
    return value;
  }

  readUint32() {
    const value = this.view.getUint32(this.offset);
    this.offset += 4;
    return value;
  }

  readFloat32() {
    const value = this.view.getFloat32(this.offset);
    this.offset += 4;
    return value;
  }

  readBool() {
    return this.readUint8() === 1;
  }

  readString() {
    const length = this.readUint16();
    const bytes = new Uint8Array(this.view.buffer, this.view.byteOffset + this.offset, length);
    this.offset += length;
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
    writer.writeFloat32(wall.x1 || 0);
    writer.writeFloat32(wall.y1 || 0);
    writer.writeFloat32(wall.x2 || 0);
    writer.writeFloat32(wall.y2 || 0);
  });
}

function decodeWalls(reader) {
  const count = reader.readUint16();
  const walls = [];
  for (let i = 0; i < count; i += 1) {
    walls.push({ x1: reader.readFloat32(), y1: reader.readFloat32(), x2: reader.readFloat32(), y2: reader.readFloat32() });
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
  const mines = [];
  for (let i = 0; i < count; i += 1) {
    mines.push({ x: reader.readFloat32(), y: reader.readFloat32(), size: reader.readFloat32() });
  }
  return mines;
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
    const appearance =
      player.appearanceJson || (player.appearance ? JSON.stringify(player.appearance).slice(0, 300) : "");
    writer.writeString(appearance || "");
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
    players.push(player);
  }
  return players;
}

export function encodeStateToBase64(state) {
  const writer = new BinaryWriter();
  writer.writeUint32(Math.floor(state.serverTime || 0));
  writer.writeUint32(state.tickIndex >>> 0 || 0);
  writer.writeString(state.roomName || "");
  writer.writeUint8(PHASE_CODES[state.phase] ?? 0);
  writer.writeFloat32(state.countdown || 0);
  writer.writeFloat32(state.remaining || 0);
  writer.writeBool(Boolean(state.goldenChainActive));
  writer.writeString(state.winnerId || "");
  writer.writeString(state.message || "");

  const statusType = state.statusEffect?.type || "";
  writer.writeString(statusType);
  writer.writeFloat32(state.statusEffect?.remaining || 0);

  const fishTypeCode = FISH_TYPE_CODES[state.fish?.type] ?? 0;
  writer.writeUint8(fishTypeCode);
  writer.writeFloat32(state.fish?.x || 0);
  writer.writeFloat32(state.fish?.y || 0);
  writer.writeFloat32(state.fish?.size || 0);
  writer.writeBool(Boolean(state.fish?.alive));
  writer.writeInt16(Number(state.fish?.direction) || 0);

  writer.writeBool(Boolean(state.powerUp?.active));
  writer.writeFloat32(state.powerUp?.x || 0);
  writer.writeFloat32(state.powerUp?.y || 0);
  writer.writeFloat32(state.powerUp?.size || 0);
  writer.writeFloat32(state.powerUp?.remaining || 0);
  writer.writeUint8(POWER_UP_TYPE_CODES[state.powerUp?.type] ?? 0);

  encodeWalls(state.walls, writer);
  encodeMines(state.mines, writer);

  encodePlayers(state.players || [], writer);
  return toBase64(writer.toUint8Array());
}

export function decodeStateFromBase64(payload) {
  const base64 = typeof payload === "string" ? payload : payload?.binary || payload?.b;
  if (!base64) {
    return null;
  }
  const buffer = fromBase64(base64);
  const reader = new BinaryReader(buffer);
  const serverTime = reader.readUint32();
  const tickIndex = reader.readUint32();
  const roomName = reader.readString();
  const phaseCode = reader.readUint8();
  const countdown = reader.readFloat32();
  const remaining = reader.readFloat32();
  const goldenChainActive = reader.readBool();
  const winnerId = reader.readString();
  const message = reader.readString();
  const statusType = reader.readString();
  const statusRemaining = reader.readFloat32();
  const fishType = reader.readUint8();
  const fish = {
    type: Object.keys(FISH_TYPE_CODES).find((key) => FISH_TYPE_CODES[key] === fishType) || "normal",
    x: reader.readFloat32(),
    y: reader.readFloat32(),
    size: reader.readFloat32(),
    alive: reader.readBool(),
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

  const walls = decodeWalls(reader);
  const mines = decodeMines(reader);
  const players = decodePlayers(reader);

  return {
    phase: Object.keys(PHASE_CODES).find((key) => PHASE_CODES[key] === phaseCode) || "lobby",
    countdown,
    remaining,
    message,
    winnerId: winnerId || null,
    goldenChainActive,
    statusEffect: statusType ? { type: statusType, remaining: statusRemaining } : null,
    fish,
    powerUp,
    walls,
    mines,
    players,
    serverTime,
    tickIndex,
    roomName
  };
}

export function encodeInputToBase64(playerId, vector) {
  const writer = new BinaryWriter();
  writer.writeString(playerId || "");
  writer.writeFloat32(vector?.x || 0);
  writer.writeFloat32(vector?.y || 0);
  return toBase64(writer.toUint8Array());
}

export function decodeInputFromBase64(payload) {
  const base64 = typeof payload === "string" ? payload : payload?.binary || payload?.b;
  if (!base64) {
    return null;
  }
  const buffer = fromBase64(base64);
  const reader = new BinaryReader(buffer);
  return { playerId: reader.readString(), vector: { x: reader.readFloat32(), y: reader.readFloat32() } };
}
