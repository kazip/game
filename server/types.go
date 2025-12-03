package main

import "time"

const (
	worldSize             = 500.0
	bombWorldScale        = 5.0
	hideSeekWorldScale    = 3.0
	catSpeed              = 180.0
	catSize               = 36.0
	fishSize              = 28.0
	fishSwimSpeed         = 36.0
	gridSize              = 10
	wallThicknessRate     = 0.6
	bombWallThicknessRate = 0.35
	gridCellSize          = worldSize / gridSize
	wallThickness         = gridCellSize * wallThicknessRate
	maxWallTotalLen       = 10
	bombMaxWallTotalLen   = 160
	bombMaxSegments       = 20
	maxSegments           = 2
	tickRate              = time.Second / 60
	broadcastRate         = time.Second / 15
	countdownDuration     = 3 * time.Second
	roundDuration         = 60 * time.Second
	fishCatchDistance     = 34.0
	maxMines              = 3
	mineSize              = 26.0
	mineMinDistance       = 25.0
	powerUpSize           = 34.0
	powerUpChance         = 0.05
	powerUpLifetime       = 5.0
	bombPowerUpLifetime   = 30.0
	powerUpDuration       = 30.0
	timeIncreaseLimit     = 15.0
	timeDecreaseLimit     = 5.0
	bombPowerUpInterval   = 5.0
	bombPowerUpMax        = 10
	bombTimerDuration     = 30.0
	bombSlowDuration      = 1.0
	bombSlowFactor        = 0.6
	bombTimerBonus        = 10.0
	dataFileName          = "data.json"
	reconnectGrace        = 10 * time.Second
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
	Mode       string         `json:"mode"`
	Phase      string         `json:"phase"`
	Countdown  float64        `json:"countdown"`
	Remaining  float64        `json:"remaining"`
	Message    string         `json:"message"`
	SeekerID   string         `json:"seekerId"`
	BombHolder string         `json:"bombHolder"`
	BombTimer  float64        `json:"bombTimer"`
	Players    []*playerState `json:"players"`
	Fish       fishState      `json:"fish"`
	Walls      []wall         `json:"walls"`
	Mines      []mine         `json:"mines"`
	PowerUp    powerUpState   `json:"powerUp"`
	PowerUps   []powerUpState `json:"powerUps,omitempty"`
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
	PlayerID  string  `json:"playerId,omitempty"`
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
	Mode           *string        `json:"mode,omitempty"`
	Phase          *string        `json:"phase,omitempty"`
	Countdown      *float64       `json:"countdown,omitempty"`
	Remaining      *float64       `json:"remaining,omitempty"`
	Message        *string        `json:"message,omitempty"`
	SeekerID       *string        `json:"seekerId,omitempty"`
	BombHolder     *string        `json:"bombHolder,omitempty"`
	BombTimer      *float64       `json:"bombTimer,omitempty"`
	WinnerID       *string        `json:"winnerId,omitempty"`
	Golden         *bool          `json:"goldenChainActive,omitempty"`
	Status         *statusEffect  `json:"statusEffect,omitempty"`
	Fish           *fishState     `json:"fish,omitempty"`
	PowerUp        *powerUpState  `json:"powerUp,omitempty"`
	PowerUps       []powerUpState `json:"powerUps,omitempty"`
	Walls          []wall         `json:"walls,omitempty"`
	Mines          []mine         `json:"mines,omitempty"`
	Players        []playerPatch  `json:"players,omitempty"`
	RemovedPlayers []string       `json:"removedPlayers,omitempty"`
}

type chatMessage struct {
	PlayerID string `json:"playerId"`
	Name     string `json:"name"`
	Text     string `json:"text"`
	At       int64  `json:"at"`
}
