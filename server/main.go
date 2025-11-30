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

	"github.com/gorilla/websocket"
)

const (
	worldSize         = 500.0
	catSpeed          = 180.0
	catSize           = 36.0
	fishSize          = 28.0
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
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Ready          bool          `json:"ready"`
	Alive          bool          `json:"alive"`
	X              float64       `json:"x"`
	Y              float64       `json:"y"`
	Size           float64       `json:"size"`
	Facing         int           `json:"facing"`
	Moving         bool          `json:"moving"`
	WalkCycle      float64       `json:"walkCycle"`
	StepAccum      float64       `json:"stepAccumulator"`
	Score          int           `json:"score"`
	Appearance     catAppearance `json:"appearance"`
	AppearanceJSON string        `json:"appearanceJson"`
}

type fishState struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Size      float64 `json:"size"`
	Alive     bool    `json:"alive"`
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
	Error      string        `json:"error,omitempty"`
}

type chatMessage struct {
	PlayerID string `json:"playerId"`
	Name     string `json:"name"`
	Text     string `json:"text"`
	At       int64  `json:"at"`
}

type room struct {
	name        string
	players     map[string]*playerState
	inputs      map[string]vector
	connections map[*websocket.Conn]string
	state       gameState
	server      *server
	mu          sync.Mutex
	cancel      chan struct{}
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
		name:        name,
		players:     make(map[string]*playerState),
		inputs:      make(map[string]vector),
		connections: make(map[*websocket.Conn]string),
		cancel:      make(chan struct{}),
		server:      s,
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
	r.mu.Unlock()

	r.sendFullState(conn)

	go func() {
		defer func() {
			conn.Close()
			r.dropConnection(conn)
		}()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
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
		player.AppearanceJSON = stringifyAppearance(appearance)
	}
}

func (r *room) dropConnection(conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.connections[conn]
	delete(r.connections, conn)
	conn.Close()
	if ok {
		delete(r.inputs, id)
		delete(r.players, id)
		if len(r.players) == 0 {
			r.state.Phase = "lobby"
			r.state.Message = "Ожидаем игроков"
		}
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
	stateCopy := r.state
	players := make([]*playerState, 0, len(r.players))
	for _, p := range r.players {
		copy := *p
		players = append(players, &copy)
	}
	stateCopy.Players = players
	r.mu.Unlock()

	data, _ := json.Marshal(wsMessage{Type: "state", State: &stateCopy})
	conn.WriteMessage(websocket.TextMessage, data)
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
		r.updateFishLocked()
		if r.state.Remaining <= 0 {
			r.endRoundLocked()
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

func (r *room) endRoundLocked() {
	r.state.Phase = "ended"
	r.state.Message = "Раунд завершён"
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

func (r *room) updateFishLocked() {
	if !r.state.Fish.Alive {
		r.spawnFishLocked()
		return
	}
	for _, p := range r.players {
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
	stateCopy := r.state
	players := make([]*playerState, 0, len(r.players))
	for _, p := range r.players {
		copy := *p
		players = append(players, &copy)
	}
	stateCopy.Players = players
	r.mu.Unlock()

	data, _ := json.Marshal(wsMessage{Type: "state", State: &stateCopy})
	r.mu.Lock()
	defer r.mu.Unlock()
	for conn := range r.connections {
		conn.WriteMessage(websocket.TextMessage, data)
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

func resolveEntityWallCollisions(entity *playerState, walls []wall) {
	radius := entity.Size/2 + 2
	for _, w := range walls {
		closestX := clampFloat(entity.X, w.X, w.X+w.Width)
		closestY := clampFloat(entity.Y, w.Y, w.Y+w.Height)
		dx := entity.X - closestX
		dy := entity.Y - closestY
		if dx*dx+dy*dy >= radius*radius {
			continue
		}
		if w.Width < w.Height {
			if entity.X < w.X+w.Width/2 {
				dx = -1
			} else {
				dx = 1
			}
			dy = 0
		} else {
			if entity.Y < w.Y+w.Height/2 {
				dy = -1
			} else {
				dy = 1
			}
			dx = 0
		}
		norm := math.Hypot(dx, dy)
		if norm == 0 {
			continue
		}
		dx /= norm
		dy /= norm
		entity.X = closestX + dx*radius
		entity.Y = closestY + dy*radius
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
	b, _ := json.Marshal(app)
	return string(b)
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
