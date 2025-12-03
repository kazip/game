package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

var (
	phaseCodes       = map[string]uint8{"lobby": 0, "countdown": 1, "playing": 2, "ended": 3}
	fishTypeCodes    = map[string]uint8{"normal": 0, "golden": 1, "timeIncrease": 2, "timeDecrease": 3}
	powerUpTypeCodes = map[string]uint8{"none": 0, "fast": 1, "slow": 2, "invert": 3, "memory": 4}

	messageTypeFull  uint8 = 0
	messageTypePatch uint8 = 1
)

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

func (r *binaryReader) readFloatVector() (*Vector, error) {
	x, err := r.readFloat32()
	if err != nil {
		return nil, err
	}
	y, err := r.readFloat32()
	if err != nil {
		return nil, err
	}
	return &Vector{X: float64(x), Y: float64(y)}, nil
}

func DecodeInputBuffer(data []byte) (*string, *Vector) {
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

func encodeWallsBinary(walls []Wall, writer *binaryWriter) {
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

func encodePowerUpsBinary(powerUps []PowerUpState, writer *binaryWriter) {
	count := len(powerUps)
	if count > 255 {
		count = 255
	}
	writer.writeUint8(uint8(count))
	for i := 0; i < count; i++ {
		writer.writeBool(powerUps[i].Active)
		writer.writeFloat32(float32(powerUps[i].X))
		writer.writeFloat32(float32(powerUps[i].Y))
		writer.writeFloat32(float32(powerUps[i].Size))
		writer.writeFloat32(float32(powerUps[i].Remaining))
		writer.writeUint8(powerUpTypeCodes[powerUps[i].Type])
	}
}

func encodeMinesBinary(mines []Mine, writer *binaryWriter) {
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

func encodePlayersBinary(players []PlayerState, writer *binaryWriter) {
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

		appearance := p.Appearance
		if len(appearance) > 300 {
			appearance = appearance[:300]
		}
		writer.writeString(appearance)
		writer.writeString(p.Disguise)
	}
}

func EncodeState(state GameState) []byte {
	writer := &binaryWriter{}
	writer.writeUint8(messageTypeFull)
	writer.writeUint32(uint32(state.ServerTime))
	writer.writeUint32(state.TickIndex)
	writer.writeString(state.RoomName)
	writer.writeString(state.Mode)
	writer.writeUint8(phaseCodes[state.Phase])
	writer.writeFloat32(float32(state.Countdown))
	writer.writeFloat32(float32(state.Remaining))
	writer.writeString(state.HidePhase)
	writer.writeBool(state.Golden)
	writer.writeString(state.WinnerID)
	writer.writeString(state.Message)
	writer.writeString(state.SeekerID)
	writer.writeString(state.BombHolder)
	writer.writeFloat32(float32(state.BombTimer))

	statusType := ""
	statusRemaining := float32(0)
	statusPlayer := ""
	if state.Status != nil {
		statusType = state.Status.Type
		statusRemaining = float32(state.Status.Remaining)
		statusPlayer = state.Status.PlayerID
	}
	writer.writeString(statusType)
	writer.writeFloat32(statusRemaining)
	writer.writeString(statusPlayer)

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

	encodePowerUpsBinary(state.PowerUps, writer)

	encodeWallsBinary(state.Walls, writer)
	encodeMinesBinary(state.Mines, writer)
	encodePlayersBinary(state.Players, writer)
	return writer.bytes()
}

func encodePlayerPatchBinary(p PlayerPatch, writer *binaryWriter) {
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
	if p.Appearance != nil {
		flags2 |= 1 << 3
	}
	if p.Disguise != nil {
		flags2 |= 1 << 4
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
	if p.Appearance != nil {
		appearance := *p.Appearance
		if len(appearance) > 300 {
			appearance = appearance[:300]
		}
		writer.writeString(appearance)
	}
	if p.Disguise != nil {
		writer.writeString(*p.Disguise)
	}
}

func EncodePatch(patch *StatePatch, serverTime int64, tickIndex uint32) []byte {
	writer := &binaryWriter{}
	writer.writeUint8(messageTypePatch)
	writer.writeUint32(uint32(serverTime))
	writer.writeUint32(tickIndex)

	var flags1 uint8
	var flags2 uint8
	var flags3 uint8

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
	if patch.Mode != nil {
		flags2 |= 1 << 5
	}
	if patch.BombHolder != nil {
		flags2 |= 1 << 6
	}
	if patch.BombTimer != nil {
		flags2 |= 1 << 7
	}

	if len(patch.PowerUps) > 0 {
		flags3 |= 1 << 0
	}
	if patch.SeekerID != nil {
		flags3 |= 1 << 1
	}
	if patch.HidePhase != nil {
		flags3 |= 1 << 2
	}

	writer.writeUint8(flags1)
	writer.writeUint8(flags2)
	writer.writeUint8(flags3)

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
		writer.writeString(patch.Status.PlayerID)
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
	if len(patch.PowerUps) > 0 {
		encodePowerUpsBinary(patch.PowerUps, writer)
	}
	if patch.SeekerID != nil {
		writer.writeString(*patch.SeekerID)
	}
	if patch.HidePhase != nil {
		writer.writeString(*patch.HidePhase)
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

	if patch.Mode != nil {
		writer.writeString(*patch.Mode)
	}
	if patch.BombHolder != nil {
		writer.writeString(*patch.BombHolder)
	}
	if patch.BombTimer != nil {
		writer.writeFloat32(float32(*patch.BombTimer))
	}

	return writer.bytes()
}
