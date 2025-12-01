package main

import (
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

	"catgame/protocol"

	"github.com/gorilla/websocket"
)

func quantizeStateForSend(state *gameState) {
	state.Countdown = quantizeSeconds(state.Countdown)
	state.Remaining = quantizeSeconds(state.Remaining)
	state.BombTimer = quantizeSeconds(state.BombTimer)

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

func decodeInputBuffer(data []byte) (*string, *vector) {
	id, vec := protocol.DecodeInputBuffer(data)
	if id == nil || vec == nil {
		return nil, nil
	}
	return id, &vector{X: vec.X, Y: vec.Y}
}

func toProtocolPlayerState(p *playerState) protocol.PlayerState {
	appearance := stringifyAppearance(p.Appearance)
	if len(appearance) > 300 {
		appearance = appearance[:300]
	}
	return protocol.PlayerState{
		ID:         p.ID,
		Name:       p.Name,
		Ready:      p.Ready,
		Alive:      p.Alive,
		X:          p.X,
		Y:          p.Y,
		Size:       p.Size,
		Facing:     p.Facing,
		Moving:     p.Moving,
		WalkCycle:  p.WalkCycle,
		StepAccum:  p.StepAccum,
		Score:      p.Score,
		Appearance: appearance,
	}
}

func toProtocolGameState(state gameState) protocol.GameState {
	players := make([]protocol.PlayerState, len(state.Players))
	for i, p := range state.Players {
		players[i] = toProtocolPlayerState(p)
	}
	walls := make([]protocol.Wall, len(state.Walls))
	for i, w := range state.Walls {
		walls[i] = protocol.Wall{X: w.X, Y: w.Y, Width: w.Width, Height: w.Height}
	}
	mines := make([]protocol.Mine, len(state.Mines))
	for i, m := range state.Mines {
		mines[i] = protocol.Mine{X: m.X, Y: m.Y, Size: m.Size}
	}
	var status *protocol.StatusEffect
	if state.Status != nil {
		status = &protocol.StatusEffect{Type: state.Status.Type, Remaining: state.Status.Remaining}
	}

	return protocol.GameState{
		RoomName:   state.RoomName,
		Mode:       state.Mode,
		Phase:      state.Phase,
		Countdown:  state.Countdown,
		Remaining:  state.Remaining,
		Message:    state.Message,
		BombHolder: state.BombHolder,
		BombTimer:  state.BombTimer,
		Players:    players,
		Fish:       protocol.FishState(state.Fish),
		Walls:      walls,
		Mines:      mines,
		PowerUp:    protocol.PowerUpState(state.PowerUp),
		Status:     status,
		WinnerID:   state.WinnerID,
		Golden:     state.Golden,
		TickIndex:  state.TickIndex,
		ServerTime: state.ServerTime,
	}
}

func toProtocolPlayerPatch(p playerPatch) protocol.PlayerPatch {
	protoPatch := protocol.PlayerPatch{ID: p.ID}
	protoPatch.Name = p.Name
	protoPatch.Ready = p.Ready
	protoPatch.Alive = p.Alive
	protoPatch.X = p.X
	protoPatch.Y = p.Y
	protoPatch.Size = p.Size
	protoPatch.Facing = p.Facing
	protoPatch.Moving = p.Moving
	protoPatch.WalkCycle = p.WalkCycle
	protoPatch.StepAccum = p.StepAccum
	protoPatch.Score = p.Score
	if len(p.Appearance) > 0 {
		appearance := stringifyAppearance(p.Appearance)
		if len(appearance) > 300 {
			appearance = appearance[:300]
		}
		protoPatch.Appearance = &appearance
	}
	return protoPatch
}

func toProtocolStatePatch(patch *statePatch) *protocol.StatePatch {
	if patch == nil {
		return nil
	}
	protoPatch := &protocol.StatePatch{}
	protoPatch.Mode = patch.Mode
	protoPatch.Phase = patch.Phase
	protoPatch.Countdown = patch.Countdown
	protoPatch.Remaining = patch.Remaining
	protoPatch.Message = patch.Message
	protoPatch.BombHolder = patch.BombHolder
	protoPatch.BombTimer = patch.BombTimer
	protoPatch.WinnerID = patch.WinnerID
	protoPatch.Golden = patch.Golden
	if patch.Status != nil {
		protoPatch.Status = &protocol.StatusEffect{Type: patch.Status.Type, Remaining: patch.Status.Remaining}
	}
	if patch.Fish != nil {
		fish := protocol.FishState(*patch.Fish)
		protoPatch.Fish = &fish
	}
	if patch.PowerUp != nil {
		power := protocol.PowerUpState(*patch.PowerUp)
		protoPatch.PowerUp = &power
	}
	if len(patch.Walls) > 0 {
		walls := make([]protocol.Wall, len(patch.Walls))
		for i, w := range patch.Walls {
			walls[i] = protocol.Wall{X: w.X, Y: w.Y, Width: w.Width, Height: w.Height}
		}
		protoPatch.Walls = walls
	}
	if len(patch.Mines) > 0 {
		mines := make([]protocol.Mine, len(patch.Mines))
		for i, m := range patch.Mines {
			mines[i] = protocol.Mine{X: m.X, Y: m.Y, Size: m.Size}
		}
		protoPatch.Mines = mines
	}
	if len(patch.Players) > 0 {
		players := make([]protocol.PlayerPatch, len(patch.Players))
		for i, p := range patch.Players {
			players[i] = toProtocolPlayerPatch(p)
		}
		protoPatch.Players = players
	}
	if len(patch.RemovedPlayers) > 0 {
		protoPatch.RemovedPlayers = append(protoPatch.RemovedPlayers, patch.RemovedPlayers...)
	}
	return protoPatch
}

func (r *room) snapshotLocked() gameState {
	stateCopy := r.state
	players := make([]*playerState, 0, len(r.players))
	for _, p := range r.players {
		cp := *p
		players = append(players, &cp)
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
	return p == nil || (p.Mode == nil && p.Phase == nil && p.Countdown == nil && p.Remaining == nil && p.Message == nil && p.BombHolder == nil && p.BombTimer == nil && p.WinnerID == nil && p.Status == nil && p.Fish == nil && p.PowerUp == nil && len(p.Walls) == 0 && len(p.Mines) == 0 && len(p.Players) == 0 && len(p.RemovedPlayers) == 0 && p.Golden == nil)
}

func buildStatePatch(previous, current gameState) *statePatch {
	patch := &statePatch{}

	if previous.Mode != current.Mode {
		patch.Mode = stringPtr(current.Mode)
	}
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
	if previous.BombHolder != current.BombHolder {
		patch.BombHolder = stringPtr(current.BombHolder)
	}
	if floatChanged(previous.BombTimer, current.BombTimer) {
		patch.BombTimer = floatPtr(current.BombTimer)
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
	bombSlowTimers     map[string]float64
	lastBombPassFrom   string
	lastBombPassTo     string
	lastBombPassAt     time.Time
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

func normalizeMode(mode string) string {
	switch mode {
	case "bomb-pass":
		return mode
	default:
		return "classic"
	}
}

func (s *server) getOrCreateRoom(name, mode string) *room {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.rooms[name]; ok {
		return existing
	}
	normalizedMode := normalizeMode(mode)
	world := worldSize
	if normalizedMode == "bomb-pass" {
		world = worldSize * bombWorldScale
	}
	r := &room{
		name:             name,
		players:          make(map[string]*playerState),
		inputs:           make(map[string]vector),
		connections:      make(map[*websocket.Conn]string),
		disconnectTimers: make(map[string]*time.Timer),
		cancel:           make(chan struct{}),
		server:           s,
		bombSlowTimers:   make(map[string]float64),
	}
	r.state = gameState{
		RoomName:   name,
		Mode:       normalizedMode,
		Phase:      "lobby",
		Fish:       fishState{X: world / 2, Y: world / 2, Size: fishSize, Alive: false, Type: "normal", Direction: 1},
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
			"mode":        r.state.Mode,
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
	mode := r.URL.Query().Get("mode")
	if roomName == "" || playerID == "" {
		http.Error(w, "room and playerId required", http.StatusBadRequest)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}
	normalizedMode := normalizeMode(mode)
	rInstance := s.getOrCreateRoom(roomName, normalizedMode)
	if rInstance.state.Mode != normalizedMode {
		conn.WriteJSON(wsMessage{Type: "error", Error: "Эта комната создана в другом режиме."})
		conn.Close()
		return
	}
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

	var timer *time.Timer
	timer = time.AfterFunc(reconnectGrace, func() {
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
		world := r.currentWorldSize()
		player = &playerState{ID: id, Name: fallbackName(name), Size: catSize, X: world / 2, Y: world / 2, Facing: 1}
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
	data := protocol.EncodeState(toProtocolGameState(stateCopy))
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
		r.updateStatusEffectLocked()
		if r.isBombMode() {
			r.updatePlayersLocked()
			r.updateBombPassLocked()
		} else {
			r.state.Remaining -= tickRate.Seconds()
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
	r.bombSlowTimers = make(map[string]float64)
	r.resetBombPassHistoryLocked()
	if r.isBombMode() {
		r.state.Fish = fishState{Size: fishSize, Alive: false, Type: "normal", Direction: 1}
		r.state.PowerUp.Active = false
		r.state.Mines = nil
		r.buildBombPassArenaLocked()
	} else {
		r.spawnFishLocked()
	}
	for _, p := range r.players {
		p.Alive = true
		p.Score = 0
	}
	r.state.BombHolder = ""
	r.state.BombTimer = bombTimerDuration
	if r.isBombMode() {
		r.assignBombToRandomAliveLocked(true)
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
	if r.isBombMode() {
		for _, p := range r.players {
			if p.Alive {
				return p.ID
			}
		}
		return ""
	}
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
	world := r.currentWorldSize()
	for id, p := range r.players {
		if !p.Alive {
			continue
		}
		input := r.inputs[id]
		speed := catSpeed * tickRate.Seconds() * speedMultiplier
		if r.isBombMode() {
			speed *= r.getBombSpeedMultiplierLocked(id)
		}
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
		p.X = clampFloat(p.X, p.Size/2, world-p.Size/2)
		p.Y = clampFloat(p.Y, p.Size/2, world-p.Size/2)

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

	if r.isBombMode() {
		r.resolvePlayerCollisionsLocked()
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

func (r *room) alivePlayersLocked() []*playerState {
	players := make([]*playerState, 0, len(r.players))
	for _, p := range r.players {
		if p.Alive {
			players = append(players, p)
		}
	}
	return players
}

func (r *room) isBombMode() bool {
	return r.state.Mode == "bomb-pass"
}

func (r *room) currentWorldSize() float64 {
	if r.isBombMode() {
		return worldSize * bombWorldScale
	}
	return worldSize
}

func (r *room) applyBombSlowdownLocked(playerID string) {
	if !r.isBombMode() {
		return
	}
	r.bombSlowTimers[playerID] = bombSlowDuration
}

func (r *room) tickBombSlowdownsLocked() {
	if len(r.bombSlowTimers) == 0 {
		return
	}
	for id, remaining := range r.bombSlowTimers {
		remaining -= tickRate.Seconds()
		if remaining <= 0 {
			delete(r.bombSlowTimers, id)
		} else {
			r.bombSlowTimers[id] = remaining
		}
	}
}

func (r *room) getBombSpeedMultiplierLocked(playerID string) float64 {
	remaining, ok := r.bombSlowTimers[playerID]
	if ok && remaining > 0 {
		return bombSlowFactor
	}
	return 1
}

func (r *room) resetBombPassHistoryLocked() {
	r.lastBombPassFrom = ""
	r.lastBombPassTo = ""
	r.lastBombPassAt = time.Time{}
}

func (r *room) resolvePlayerCollisionsLocked() {
	players := r.alivePlayersLocked()
	world := r.currentWorldSize()
	for i := 0; i < len(players); i++ {
		for j := i + 1; j < len(players); j++ {
			a := players[i]
			b := players[j]
			dx := b.X - a.X
			dy := b.Y - a.Y
			dist := math.Hypot(dx, dy)
			minDist := (a.Size + b.Size) / 2
			if minDist <= 0 {
				continue
			}
			if dist >= minDist {
				continue
			}
			if dist == 0 {
				dx = 1
				dy = 0
				dist = 1
			}
			nx := dx / dist
			ny := dy / dist
			overlap := minDist - dist
			push := overlap / 2
			a.X -= nx * push
			a.Y -= ny * push
			b.X += nx * push
			b.Y += ny * push
		}
	}

	for _, p := range players {
		resolveEntityWallCollisions(p, r.state.Walls)
		p.X = clampFloat(p.X, p.Size/2, world-p.Size/2)
		p.Y = clampFloat(p.Y, p.Size/2, world-p.Size/2)
	}
}

func (r *room) assignBombToRandomAliveLocked(resetTimer bool) {
	alive := r.alivePlayersLocked()
	if len(alive) == 0 {
		r.state.BombHolder = ""
		return
	}
	r.resetBombPassHistoryLocked()
	picked := alive[rand.Intn(len(alive))]
	r.state.BombHolder = picked.ID
	if resetTimer {
		r.state.BombTimer = bombTimerDuration
	}
	r.applyBombSlowdownLocked(picked.ID)
	r.state.Message = fmt.Sprintf("Бомба у %s!", fallbackName(picked.Name))
}

func (r *room) handleBombTransferLocked() {
	holder, ok := r.players[r.state.BombHolder]
	if !ok || holder == nil || !holder.Alive {
		return
	}
	for id, p := range r.players {
		if id == holder.ID || !p.Alive {
			continue
		}
		dist := math.Hypot(holder.X-p.X, holder.Y-p.Y)
		if dist <= (holder.Size+p.Size)/2 {
			recentBackTransfer :=
				r.lastBombPassFrom != "" &&
					r.lastBombPassTo != "" &&
					time.Since(r.lastBombPassAt) < time.Second &&
					holder.ID == r.lastBombPassTo &&
					p.ID == r.lastBombPassFrom
			if recentBackTransfer {
				continue
			}
			r.state.BombHolder = p.ID
			r.state.BombTimer = math.Max(r.state.BombTimer, 0) + bombTimerBonus
			r.applyBombSlowdownLocked(p.ID)
			r.lastBombPassFrom = holder.ID
			r.lastBombPassTo = p.ID
			r.lastBombPassAt = time.Now()
			r.state.Message = fmt.Sprintf("%s передал бомбу %s", fallbackName(holder.Name), fallbackName(p.Name))
			return
		}
	}
}

func (r *room) updateBombPassLocked() {
	if r.countAlivePlayersLocked() <= 1 {
		r.endRoundLocked("Раунд завершён")
		return
	}
	r.tickBombSlowdownsLocked()
	holder, exists := r.players[r.state.BombHolder]
	if r.state.BombHolder == "" || !exists || holder == nil || !holder.Alive {
		r.assignBombToRandomAliveLocked(true)
	}
	r.handleBombTransferLocked()
	r.state.BombTimer -= tickRate.Seconds()
	if r.state.BombTimer < 0 {
		r.state.BombTimer = 0
	}
	r.state.Remaining = r.state.BombTimer

	holder, ok := r.players[r.state.BombHolder]
	if !ok || holder == nil || !holder.Alive {
		r.assignBombToRandomAliveLocked(true)
	}
	holder = r.players[r.state.BombHolder]
	if holder != nil && holder.Alive && r.state.BombTimer <= 0 {
		holder.Alive = false
		holder.Moving = false
		r.state.Message = fmt.Sprintf("%s не успел избавиться от бомбы!", fallbackName(holder.Name))
		r.state.BombHolder = ""
		r.state.BombTimer = bombTimerDuration
	}
	if r.countAlivePlayersLocked() <= 1 {
		r.endRoundLocked("Выжил только один котик!")
		return
	}
	if r.state.BombHolder == "" {
		r.assignBombToRandomAliveLocked(true)
	}
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
	world := r.currentWorldSize()
	r.state.Fish.X = clampFloat(nextX, r.state.Fish.Size/2, world-r.state.Fish.Size/2)

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
	world := r.currentWorldSize()
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
		catCells = append(catCells, positionToGridCell(p.X, p.Y, world))
	}

	placed := false
	for attempt := 0; attempt < 200; attempt++ {
		x := margin + rand.Float64()*(world-margin*2)
		y := margin + rand.Float64()*(world-margin*2)
		fishCell := positionToGridCell(x, y, world)
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
		fish.X = world / 2
		fish.Y = world / 2
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
	protoState := toProtocolGameState(stateCopy)
	if previous == nil {
		data = protocol.EncodeState(protoState)
	} else if patch := buildStatePatch(*previous, stateCopy); patch != nil {
		data = protocol.EncodePatch(toProtocolStatePatch(patch), protoState.ServerTime, protoState.TickIndex)
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
	world := r.currentWorldSize()
	for attempt := 0; attempt < 40; attempt++ {
		x := margin + rand.Float64()*(world-margin*2)
		y := margin + rand.Float64()*(world-margin*2)
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
	world := r.currentWorldSize()
	for _, p := range r.players {
		resolveEntityWallCollisions(p, r.state.Walls)
		p.X = clampFloat(p.X, p.Size/2, world-p.Size/2)
		p.Y = clampFloat(p.Y, p.Size/2, world-p.Size/2)
	}
}

func (r *room) handlePowerUpAfterWallChangeLocked() {
	if r.state.PowerUp.Active && circleIntersectsAnyWall(r.state.PowerUp.X, r.state.PowerUp.Y, r.state.PowerUp.Size/2+2, r.state.Walls) {
		r.clearPowerUpLocked()
	}
}

func (r *room) buildBombPassArenaLocked() {
	world := r.currentWorldSize()
	layout := buildBoundaryWalls(world)

	players := make([]*playerState, 0, len(r.players))
	for _, p := range r.players {
		players = append(players, p)
	}
	if len(players) == 0 {
		r.state.Walls = layout
		return
	}

	catCells := make([]gridCell, 0, len(players))
	for _, p := range players {
		catCells = append(catCells, positionToGridCell(p.X, p.Y, world))
	}

	anchor := gridCell{Row: gridSize / 2, Col: gridSize / 2}
	if containsCell(catCells, anchor) {
		anchor = gridCell{Row: (gridSize / 2) - 1, Col: gridSize / 2}
	}

	if candidate := r.generateWallsLayoutForPlayers(catCells, anchor); candidate != nil {
		layout = append(layout, candidate...)
	}

	r.state.Walls = layout
	r.handlePowerUpAfterWallChangeLocked()
	r.resolvePlayersAfterWallChangeLocked()
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
		candidateWalls := convertSegmentsToWalls(segments, r.currentWorldSize())
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

	world := r.currentWorldSize()
	for len(result) < mineCount && attempts < 200 {
		attempts++
		x := margin + rand.Float64()*(world-margin*2)
		y := margin + rand.Float64()*(world-margin*2)
		if !r.isMinePositionValidLocked(x, y, radius, result) {
			continue
		}
		result = append(result, mine{X: x, Y: y, Size: mineSize})
	}
	return result
}

func (r *room) isMinePositionValidLocked(x, y, radius float64, existing []mine) bool {
	safeRadius := radius + mineMinDistance
	world := r.currentWorldSize()
	if x-safeRadius < 0 || y-safeRadius < 0 || x+safeRadius > world || y+safeRadius > world {
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

func positionToGridCell(x, y, world float64) gridCell {
	cellSize := world / gridSize
	col := clampGridIndex(int(math.Floor(x / cellSize)))
	row := clampGridIndex(int(math.Floor(y / cellSize)))
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

func convertSegmentsToWalls(segments []wallSegment, world float64) []wall {
	walls := make([]wall, 0, len(segments))
	cellSize := world / gridSize
	thickness := cellSize * wallThicknessRate
	for _, seg := range segments {
		if seg.Orientation == "horizontal" {
			walls = append(walls, wall{
				X:      float64(seg.Col) * cellSize,
				Y:      float64(seg.Row)*cellSize + (cellSize-thickness)/2,
				Width:  float64(seg.Length) * cellSize,
				Height: thickness,
			})
		} else {
			walls = append(walls, wall{
				X:      float64(seg.Col)*cellSize + (cellSize-thickness)/2,
				Y:      float64(seg.Row) * cellSize,
				Width:  thickness,
				Height: float64(seg.Length) * cellSize,
			})
		}
	}
	return walls
}

func buildBoundaryWalls(world float64) []wall {
	thickness := (world / gridSize) * wallThicknessRate
	return []wall{
		{X: 0, Y: 0, Width: world, Height: thickness},
		{X: 0, Y: world - thickness, Width: world, Height: thickness},
		{X: 0, Y: 0, Width: thickness, Height: world},
		{X: world - thickness, Y: 0, Width: thickness, Height: world},
	}
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
	world := r.currentWorldSize()
	if x-radius < 0 || x+radius > world {
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
