package main

import (
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"net/http"
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
	tickRate          = time.Second / 60
	broadcastRate     = time.Second / 15
	countdownDuration = 3 * time.Second
	roundDuration     = 60 * time.Second
	fishCatchDistance = 34.0
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
	WinnerID   string         `json:"winnerId"`
	ServerTime int64          `json:"serverTime"`
}

type wsMessage struct {
	Type       string        `json:"type"`
	Ready      *bool         `json:"ready,omitempty"`
	Vector     *vector       `json:"vector,omitempty"`
	Message    *chatMessage  `json:"message,omitempty"`
	Appearance catAppearance `json:"appearance,omitempty"`
	State      *gameState    `json:"state,omitempty"`
	Error      string        `json:"message,omitempty"`
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
	return &server{
		cats:     make(map[string]catProfile),
		rooms:    make(map[string]*room),
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	}
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
	player := r.ensurePlayer(playerID, playerName)
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
	for id, p := range r.players {
		input := r.inputs[id]
		speed := catSpeed * tickRate.Seconds()
		p.X = clampFloat(p.X+input.X*speed, p.Size/2, worldSize-p.Size/2)
		p.Y = clampFloat(p.Y+input.Y*speed, p.Size/2, worldSize-p.Size/2)
		p.Moving = math.Abs(input.X) > 0.01 || math.Abs(input.Y) > 0.01
		if p.Moving {
			p.Facing = 1
			if input.X < -0.01 {
				p.Facing = -1
			}
			p.StepAccum += tickRate.Seconds() * 4
			p.WalkCycle = math.Mod(p.StepAccum, 1)
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

func (r *room) spawnFishLocked() {
	r.state.Fish.Alive = true
	r.state.Fish.X = 40 + rand.Float64()*(worldSize-80)
	r.state.Fish.Y = 40 + rand.Float64()*(worldSize-80)
	r.state.Fish.Size = fishSize
	r.state.Fish.Type = "normal"
	r.state.Fish.Direction = 1
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

func main() {
	rand.Seed(time.Now().UnixNano())
	srv := newServer()

	http.HandleFunc("/api/cats/{id}", srv.handleCats)
	http.HandleFunc("/api/scores", srv.handleScores)
	http.HandleFunc("/api/rooms", srv.handleRooms)
	http.HandleFunc("/ws", srv.handleWS)

	addr := ":8080"
	log.Printf("Cat game server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
