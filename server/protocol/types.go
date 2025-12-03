package protocol

type Vector struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Wall struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Mine struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Size float64 `json:"size"`
}

type PowerUpState struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Size      float64 `json:"size"`
	Active    bool    `json:"active"`
	Remaining float64 `json:"remaining"`
	Type      string  `json:"type"`
}

type StatusEffect struct {
	Type      string  `json:"type"`
	Remaining float64 `json:"remaining"`
	PlayerID  string  `json:"playerId,omitempty"`
}

type FishState struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Size      float64 `json:"size"`
	Alive     bool    `json:"alive"`
	Spawned   bool    `json:"spawned,omitempty"`
	Type      string  `json:"type"`
	Direction int     `json:"direction"`
}

type PlayerState struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Ready      bool    `json:"ready"`
	Alive      bool    `json:"alive"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Size       float64 `json:"size"`
	Facing     int     `json:"facing"`
	Moving     bool    `json:"moving"`
	WalkCycle  float64 `json:"walkCycle"`
	StepAccum  float64 `json:"stepAccumulator"`
	Score      int     `json:"score"`
	Appearance string  `json:"appearance"`
	Disguise   string  `json:"disguise,omitempty"`
}

type GameState struct {
	RoomName   string         `json:"roomName"`
	Mode       string         `json:"mode"`
	Phase      string         `json:"phase"`
	Countdown  float64        `json:"countdown"`
	Remaining  float64        `json:"remaining"`
	HidePhase  string         `json:"hidePhase"`
	Message    string         `json:"message"`
	SeekerID   string         `json:"seekerId"`
	BombHolder string         `json:"bombHolder"`
	BombTimer  float64        `json:"bombTimer"`
	Players    []PlayerState  `json:"players"`
	Fish       FishState      `json:"fish"`
	Walls      []Wall         `json:"walls"`
	Mines      []Mine         `json:"mines"`
	PowerUp    PowerUpState   `json:"powerUp"`
	PowerUps   []PowerUpState `json:"powerUps"`
	Status     *StatusEffect  `json:"statusEffect"`
	WinnerID   string         `json:"winnerId"`
	Golden     bool           `json:"goldenChainActive"`
	TickIndex  uint32         `json:"tickIndex"`
	ServerTime int64          `json:"serverTime"`
}

type PlayerPatch struct {
	ID         string   `json:"id"`
	Name       *string  `json:"name,omitempty"`
	Ready      *bool    `json:"ready,omitempty"`
	Alive      *bool    `json:"alive,omitempty"`
	X          *float64 `json:"x,omitempty"`
	Y          *float64 `json:"y,omitempty"`
	Size       *float64 `json:"size,omitempty"`
	Facing     *int     `json:"facing,omitempty"`
	Moving     *bool    `json:"moving,omitempty"`
	WalkCycle  *float64 `json:"walkCycle,omitempty"`
	StepAccum  *float64 `json:"stepAccumulator,omitempty"`
	Score      *int     `json:"score,omitempty"`
	Appearance *string  `json:"appearance,omitempty"`
	Disguise   *string  `json:"disguise,omitempty"`
}

type StatePatch struct {
	Mode           *string        `json:"mode,omitempty"`
	Phase          *string        `json:"phase,omitempty"`
	Countdown      *float64       `json:"countdown,omitempty"`
	Remaining      *float64       `json:"remaining,omitempty"`
	HidePhase      *string        `json:"hidePhase,omitempty"`
	Message        *string        `json:"message,omitempty"`
	SeekerID       *string        `json:"seekerId,omitempty"`
	BombHolder     *string        `json:"bombHolder,omitempty"`
	BombTimer      *float64       `json:"bombTimer,omitempty"`
	WinnerID       *string        `json:"winnerId,omitempty"`
	Golden         *bool          `json:"goldenChainActive,omitempty"`
	Status         *StatusEffect  `json:"statusEffect,omitempty"`
	Fish           *FishState     `json:"fish,omitempty"`
	PowerUp        *PowerUpState  `json:"powerUp,omitempty"`
	PowerUps       []PowerUpState `json:"powerUps"`
	Walls          []Wall         `json:"walls,omitempty"`
	Mines          []Mine         `json:"mines,omitempty"`
	Players        []PlayerPatch  `json:"players,omitempty"`
	RemovedPlayers []string       `json:"removedPlayers,omitempty"`
}
