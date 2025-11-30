package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	worldSize         = 500.0
	catSpeed          = 180.0
	catSize           = 36.0
	fishSize          = 28.0
	fishSwimSpeed     = 36.0
	gridSize          = 10
	gridCellSize      = worldSize / gridSize
	wallThickness     = gridCellSize * 0.6
	maxWallTotalLen   = 10
	tickRate          = time.Second / 60
	broadcastRate     = time.Second / 15
	countdownDuration = 3 * time.Second
	roundDuration     = 60 * time.Second
	fishCatchDistance = 34.0
	maxMines          = 3
	mineSize          = 26.0
	mineMinDistance   = 25.0
	powerUpSize       = 34.0
	powerUpChance     = 0.05
	powerUpLifetime   = 5.0
	powerUpDuration   = 30.0
	timeIncreaseLimit = 15.0
	timeDecreaseLimit = 5.0
	dataFileName      = "data.json"
	reconnectGrace    = 10 * time.Second
)

var (
	phaseCodes       = map[string]uint8{"lobby": 0, "countdown": 1, "playing": 2, "ended": 3}
	fishTypeCodes    = map[string]uint8{"normal": 0, "golden": 1, "timeIncrease": 2, "timeDecrease": 3}
	powerUpTypeCodes = map[string]uint8{"none": 0, "fast": 1, "slow": 2, "invert": 3}

	messageTypeFull  uint8 = 0
	messageTypePatch uint8 = 1
)

type vector struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type catAppearance map[string]any

type catProfile struct {
	PlayerID   string        `json:"playerId"`
	Name       string        `json:"name"`
	Appearance catAppearance `json:"appearance"`
}

type scoreEntry struct {
	PlayerID  string    `json:"playerId"`
	Name      string    `json:"name"`
	Score     int       `json:"score"`
	CreatedAt time.Time `json:"created_at"`
}

type playerState struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Ready      bool          `json:"ready"`
	Alive      bool          `json:"alive"`
	X          float64       `json:"x"`
	Y          float64       `json:"y"`
	Size       float64       `json:"size"`
	Facing     int           `json:"facing"`
	Moving     bool          `json:"moving"`
	WalkCycle  float64       `json:"walkCycle"`
	StepAccum  float64       `json:"stepAccumulator"`
	Score      int           `json:"score"`
	Appearance catAppearance `json:"appearance"`
}

type fishState struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Size      float64 `json:"size"`
	Alive     bool    `json:"alive"`
	Spawned   bool    `json:"spawned,omitempty"`
	Type      string  `json:"type"`
	Direction int     `json:"direction"`
}

type gameState struct {
	RoomName   string         `json:"roomName"`
	Phase      string         `json:"phase"`
	Countdown  float64        `json:"countdown"`
	Remaining  float64        `json:"remaining"`
	Message    string         `json:"message"`
	Players    []*playerState `json:"players"`
	Fish       fishState      `json:"fish"`
	Walls      []wall         `json:"walls"`
	Mines      []mine         `json:"mines"`
	PowerUp    powerUpState   `json:"powerUp"`
	Status     *statusEffect  `json:"statusEffect"`
	WinnerID   string         `json:"winnerId"`
	Golden     bool           `json:"goldenChainActive"`
	TickIndex  uint32         `json:"tickIndex"`
	ServerTime int64          `json:"serverTime"`
}

type wall struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type mine struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Size float64 `json:"size"`
}

type powerUpState struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Size      float64 `json:"size"`
	Active    bool    `json:"active"`
	Remaining float64 `json:"remaining"`
	Type      string  `json:"type"`
}

type statusEffect struct {
	Type      string  `json:"type"`
	Remaining float64 `json:"remaining"`
}

type gridCell struct {
	Row int
	Col int
}

type wallSegment struct {
	Row         int
	Col         int
	Length      int
	Orientation string
}

type wsMessage struct {
	Type       string        `json:"type"`
	Ready      *bool         `json:"ready,omitempty"`
	Vector     *vector       `json:"vector,omitempty"`
	Message    *chatMessage  `json:"message,omitempty"`
	Appearance catAppearance `json:"appearance,omitempty"`
	State      *gameState    `json:"state,omitempty"`
	Patch      *statePatch   `json:"patch,omitempty"`
	Full       bool          `json:"full,omitempty"`
	Error      string        `json:"error,omitempty"`
}

type playerPatch struct {
	ID         string        `json:"id"`
	Name       *string       `json:"name,omitempty"`
	Ready      *bool         `json:"ready,omitempty"`
	Alive      *bool         `json:"alive,omitempty"`
	X          *float64      `json:"x,omitempty"`
	Y          *float64      `json:"y,omitempty"`
	Size       *float64      `json:"size,omitempty"`
	Facing     *int          `json:"facing,omitempty"`
	Moving     *bool         `json:"moving,omitempty"`
	WalkCycle  *float64      `json:"walkCycle,omitempty"`
	StepAccum  *float64      `json:"stepAccumulator,omitempty"`
	Score      *int          `json:"score,omitempty"`
	Appearance catAppearance `json:"appearance,omitempty"`
}

type statePatch struct {
	Phase          *string       `json:"phase,omitempty"`
	Countdown      *float64      `json:"countdown,omitempty"`
	Remaining      *float64      `json:"remaining,omitempty"`
	Message        *string       `json:"message,omitempty"`
	WinnerID       *string       `json:"winnerId,omitempty"`
	Golden         *bool         `json:"goldenChainActive,omitempty"`
	Status         *statusEffect `json:"statusEffect,omitempty"`
	Fish           *fishState    `json:"fish,omitempty"`
	PowerUp        *powerUpState `json:"powerUp,omitempty"`
	Walls          []wall        `json:"walls,omitempty"`
	Mines          []mine        `json:"mines,omitempty"`
	Players        []playerPatch `json:"players,omitempty"`
	RemovedPlayers []string      `json:"removedPlayers,omitempty"`
}

type chatMessage struct {
	PlayerID string `json:"playerId"`
	Name     string `json:"name"`
	Text     string `json:"text"`
	At       int64  `json:"at"`
}

func floatChanged(a, b float64) bool {
	return math.Abs(a-b) > 0.0001
}

func stringPtr(v string) *string { return &v }

func boolPtr(v bool) *bool { return &v }

func floatPtr(v float64) *float64 { return &v }

func intPtr(v int) *int { return &v }

func roundFloat(v float64, decimals int) float64 {
	factor := math.Pow10(decimals)
	return math.Round(v*factor) / factor
}

func quantizeCoord(v float64) float64 {
	return math.Round(v)
}

func quantizeWalkCycle(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return roundFloat(v*65535, 0) / 65535
}

func quantizeSeconds(v float64) float64 {
	return roundFloat(v, 3)
}

func quantizePlayerForSend(p *playerState) {
	p.X = quantizeCoord(p.X)
	p.Y = quantizeCoord(p.Y)
	p.Size = quantizeCoord(p.Size)
	p.WalkCycle = quantizeWalkCycle(p.WalkCycle)
	p.StepAccum = roundFloat(p.StepAccum, 3)
}

type binaryWriter struct {
	buf bytes.Buffer
}

func (w *binaryWriter) writeUint8(v uint8) {
	_ = w.buf.WriteByte(v)
}

func (w *binaryWriter) writeBool(v bool) {
	if v {
		w.writeUint8(1)
		return
	}
	w.writeUint8(0)
}

func (w *binaryWriter) writeInt16(v int16) {
	_ = binary.Write(&w.buf, binary.BigEndian, v)
}

func (w *binaryWriter) writeUint16(v uint16) {
	_ = binary.Write(&w.buf, binary.BigEndian, v)
}

func (w *binaryWriter) writeUint32(v uint32) {
	_ = binary.Write(&w.buf, binary.BigEndian, v)
}

func (w *binaryWriter) writeFloat32(v float32) {
	_ = binary.Write(&w.buf, binary.BigEndian, v)
}

func (w *binaryWriter) writeString(v string) {
	bytesValue := []byte(v)
	if len(bytesValue) > math.MaxUint16 {
		bytesValue = bytesValue[:math.MaxUint16]
	}
	w.writeUint16(uint16(len(bytesValue)))
	_, _ = w.buf.Write(bytesValue)
}

func (w *binaryWriter) bytes() []byte {
	return w.buf.Bytes()
}

type binaryReader struct {
	data   []byte
	offset int
}

func (r *binaryReader) readUint8() (uint8, error) {
	if r.offset+1 > len(r.data) {
		return 0, fmt.Errorf("out of bounds")
	}
	value := r.data[r.offset]
	r.offset++
	return value, nil
}

func (r *binaryReader) readInt16() (int16, error) {
	if r.offset+2 > len(r.data) {
		return 0, fmt.Errorf("out of bounds")
	}
	value := int16(binary.BigEndian.Uint16(r.data[r.offset:]))
	r.offset += 2
	return value, nil
}

func (r *binaryReader) readUint16() (uint16, error) {
	if r.offset+2 > len(r.data) {
		return 0, fmt.Errorf("out of bounds")
	}
	value := binary.BigEndian.Uint16(r.data[r.offset:])
	r.offset += 2
	return value, nil
}

func (r *binaryReader) readFloat32() (float32, error) {
	if r.offset+4 > len(r.data) {
		return 0, fmt.Errorf("out of bounds")
	}
	bits := binary.BigEndian.Uint32(r.data[r.offset:])
	r.offset += 4
	return math.Float32frombits(bits), nil
}

func (r *binaryReader) readString() (string, error) {
	length, err := r.readUint16()
	if err != nil {
		return "", err
	}
	if r.offset+int(length) > len(r.data) {
		return "", fmt.Errorf("out of bounds")
	}
	value := string(r.data[r.offset : r.offset+int(length)])
	r.offset += int(length)
	return value, nil
}

func (r *binaryReader) readFloatVector() (*vector, error) {
	x, err := r.readFloat32()
	if err != nil {
		return nil, err
	}
	y, err := r.readFloat32()
	if err != nil {
		return nil, err
	}
	return &vector{X: float64(x), Y: float64(y)}, nil
}

func decodeInputBuffer(data []byte) (*string, *vector) {
	reader := &binaryReader{data: data}
	playerID, err := reader.readString()
	if err != nil {
		return nil, nil
	}
	vec, err := reader.readFloatVector()
	if err != nil {
		return nil, nil
	}
	return &playerID, vec
}

func encodeWallsBinary(walls []wall, writer *binaryWriter) {
	count := len(walls)
	if count > 1024 {
		count = 1024
	}
	writer.writeUint16(uint16(count))
	for i := 0; i < count; i++ {
		writer.writeFloat32(float32(walls[i].X))
		writer.writeFloat32(float32(walls[i].Y))
		writer.writeFloat32(float32(walls[i].Width))
		writer.writeFloat32(float32(walls[i].Height))
	}
}

func encodeMinesBinary(mines []mine, writer *binaryWriter) {
	count := len(mines)
	if count > 32 {
		count = 32
	}
	writer.writeUint8(uint8(count))
	for i := 0; i < count; i++ {
		writer.writeFloat32(float32(mines[i].X))
		writer.writeFloat32(float32(mines[i].Y))
		writer.writeFloat32(float32(mines[i].Size))
	}
}

func encodePlayersBinary(players []*playerState, writer *binaryWriter) {
	count := len(players)
	if count > 32 {
		count = 32
	}
	writer.writeUint8(uint8(count))
	for i := 0; i < count; i++ {
		p := players[i]
		writer.writeString(p.ID)
		writer.writeString(p.Name)
		writer.writeUint32(uint32(p.Score))
		writer.writeBool(p.Ready)
		writer.writeBool(p.Alive)
		writer.writeFloat32(float32(p.X))
		writer.writeFloat32(float32(p.Y))
		writer.writeFloat32(float32(p.Size))
		writer.writeInt16(int16(p.Facing))
		writer.writeBool(p.Moving)
		writer.writeFloat32(float32(p.WalkCycle))
		writer.writeFloat32(float32(p.StepAccum))

		appearance := stringifyAppearance(p.Appearance)
		if len(appearance) > 300 {
			appearance = appearance[:300]
		}
		writer.writeString(appearance)
	}
}

func encodeStateBinary(state gameState) []byte {
	writer := &binaryWriter{}
	writer.writeUint8(messageTypeFull)
	writer.writeUint32(uint32(state.ServerTime))
	writer.writeUint32(state.TickIndex)
	writer.writeString(state.RoomName)
	writer.writeUint8(phaseCodes[state.Phase])
	writer.writeFloat32(float32(state.Countdown))
	writer.writeFloat32(float32(state.Remaining))
	writer.writeBool(state.Golden)
	writer.writeString(state.WinnerID)
	writer.writeString(state.Message)

	statusType := ""
	statusRemaining := float32(0)
	if state.Status != nil {
		statusType = state.Status.Type
		statusRemaining = float32(state.Status.Remaining)
	}
	writer.writeString(statusType)
	writer.writeFloat32(statusRemaining)

	writer.writeUint8(fishTypeCodes[state.Fish.Type])
	writer.writeFloat32(float32(state.Fish.X))
	writer.writeFloat32(float32(state.Fish.Y))
	writer.writeFloat32(float32(state.Fish.Size))
	writer.writeBool(state.Fish.Alive)
	writer.writeBool(state.Fish.Spawned)
	writer.writeInt16(int16(state.Fish.Direction))

	writer.writeBool(state.PowerUp.Active)
	writer.writeFloat32(float32(state.PowerUp.X))
	writer.writeFloat32(float32(state.PowerUp.Y))
	writer.writeFloat32(float32(state.PowerUp.Size))
	writer.writeFloat32(float32(state.PowerUp.Remaining))
	writer.writeUint8(powerUpTypeCodes[state.PowerUp.Type])

	encodeWallsBinary(state.Walls, writer)
	encodeMinesBinary(state.Mines, writer)
	encodePlayersBinary(state.Players, writer)
	return writer.bytes()
}

func encodePlayerPatchBinary(p playerPatch, writer *binaryWriter) {
	writer.writeString(p.ID)
	var flags1 uint8
	var flags2 uint8
	if p.Name != nil {
		flags1 |= 1 << 0
	}
	if p.Ready != nil {
		flags1 |= 1 << 1
	}
	if p.Alive != nil {
		flags1 |= 1 << 2
	}
	if p.X != nil {
		flags1 |= 1 << 3
	}
	if p.Y != nil {
		flags1 |= 1 << 4
	}
	if p.Size != nil {
		flags1 |= 1 << 5
	}
	if p.Facing != nil {
		flags1 |= 1 << 6
	}
	if p.Moving != nil {
		flags1 |= 1 << 7
	}
	if p.WalkCycle != nil {
		flags2 |= 1 << 0
	}
	if p.StepAccum != nil {
		flags2 |= 1 << 1
	}
	if p.Score != nil {
		flags2 |= 1 << 2
	}
	if len(p.Appearance) > 0 {
		flags2 |= 1 << 3
	}

	writer.writeUint8(flags1)
	writer.writeUint8(flags2)

	if p.Name != nil {
		writer.writeString(*p.Name)
	}
	if p.Ready != nil {
		writer.writeBool(*p.Ready)
	}
	if p.Alive != nil {
		writer.writeBool(*p.Alive)
	}
	if p.X != nil {
		writer.writeFloat32(float32(*p.X))
	}
	if p.Y != nil {
		writer.writeFloat32(float32(*p.Y))
	}
	if p.Size != nil {
		writer.writeFloat32(float32(*p.Size))
	}
	if p.Facing != nil {
		writer.writeInt16(int16(*p.Facing))
	}
	if p.Moving != nil {
		writer.writeBool(*p.Moving)
	}
	if p.WalkCycle != nil {
		writer.writeFloat32(float32(*p.WalkCycle))
	}
	if p.StepAccum != nil {
		writer.writeFloat32(float32(*p.StepAccum))
	}
	if p.Score != nil {
		writer.writeUint32(uint32(*p.Score))
	}
	if len(p.Appearance) > 0 {
		appearance := stringifyAppearance(p.Appearance)
		if len(appearance) > 300 {
			appearance = appearance[:300]
		}
		writer.writeString(appearance)
	}
}

func encodePatchBinary(patch *statePatch, serverTime int64, tickIndex uint32) []byte {
	writer := &binaryWriter{}
	writer.writeUint8(messageTypePatch)
	writer.writeUint32(uint32(serverTime))
	writer.writeUint32(tickIndex)

	var flags1 uint8
	var flags2 uint8

	if patch.Phase != nil {
		flags1 |= 1 << 0
	}
	if patch.Countdown != nil {
		flags1 |= 1 << 1
	}
	if patch.Remaining != nil {
		flags1 |= 1 << 2
	}
	if patch.Message != nil {
		flags1 |= 1 << 3
	}
	if patch.WinnerID != nil {
		flags1 |= 1 << 4
	}
	if patch.Golden != nil {
		flags1 |= 1 << 5
	}
	if patch.Status != nil {
		flags1 |= 1 << 6
	}
	if patch.Fish != nil {
		flags1 |= 1 << 7
	}

	if patch.PowerUp != nil {
		flags2 |= 1 << 0
	}
	if len(patch.Walls) > 0 {
		flags2 |= 1 << 1
	}
	if len(patch.Mines) > 0 {
		flags2 |= 1 << 2
	}
	if len(patch.Players) > 0 {
		flags2 |= 1 << 3
	}
	if len(patch.RemovedPlayers) > 0 {
		flags2 |= 1 << 4
	}

	writer.writeUint8(flags1)
	writer.writeUint8(flags2)

	if patch.Phase != nil {
		writer.writeString(*patch.Phase)
	}
	if patch.Countdown != nil {
		writer.writeFloat32(float32(*patch.Countdown))
	}
	if patch.Remaining != nil {
		writer.writeFloat32(float32(*patch.Remaining))
	}
	if patch.Message != nil {
		writer.writeString(*patch.Message)
	}
	if patch.WinnerID != nil {
		writer.writeString(*patch.WinnerID)
	}
	if patch.Golden != nil {
		writer.writeBool(*patch.Golden)
	}
	if patch.Status != nil {
		writer.writeString(patch.Status.Type)
		writer.writeFloat32(float32(patch.Status.Remaining))
	}
	if patch.Fish != nil {
		writer.writeUint8(fishTypeCodes[patch.Fish.Type])
		writer.writeFloat32(float32(patch.Fish.X))
		writer.writeFloat32(float32(patch.Fish.Y))
		writer.writeFloat32(float32(patch.Fish.Size))
		writer.writeBool(patch.Fish.Alive)
		writer.writeBool(patch.Fish.Spawned)
		writer.writeInt16(int16(patch.Fish.Direction))
	}
	if patch.PowerUp != nil {
		writer.writeBool(patch.PowerUp.Active)
		writer.writeFloat32(float32(patch.PowerUp.X))
		writer.writeFloat32(float32(patch.PowerUp.Y))
		writer.writeFloat32(float32(patch.PowerUp.Size))
		writer.writeFloat32(float32(patch.PowerUp.Remaining))
		writer.writeUint8(powerUpTypeCodes[patch.PowerUp.Type])
	}
	if len(patch.Walls) > 0 {
		encodeWallsBinary(patch.Walls, writer)
	}
	if len(patch.Mines) > 0 {
		encodeMinesBinary(patch.Mines, writer)
	}
	if len(patch.Players) > 0 {
		count := len(patch.Players)
		if count > 32 {
			count = 32
		}
		writer.writeUint8(uint8(count))
		for i := 0; i < count; i++ {
			encodePlayerPatchBinary(patch.Players[i], writer)
		}
	}
	if len(patch.RemovedPlayers) > 0 {
		count := len(patch.RemovedPlayers)
		if count > 32 {
			count = 32
		}
		writer.writeUint8(uint8(count))
		for i := 0; i < count; i++ {
			writer.writeString(patch.RemovedPlayers[i])
		}
	}

	return writer.bytes()
}

func quantizeStateForSend(state *gameState) {
	state.Countdown = quantizeSeconds(state.Countdown)
	state.Remaining = quantizeSeconds(state.Remaining)

	state.Fish.X = quantizeCoord(state.Fish.X)
	state.Fish.Y = quantizeCoord(state.Fish.Y)
	state.Fish.Size = quantizeCoord(state.Fish.Size)

	state.PowerUp.X = quantizeCoord(state.PowerUp.X)
	state.PowerUp.Y = quantizeCoord(state.PowerUp.Y)
	state.PowerUp.Size = quantizeCoord(state.PowerUp.Size)
	state.PowerUp.Remaining = quantizeSeconds(state.PowerUp.Remaining)

	if state.Status != nil {
		state.Status.Remaining = quantizeSeconds(state.Status.Remaining)
	}

	for i := range state.Walls {
		state.Walls[i].X = quantizeCoord(state.Walls[i].X)
		state.Walls[i].Y = quantizeCoord(state.Walls[i].Y)
		state.Walls[i].Width = quantizeCoord(state.Walls[i].Width)
		state.Walls[i].Height = quantizeCoord(state.Walls[i].Height)
	}

	for i := range state.Mines {
		state.Mines[i].X = quantizeCoord(state.Mines[i].X)
		state.Mines[i].Y = quantizeCoord(state.Mines[i].Y)
		state.Mines[i].Size = quantizeCoord(state.Mines[i].Size)
	}

	for _, p := range state.Players {
		quantizePlayerForSend(p)
	}
}

func statusEqual(a, b *statusEffect) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Type == b.Type && !floatChanged(a.Remaining, b.Remaining)
}

func fishEqual(a, b fishState) bool {
	return !floatChanged(a.X, b.X) && !floatChanged(a.Y, b.Y) && !floatChanged(a.Size, b.Size) && a.Alive == b.Alive && a.Type == b.Type && a.Direction == b.Direction && a.Spawned == b.Spawned
}

func powerUpEqual(a, b powerUpState) bool {
	return !floatChanged(a.X, b.X) && !floatChanged(a.Y, b.Y) && !floatChanged(a.Size, b.Size) && !floatChanged(a.Remaining, b.Remaining) && a.Active == b.Active && a.Type == b.Type
}

func wallsEqual(a, b []wall) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if floatChanged(a[i].X, b[i].X) || floatChanged(a[i].Y, b[i].Y) || floatChanged(a[i].Width, b[i].Width) || floatChanged(a[i].Height, b[i].Height) {
			return false
		}
	}
	return true
}

func minesEqual(a, b []mine) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if floatChanged(a[i].X, b[i].X) || floatChanged(a[i].Y, b[i].Y) || floatChanged(a[i].Size, b[i].Size) {
			return false
		}
	}
	return true
}

func appearanceEqual(a, b *playerState) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return stringifyAppearance(a.Appearance) == stringifyAppearance(b.Appearance)
}

func cloneWalls(src []wall) []wall {
	if len(src) == 0 {
		return nil
	}
	dst := make([]wall, len(src))
	copy(dst, src)
	return dst
}

func cloneMines(src []mine) []mine {
	if len(src) == 0 {
		return nil
	}
	dst := make([]mine, len(src))
	copy(dst, src)
	return dst
}

func (r *room) snapshotLocked() gameState {
	stateCopy := r.state
	players := make([]*playerState, 0, len(r.players))
	for _, p := range r.players {
		copy := *p
		players = append(players, &copy)
	}
	stateCopy.Players = players
	stateCopy.Walls = cloneWalls(r.state.Walls)
	stateCopy.Mines = cloneMines(r.state.Mines)
	return stateCopy
}

func (p playerPatch) isEmpty() bool {
	return p.Name == nil && p.Ready == nil && p.Alive == nil && p.X == nil && p.Y == nil && p.Size == nil && p.Facing == nil && p.Moving == nil && p.WalkCycle == nil && p.StepAccum == nil && p.Score == nil && len(p.Appearance) == 0
}

func buildPlayerPatch(previous, current *playerState) *playerPatch {
	if current == nil {
		return nil
	}
	patch := playerPatch{ID: current.ID}
	if previous == nil || previous.Name != current.Name {
		patch.Name = stringPtr(current.Name)
	}
	if previous == nil || previous.Ready != current.Ready {
		patch.Ready = boolPtr(current.Ready)
	}
	if previous == nil || previous.Alive != current.Alive {
		patch.Alive = boolPtr(current.Alive)
	}
	if previous == nil || floatChanged(previous.X, current.X) {
		patch.X = floatPtr(current.X)
	}
	if previous == nil || floatChanged(previous.Y, current.Y) {
		patch.Y = floatPtr(current.Y)
	}
	if previous == nil || floatChanged(previous.Size, current.Size) {
		patch.Size = floatPtr(current.Size)
	}
	if previous == nil || previous.Facing != current.Facing {
		patch.Facing = intPtr(current.Facing)
	}
	if previous == nil || previous.Moving != current.Moving {
		patch.Moving = boolPtr(current.Moving)
	}
	if previous == nil || floatChanged(previous.WalkCycle, current.WalkCycle) {
		patch.WalkCycle = floatPtr(current.WalkCycle)
	}
	if previous == nil || floatChanged(previous.StepAccum, current.StepAccum) {
		patch.StepAccum = floatPtr(current.StepAccum)
	}
	if previous == nil || previous.Score != current.Score {
		patch.Score = intPtr(current.Score)
	}
	if !appearanceEqual(previous, current) {
		patch.Appearance = current.Appearance
	}
	if patch.isEmpty() {
		return nil
	}
	return &patch
}

func (p *statePatch) isEmpty() bool {
	return p == nil || (p.Phase == nil && p.Countdown == nil && p.Remaining == nil && p.Message == nil && p.WinnerID == nil && p.Status == nil && p.Fish == nil && p.PowerUp == nil && len(p.Walls) == 0 && len(p.Mines) == 0 && len(p.Players) == 0 && len(p.RemovedPlayers) == 0 && p.Golden == nil)
}

func buildStatePatch(previous, current gameState) *statePatch {
	patch := &statePatch{}

	if previous.Phase != current.Phase {
		patch.Phase = stringPtr(current.Phase)
	}
	if floatChanged(previous.Countdown, current.Countdown) {
		patch.Countdown = floatPtr(current.Countdown)
	}
	if floatChanged(previous.Remaining, current.Remaining) {
		patch.Remaining = floatPtr(current.Remaining)
	}
	if previous.Message != current.Message {
		patch.Message = stringPtr(current.Message)
	}
	if previous.WinnerID != current.WinnerID {
		patch.WinnerID = stringPtr(current.WinnerID)
	}
	if previous.Golden != current.Golden {
		patch.Golden = boolPtr(current.Golden)
	}
	if !statusEqual(previous.Status, current.Status) {
		patch.Status = current.Status
	}
	if !fishEqual(previous.Fish, current.Fish) {
		fishCopy := current.Fish
		patch.Fish = &fishCopy
	}
	if !powerUpEqual(previous.PowerUp, current.PowerUp) {
		powerCopy := current.PowerUp
		patch.PowerUp = &powerCopy
	}
	if !wallsEqual(previous.Walls, current.Walls) {
		patch.Walls = cloneWalls(current.Walls)
	}
	if !minesEqual(previous.Mines, current.Mines) {
		patch.Mines = cloneMines(current.Mines)
	}

	prevPlayers := make(map[string]*playerState)
	for _, p := range previous.Players {
		prevPlayers[p.ID] = p
	}
	currentPlayers := make(map[string]*playerState)
	for _, p := range current.Players {
		currentPlayers[p.ID] = p
	}

	for id := range prevPlayers {
		if _, ok := currentPlayers[id]; !ok {
			patch.RemovedPlayers = append(patch.RemovedPlayers, id)
		}
	}

	for id, p := range currentPlayers {
		patchEntry := buildPlayerPatch(prevPlayers[id], p)
		if patchEntry != nil {
			patch.Players = append(patch.Players, *patchEntry)
		}
	}

	if patch.isEmpty() {
		return nil
	}
	return patch
}

type room struct {
	name               string
	players            map[string]*playerState
	inputs             map[string]vector
	connections        map[*websocket.Conn]string
	disconnectTimers   map[string]*time.Timer
	state              gameState
	lastBroadcastState *gameState
	server             *server
	mu                 sync.Mutex
	cancel             chan struct{}
	tickIndex          uint32
}

type server struct {
	cats     map[string]catProfile
	scores   []scoreEntry
	rooms    map[string]*room
	mu       sync.Mutex
	upgrader websocket.Upgrader
}

func newServer() *server {
	srv := &server{
		cats:     make(map[string]catProfile),
		rooms:    make(map[string]*room),
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	}
	srv.loadFromDisk()
	return srv
}

func (s *server) getOrCreateRoom(name string) *room {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.rooms[name]; ok {
		return existing
	}
	r := &room{
		name:             name,
		players:          make(map[string]*playerState),
		inputs:           make(map[string]vector),
		connections:      make(map[*websocket.Conn]string),
		disconnectTimers: make(map[string]*time.Timer),
		cancel:           make(chan struct{}),
		server:           s,
	}
	r.state = gameState{
		RoomName:   name,
		Phase:      "lobby",
		Fish:       fishState{X: worldSize / 2, Y: worldSize / 2, Size: fishSize, Alive: false, Type: "normal", Direction: 1},
		PowerUp:    powerUpState{Size: powerUpSize},
		Remaining:  roundDuration.Seconds(),
		Message:    "Ожидаем игроков",
		ServerTime: time.Now().UnixMilli(),
	}
	s.rooms[name] = r
	go r.run()
	return r
}

func (s *server) removeConnection(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rooms {
		r.dropConnection(conn)
	}
}

func (s *server) saveCat(profile catProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cats[profile.PlayerID] = profile
	s.persistLocked()
}

func (s *server) getCat(id string) (catProfile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof, ok := s.cats[id]
	return prof, ok
}

func (s *server) addScore(entry scoreEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry.CreatedAt = time.Now()
	s.scores = append(s.scores, entry)
	// keep scores sorted desc and trimmed
	sort.SliceStable(s.scores, func(i, j int) bool {
		if s.scores[i].Score == s.scores[j].Score {
			return s.scores[i].CreatedAt.Before(s.scores[j].CreatedAt)
		}
		return s.scores[i].Score > s.scores[j].Score
	})
	if len(s.scores) > 50 {
		s.scores = s.scores[:50]
	}
	s.persistLocked()
}

func (s *server) loadFromDisk() {
	data, err := os.ReadFile(dataFileName)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("failed to read data file: %v", err)
		}
		return
	}

	var payload struct {
		Cats   map[string]catProfile `json:"cats"`
		Scores []scoreEntry          `json:"scores"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		log.Printf("failed to decode data file: %v", err)
		return
	}

	if payload.Cats != nil {
		s.cats = payload.Cats
	}
	if payload.Scores != nil {
		s.scores = payload.Scores
	}
}

func (s *server) persistLocked() {
	payload := struct {
		Cats   map[string]catProfile `json:"cats"`
		Scores []scoreEntry          `json:"scores"`
	}{
		Cats:   s.cats,
		Scores: s.scores,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		log.Printf("failed to encode data file: %v", err)
		return
	}

	if err := os.WriteFile(dataFileName, data, 0o644); err != nil {
		log.Printf("failed to write data file: %v", err)
	}
}

func (s *server) topScores(limit int) []scoreEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit > len(s.scores) {
		limit = len(s.scores)
	}
	out := make([]scoreEntry, limit)
	copy(out, s.scores[:limit])
	return out
}

func (s *server) listRooms() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	rooms := make([]map[string]any, 0, len(s.rooms))
	for _, r := range s.rooms {
		rooms = append(rooms, map[string]any{
			"roomName":    r.name,
			"phase":       r.state.Phase,
			"playerCount": len(r.players),
			"updatedAt":   time.Now().UnixMilli(),
		})
	}
	return rooms
}

func (s *server) handleCats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		profile, ok := s.getCat(id)
		if !ok {
			writeJSON(w, map[string]any{"appearance": nil})
			return
		}
		writeJSON(w, profile)
	case http.MethodPost:
		var payload catProfile
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		payload.PlayerID = id
		s.saveCat(payload)
		writeJSON(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) handleScores(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"scores": s.topScores(10)})
	case http.MethodPost:
		var payload scoreEntry
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		if payload.Name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		s.addScore(payload)
		writeJSON(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) handleRooms(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"rooms": s.listRooms()})
}

func (s *server) handleWS(w http.ResponseWriter, r *http.Request) {
	roomName := r.URL.Query().Get("room")
	playerID := r.URL.Query().Get("playerId")
	playerName := r.URL.Query().Get("name")
	if roomName == "" || playerID == "" {
		http.Error(w, "room and playerId required", http.StatusBadRequest)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}
	rInstance := s.getOrCreateRoom(roomName)
	rInstance.handleConnection(conn, playerID, playerName)
}

func (r *room) handleConnection(conn *websocket.Conn, playerID, playerName string) {
	r.mu.Lock()
	r.connections[conn] = playerID
	_ = r.ensurePlayer(playerID, playerName)
	r.cancelDisconnectTimerLocked(playerID)
	r.mu.Unlock()

	r.sendFullState(conn)

	go func() {
		defer func() {
			conn.Close()
			r.dropConnection(conn)
		}()
		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if messageType == websocket.BinaryMessage {
				if pid, vec := decodeInputBuffer(data); pid != nil && vec != nil {
					r.mu.Lock()
					r.inputs[*pid] = *vec
					r.mu.Unlock()
				}
				continue
			}
			var msg wsMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			r.handleClientMessage(playerID, msg)
		}
	}()
}

func (r *room) handleClientMessage(playerID string, msg wsMessage) {
	switch msg.Type {
	case "ready":
		if msg.Ready != nil {
			r.setReady(playerID, *msg.Ready)
		}
	case "input":
		if msg.Vector != nil {
			r.mu.Lock()
			r.inputs[playerID] = *msg.Vector
			r.mu.Unlock()
		}
	case "chat":
		if msg.Message != nil {
			r.broadcastChat(*msg.Message)
		}
	case "appearance":
		if msg.Appearance != nil {
			r.updateAppearance(playerID, msg.Appearance)
		}
	}
}

func (r *room) setReady(playerID string, ready bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if player, ok := r.players[playerID]; ok {
		player.Ready = ready
	}
	r.updatePhaseLocked()
}

func (r *room) updatePhaseLocked() {
	r.updateLobbyMessageLocked()
}

func (r *room) updateAppearance(playerID string, appearance catAppearance) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if player, ok := r.players[playerID]; ok {
		player.Appearance = appearance
	}
}

func (r *room) dropConnection(conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.connections[conn]
	delete(r.connections, conn)
	conn.Close()
	if ok {
		r.inputs[id] = vector{}
		r.schedulePlayerRemovalLocked(id)
	}
}

func (r *room) schedulePlayerRemovalLocked(playerID string) {
	if timer, ok := r.disconnectTimers[playerID]; ok {
		timer.Stop()
	}

	timer := time.AfterFunc(reconnectGrace, func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		currentTimer, ok := r.disconnectTimers[playerID]
		if !ok || currentTimer != timer {
			return
		}
		delete(r.disconnectTimers, playerID)
		delete(r.inputs, playerID)
		delete(r.players, playerID)
		if len(r.players) == 0 {
			r.state.Phase = "lobby"
			r.state.Message = "Ожидаем игроков"
		}
	})

	r.disconnectTimers[playerID] = timer
}

func (r *room) cancelDisconnectTimerLocked(playerID string) {
	if timer, ok := r.disconnectTimers[playerID]; ok {
		timer.Stop()
		delete(r.disconnectTimers, playerID)
	}
}

func (r *room) ensurePlayer(id, name string) *playerState {
	player, ok := r.players[id]
	if !ok {
		player = &playerState{ID: id, Name: fallbackName(name), Size: catSize, X: worldSize / 2, Y: worldSize / 2, Facing: 1}
		r.players[id] = player
	}
	if name != "" {
		player.Name = name
	}
	return player
}

func (r *room) sendFullState(conn *websocket.Conn) {
	r.mu.Lock()
	stateCopy := r.snapshotLocked()
	r.mu.Unlock()
	stateCopy.TickIndex = r.tickIndex
	quantizeStateForSend(&stateCopy)
	data := encodeStateBinary(stateCopy)
	conn.WriteMessage(websocket.BinaryMessage, data)
}

func (r *room) run() {
	tick := time.NewTicker(tickRate)
	broadcast := time.NewTicker(broadcastRate)
	for {
		select {
		case <-r.cancel:
			tick.Stop()
			broadcast.Stop()
			return
		case <-tick.C:
			r.step()
		case <-broadcast.C:
			r.broadcastState()
		}
	}
}

func (r *room) step() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	r.state.ServerTime = now.UnixMilli()
	r.state.TickIndex = r.tickIndex
	r.tickIndex++
	switch r.state.Phase {
	case "countdown":
		r.state.Countdown -= tickRate.Seconds()
		if r.state.Countdown <= 0 {
			r.beginRoundLocked()
		}
	case "playing":
		r.state.Remaining -= tickRate.Seconds()
		r.updateStatusEffectLocked()
		r.updatePowerUpLocked()
		r.updatePlayersLocked()
		if r.countAlivePlayersLocked() == 0 {
			r.endRoundLocked("Раунд завершён: все коты погибли")
			return
		}
		r.updateFishLocked()
		if r.state.Remaining <= 0 {
			r.endRoundLocked("Раунд завершён")
		}
	default:
		r.updateLobbyMessageLocked()
	}
}

func (r *room) beginRoundLocked() {
	r.state.Phase = "playing"
	r.state.Countdown = 0
	r.state.Remaining = roundDuration.Seconds()
	r.state.Message = "Раунд начался"
	r.state.Status = nil
	r.state.Walls = nil
	r.state.Mines = nil
	r.state.PowerUp = powerUpState{Size: powerUpSize}
	r.spawnFishLocked()
	for _, p := range r.players {
		p.Alive = true
		p.Score = 0
	}
}

func (r *room) endRoundLocked(reason string) {
	r.state.Phase = "ended"
	if reason != "" {
		r.state.Message = reason
	} else {
		r.state.Message = "Раунд завершён"
	}
	r.state.Fish.Alive = false
	r.state.Countdown = 0
	r.state.WinnerID = r.bestPlayerIDLocked()
}

func (r *room) bestPlayerIDLocked() string {
	var best *playerState
	for _, p := range r.players {
		if best == nil || p.Score > best.Score {
			best = p
		}
	}
	if best == nil {
		return ""
	}
	return best.ID
}

func (r *room) updatePlayersLocked() {
	speedMultiplier := r.getSpeedMultiplierLocked()
	for id, p := range r.players {
		if !p.Alive {
			continue
		}
		input := r.inputs[id]
		speed := catSpeed * tickRate.Seconds() * speedMultiplier
		p.X += input.X * speed
		p.Y += input.Y * speed
		p.Moving = math.Abs(input.X) > 0.01 || math.Abs(input.Y) > 0.01
		if p.Moving {
			p.Facing = 1
			if input.X < -0.01 {
				p.Facing = -1
			}
			p.StepAccum += tickRate.Seconds() * 4
			p.WalkCycle = math.Mod(p.StepAccum, 1)
		}
		resolveEntityWallCollisions(p, r.state.Walls)
		p.X = clampFloat(p.X, p.Size/2, worldSize-p.Size/2)
		p.Y = clampFloat(p.Y, p.Size/2, worldSize-p.Size/2)

		if r.state.PowerUp.Active {
			dist := math.Hypot(p.X-r.state.PowerUp.X, p.Y-r.state.PowerUp.Y)
			if dist < (p.Size+r.state.PowerUp.Size)/2 {
				r.state.PowerUp.Active = false
				r.state.PowerUp.Remaining = 0
				r.applyRandomStatusEffectLocked()
			}
		}
		for _, m := range r.state.Mines {
			if math.Hypot(p.X-m.X, p.Y-m.Y) < (p.Size+m.Size)/2 {
				p.Alive = false
				p.Moving = false
				break
			}
		}
	}
}

func (r *room) countAlivePlayersLocked() int {
	count := 0
	for _, p := range r.players {
		if p.Alive {
			count++
		}
	}
	return count
}

func (r *room) updateFishLocked() {
	if !r.state.Fish.Alive {
		r.spawnFishLocked()
		return
	}
	swimStep := float64(r.state.Fish.Direction) * fishSwimSpeed * tickRate.Seconds()
	nextX := r.state.Fish.X + swimStep
	if r.fishCollidesAt(nextX) {
		r.state.Fish.Direction *= -1
		nextX = r.state.Fish.X + float64(r.state.Fish.Direction)*fishSwimSpeed*tickRate.Seconds()
		if r.fishCollidesAt(nextX) {
			nextX = r.state.Fish.X
		}
	}
	r.state.Fish.X = clampFloat(nextX, r.state.Fish.Size/2, worldSize-r.state.Fish.Size/2)

	for _, p := range r.players {
		if !p.Alive {
			continue
		}
		dist := math.Hypot(p.X-r.state.Fish.X, p.Y-r.state.Fish.Y)
		if dist <= fishCatchDistance {
			p.Score += 1
			r.spawnFishLocked()
			break
		}
	}
}

func (r *room) updatePowerUpLocked() {
	if r.state.Phase != "playing" {
		return
	}
	if !r.state.PowerUp.Active {
		return
	}
	r.state.PowerUp.Remaining -= tickRate.Seconds()
	if r.state.PowerUp.Remaining <= 0 {
		r.clearPowerUpLocked()
		return
	}
}

func (r *room) updateStatusEffectLocked() {
	if r.state.Status == nil {
		return
	}
	r.state.Status.Remaining -= tickRate.Seconds()
	if r.state.Status.Remaining <= 0 {
		r.state.Status = nil
	}
}

func (r *room) getSpeedMultiplierLocked() float64 {
	if r.state.Status == nil {
		return 1
	}
	switch r.state.Status.Type {
	case "speedUp":
		return 2
	case "speedDown":
		return 1 / 1.5
	default:
		return 1
	}
}

func (r *room) spawnFishLocked() {
	margin := 30.0
	fish := &r.state.Fish
	alivePlayers := make([]*playerState, 0, len(r.players))
	for _, p := range r.players {
		if p.Alive {
			alivePlayers = append(alivePlayers, p)
		}
	}
	if len(alivePlayers) == 0 {
		for _, p := range r.players {
			alivePlayers = append(alivePlayers, p)
			break
		}
	}

	catCells := make([]gridCell, 0, len(alivePlayers))
	for _, p := range alivePlayers {
		catCells = append(catCells, positionToGridCell(p.X, p.Y))
	}

	placed := false
	for attempt := 0; attempt < 200; attempt++ {
		x := margin + rand.Float64()*(worldSize-margin*2)
		y := margin + rand.Float64()*(worldSize-margin*2)
		fishCell := positionToGridCell(x, y)
		if containsCell(catCells, fishCell) {
			continue
		}
		candidateWalls := r.generateWallsLayoutForPlayers(catCells, fishCell)
		if candidateWalls == nil {
			continue
		}
		if circleIntersectsAnyWall(x, y, fish.Size/2+2, candidateWalls) {
			continue
		}
		r.state.Walls = candidateWalls
		r.handlePowerUpAfterWallChangeLocked()
		r.resolvePlayersAfterWallChangeLocked()
		fish.X = x
		fish.Y = y
		fish.Alive = true
		fish.Size = fishSize
		fish.Type = "normal"
		fish.Direction = 1
		fish.Spawned = true
		r.state.Mines = r.generateMinesLocked()
		r.refreshPowerUpLocked()
		placed = true
		break
	}

	if !placed {
		r.state.Walls = nil
		r.handlePowerUpAfterWallChangeLocked()
		r.resolvePlayersAfterWallChangeLocked()
		fish.X = worldSize / 2
		fish.Y = worldSize / 2
		fish.Alive = true
		fish.Size = fishSize
		fish.Type = "normal"
		fish.Direction = 1
		fish.Spawned = true
		r.state.Mines = r.generateMinesLocked()
		r.refreshPowerUpLocked()
	}
}

func (r *room) updateLobbyMessageLocked() {
	readyCount := 0
	for _, p := range r.players {
		if p.Ready {
			readyCount++
		}
	}
	total := len(r.players)
	if total == 0 {
		r.state.Message = "Ожидаем игроков"
		return
	}
	if readyCount == total {
		r.state.Message = "Все игроки готовы"
		r.state.Phase = "countdown"
		r.state.Countdown = countdownDuration.Seconds()
	} else {
		r.state.Message = "Ожидаем готовности игроков"
		r.state.Phase = "lobby"
	}
}

func (r *room) broadcastState() {
	r.mu.Lock()
	stateCopy := r.snapshotLocked()
	quantizeStateForSend(&stateCopy)
	stateCopy.TickIndex = r.tickIndex
	previous := r.lastBroadcastState
	connections := make([]*websocket.Conn, 0, len(r.connections))
	for conn := range r.connections {
		connections = append(connections, conn)
	}
	stateSnapshot := stateCopy
	stateSnapshot.Fish.Spawned = false
	r.lastBroadcastState = &stateSnapshot
	r.state.Fish.Spawned = false
	r.mu.Unlock()

	var data []byte
	if previous == nil {
		data = encodeStateBinary(stateCopy)
	} else if patch := buildStatePatch(*previous, stateCopy); patch != nil {
		data = encodePatchBinary(patch, stateCopy.ServerTime, stateCopy.TickIndex)
	} else {
		return
	}
	for _, conn := range connections {
		conn.WriteMessage(websocket.BinaryMessage, data)
	}
}

func (r *room) broadcastChat(msg chatMessage) {
	msg.At = time.Now().UnixMilli()
	data, _ := json.Marshal(wsMessage{Type: "chat", Message: &msg})
	r.mu.Lock()
	defer r.mu.Unlock()
	for conn := range r.connections {
		conn.WriteMessage(websocket.TextMessage, data)
	}
}

func (r *room) applyRandomStatusEffectLocked() {
	effects := []string{"speedUp", "speedDown", "timeIncrease", "timeDecrease"}
	if len(effects) == 0 {
		return
	}
	typeChoice := effects[rand.Intn(len(effects))]
	r.state.Status = &statusEffect{Type: typeChoice, Remaining: powerUpDuration}
	if typeChoice == "timeIncrease" {
		r.state.Remaining = math.Max(r.state.Remaining, timeIncreaseLimit)
	} else if typeChoice == "timeDecrease" {
		r.state.Remaining = math.Min(r.state.Remaining, timeDecreaseLimit)
	}
}

func (r *room) clearPowerUpLocked() {
	r.state.PowerUp.Active = false
	r.state.PowerUp.Remaining = 0
	r.state.PowerUp.X = 0
	r.state.PowerUp.Y = 0
}

func (r *room) spawnPowerUpLocked() {
	margin := 36.0
	for attempt := 0; attempt < 40; attempt++ {
		x := margin + rand.Float64()*(worldSize-margin*2)
		y := margin + rand.Float64()*(worldSize-margin*2)
		if !circleIntersectsAnyWall(x, y, r.state.PowerUp.Size/2+2, r.state.Walls) {
			r.state.PowerUp.X = x
			r.state.PowerUp.Y = y
			r.state.PowerUp.Active = true
			r.state.PowerUp.Remaining = powerUpLifetime
			return
		}
	}
	r.clearPowerUpLocked()
}

func (r *room) refreshPowerUpLocked() {
	if rand.Float64() < powerUpChance {
		r.spawnPowerUpLocked()
	} else if r.state.PowerUp.Active {
		r.state.PowerUp.Remaining = math.Min(r.state.PowerUp.Remaining, powerUpLifetime)
	} else {
		r.clearPowerUpLocked()
	}
}

func (r *room) resolvePlayersAfterWallChangeLocked() {
	for _, p := range r.players {
		resolveEntityWallCollisions(p, r.state.Walls)
		p.X = clampFloat(p.X, p.Size/2, worldSize-p.Size/2)
		p.Y = clampFloat(p.Y, p.Size/2, worldSize-p.Size/2)
	}
}

func (r *room) handlePowerUpAfterWallChangeLocked() {
	if r.state.PowerUp.Active && circleIntersectsAnyWall(r.state.PowerUp.X, r.state.PowerUp.Y, r.state.PowerUp.Size/2+2, r.state.Walls) {
		r.clearPowerUpLocked()
	}
}

func (r *room) generateWallsLayoutForPlayers(catCells []gridCell, fishCell gridCell) []wall {
	if len(catCells) == 0 {
		return nil
	}
	for attempt := 0; attempt < 160; attempt++ {
		segments := r.buildRandomWallSegments(catCells, fishCell)
		if segments == nil {
			continue
		}
		blockedGrid := buildBlockedGridFromSegments(segments)
		allReachable := true
		for _, c := range catCells {
			if !isPathAvailable(c, fishCell, blockedGrid) {
				allReachable = false
				break
			}
		}
		if !allReachable {
			continue
		}
		candidateWalls := convertSegmentsToWalls(segments)
		intersectsPlayer := false
		for _, p := range r.players {
			if entityIntersectsWalls(p, candidateWalls) {
				intersectsPlayer = true
				break
			}
		}
		if intersectsPlayer {
			continue
		}
		return candidateWalls
	}
	return nil
}

func (r *room) buildRandomWallSegments(catCells []gridCell, fishCell gridCell) []wallSegment {
	segments := []wallSegment{}
	occupied := make(map[string]struct{})
	totalLength := 0
	attempts := 0
	fishKey := cellKey(fishCell)
	catKeys := make(map[string]struct{}, len(catCells))
	for _, c := range catCells {
		catKeys[cellKey(c)] = struct{}{}
	}

	for totalLength < maxWallTotalLen && attempts < 80 {
		attempts++
		if len(segments) >= 2 && rand.Float64() < 0.35 {
			break
		}
		remaining := maxWallTotalLen - totalLength
		if remaining <= 0 {
			continue
		}
		maxSegmentLen := remaining
		if maxSegmentLen > 3 {
			maxSegmentLen = 3
		}
		length := 1 + rand.Intn(int(maxSegmentLen))
		orientation := "horizontal"
		if rand.Float64() < 0.5 {
			orientation = "vertical"
		}
		maxRow := gridSize - 1
		maxCol := gridSize - length
		if orientation == "vertical" {
			maxRow = gridSize - length
			maxCol = gridSize - 1
		}
		if maxRow < 0 || maxCol < 0 {
			continue
		}
		row := rand.Intn(maxRow + 1)
		col := rand.Intn(maxCol + 1)
		cells := getCellsForSegment(row, col, length, orientation)
		invalid := false
		for _, cell := range cells {
			key := cellKey(cell)
			if _, taken := occupied[key]; taken {
				invalid = true
				break
			}
			if _, isCat := catKeys[key]; isCat || key == fishKey {
				invalid = true
				break
			}
		}
		if invalid {
			continue
		}
		for _, cell := range cells {
			occupied[cellKey(cell)] = struct{}{}
		}
		segments = append(segments, wallSegment{Row: row, Col: col, Length: length, Orientation: orientation})
		totalLength += length
	}

	if len(segments) < 2 {
		return nil
	}
	return segments
}

func (r *room) generateMinesLocked() []mine {
	result := []mine{}
	mineCount := rand.Intn(maxMines + 1)
	if mineCount == 0 {
		return result
	}
	radius := mineSize / 2
	margin := radius + mineMinDistance + 4
	attempts := 0

	for len(result) < mineCount && attempts < 200 {
		attempts++
		x := margin + rand.Float64()*(worldSize-margin*2)
		y := margin + rand.Float64()*(worldSize-margin*2)
		if !r.isMinePositionValidLocked(x, y, radius, result) {
			continue
		}
		result = append(result, mine{X: x, Y: y, Size: mineSize})
	}
	return result
}

func (r *room) isMinePositionValidLocked(x, y, radius float64, existing []mine) bool {
	safeRadius := radius + mineMinDistance
	if x-safeRadius < 0 || y-safeRadius < 0 || x+safeRadius > worldSize || y+safeRadius > worldSize {
		return false
	}
	if circleIntersectsAnyWall(x, y, safeRadius, r.state.Walls) {
		return false
	}
	if r.state.Fish.Alive {
		dist := math.Hypot(x-r.state.Fish.X, y-r.state.Fish.Y)
		if dist <= r.state.Fish.Size/2+safeRadius {
			return false
		}
	}
	for _, p := range r.players {
		if !p.Alive {
			continue
		}
		dist := math.Hypot(x-p.X, y-p.Y)
		if dist <= p.Size/2+safeRadius {
			return false
		}
	}
	for _, m := range existing {
		dist := math.Hypot(x-m.X, y-m.Y)
		if dist <= m.Size/2+radius+mineMinDistance {
			return false
		}
	}
	return true
}

func clampGridIndex(v int) int {
	if v < 0 {
		return 0
	}
	if v >= gridSize {
		return gridSize - 1
	}
	return v
}

func positionToGridCell(x, y float64) gridCell {
	col := clampGridIndex(int(math.Floor(x / gridCellSize)))
	row := clampGridIndex(int(math.Floor(y / gridCellSize)))
	return gridCell{Row: row, Col: col}
}

func getCellsForSegment(row, col, length int, orientation string) []gridCell {
	cells := make([]gridCell, 0, length)
	for offset := 0; offset < length; offset++ {
		currentRow := row
		currentCol := col
		if orientation == "horizontal" {
			currentCol += offset
		} else {
			currentRow += offset
		}
		cells = append(cells, gridCell{Row: currentRow, Col: currentCol})
	}
	return cells
}

func buildBlockedGridFromSegments(segments []wallSegment) [][]bool {
	grid := make([][]bool, gridSize)
	for i := range grid {
		grid[i] = make([]bool, gridSize)
	}
	for _, seg := range segments {
		for offset := 0; offset < seg.Length; offset++ {
			row := seg.Row
			col := seg.Col
			if seg.Orientation == "horizontal" {
				col += offset
			} else {
				row += offset
			}
			if row >= 0 && row < gridSize && col >= 0 && col < gridSize {
				grid[row][col] = true
			}
		}
	}
	return grid
}

func isPathAvailable(catCell, fishCell gridCell, blocked [][]bool) bool {
	startKey := cellKey(catCell)
	targetKey := cellKey(fishCell)
	visited := map[string]struct{}{startKey: {}}
	queue := []gridCell{catCell}
	deltas := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		key := cellKey(current)
		if key == targetKey {
			return true
		}
		for _, delta := range deltas {
			nextRow := current.Row + delta[0]
			nextCol := current.Col + delta[1]
			if nextRow < 0 || nextRow >= gridSize || nextCol < 0 || nextCol >= gridSize {
				continue
			}
			if blocked[nextRow][nextCol] {
				continue
			}
			nextCell := gridCell{Row: nextRow, Col: nextCol}
			nextKey := cellKey(nextCell)
			if _, ok := visited[nextKey]; ok {
				continue
			}
			visited[nextKey] = struct{}{}
			queue = append(queue, nextCell)
		}
	}
	return false
}

func convertSegmentsToWalls(segments []wallSegment) []wall {
	walls := make([]wall, 0, len(segments))
	for _, seg := range segments {
		if seg.Orientation == "horizontal" {
			walls = append(walls, wall{
				X:      float64(seg.Col) * gridCellSize,
				Y:      float64(seg.Row)*gridCellSize + (gridCellSize-wallThickness)/2,
				Width:  float64(seg.Length) * gridCellSize,
				Height: wallThickness,
			})
		} else {
			walls = append(walls, wall{
				X:      float64(seg.Col)*gridCellSize + (gridCellSize-wallThickness)/2,
				Y:      float64(seg.Row) * gridCellSize,
				Width:  wallThickness,
				Height: float64(seg.Length) * gridCellSize,
			})
		}
	}
	return walls
}

func circleIntersectsRect(cx, cy, radius float64, rect wall) bool {
	closestX := clampFloat(cx, rect.X, rect.X+rect.Width)
	closestY := clampFloat(cy, rect.Y, rect.Y+rect.Height)
	dx := cx - closestX
	dy := cy - closestY
	return dx*dx+dy*dy < radius*radius
}

func circleIntersectsAnyWall(cx, cy, radius float64, walls []wall) bool {
	for _, w := range walls {
		if circleIntersectsRect(cx, cy, radius, w) {
			return true
		}
	}
	return false
}

func (r *room) fishCollidesAt(x float64) bool {
	radius := r.state.Fish.Size / 2
	if x-radius < 0 || x+radius > worldSize {
		return true
	}
	if circleIntersectsAnyWall(x, r.state.Fish.Y, radius, r.state.Walls) {
		return true
	}
	if r.state.PowerUp.Active {
		if math.Hypot(x-r.state.PowerUp.X, r.state.Fish.Y-r.state.PowerUp.Y) < radius+r.state.PowerUp.Size/2 {
			return true
		}
	}
	for _, m := range r.state.Mines {
		if math.Hypot(x-m.X, r.state.Fish.Y-m.Y) < radius+m.Size/2 {
			return true
		}
	}
	return false
}

func resolveEntityWallCollisions(entity *playerState, walls []wall) {
	if entity == nil || len(walls) == 0 {
		return
	}

	radius := entity.Size / 2
	moved := true
	iterations := 0

	for moved && iterations < 4 {
		moved = false
		iterations++
		for _, w := range walls {
			closestX := clampFloat(entity.X, w.X, w.X+w.Width)
			closestY := clampFloat(entity.Y, w.Y, w.Y+w.Height)
			dx := entity.X - closestX
			dy := entity.Y - closestY
			distanceSquared := dx*dx + dy*dy
			if distanceSquared < radius*radius {
				if distanceSquared == 0 {
					if w.Width < w.Height {
						dx = -1
						if entity.X >= w.X+w.Width/2 {
							dx = 1
						}
						dy = 0
					} else {
						dy = -1
						if entity.Y >= w.Y+w.Height/2 {
							dy = 1
						}
						dx = 0
					}
					distanceSquared = 1
				}
				distance := math.Sqrt(distanceSquared)
				overlap := radius - distance
				nx := dx / distance
				ny := dy / distance
				entity.X += nx * overlap
				entity.Y += ny * overlap
				moved = true
			}
		}
	}
}

func entityIntersectsWalls(entity *playerState, walls []wall) bool {
	radius := entity.Size/2 + 2
	for _, w := range walls {
		if circleIntersectsRect(entity.X, entity.Y, radius, w) {
			return true
		}
	}
	return false
}

func cellKey(c gridCell) string {
	return fmt.Sprintf("%d,%d", c.Row, c.Col)
}

func containsCell(cells []gridCell, target gridCell) bool {
	for _, c := range cells {
		if c.Row == target.Row && c.Col == target.Col {
			return true
		}
	}
	return false
}

func stringifyAppearance(app catAppearance) string {
	if len(app) == 0 {
		return ""
	}

	keys := make([]string, 0, len(app))
	for key := range app {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyJSON, _ := json.Marshal(key)
		buf.Write(keyJSON)
		buf.WriteByte(':')
		valueJSON, _ := json.Marshal(app[key])
		buf.Write(valueJSON)
	}
	buf.WriteByte('}')

	return buf.String()
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func fallbackName(name string) string {
	if name == "" {
		return "Игрок"
	}
	if len(name) > 32 {
		return name[:32]
	}
	return name
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	rand.Seed(time.Now().UnixNano())
	srv := newServer()

	http.Handle("/api/cats/{id}", withCORS(http.HandlerFunc(srv.handleCats)))
	http.Handle("/api/scores", withCORS(http.HandlerFunc(srv.handleScores)))
	http.Handle("/api/rooms", withCORS(http.HandlerFunc(srv.handleRooms)))
	http.Handle("/ws", withCORS(http.HandlerFunc(srv.handleWS))) // можно и без CORS, но не помешает

	addr := ":8080"
	log.Printf("Cat game server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
