package main

import "time"

const (
	worldSize             = 500.0
	bombWorldScale        = 5.0
	hideSeekWorldScale    = 3.0
	shooterWorldScale     = 3.0
	hideSeekHideDuration  = 60.0
	hideSeekSeekDuration  = 90.0
	hideSeekSeekerBoost   = 1.2
	shooterPrepDuration   = 30.0
	shooterRoundDuration  = 180.0
	shooterDamage         = 34
	shooterMaxHealth      = 100
	shooterShotLifetime   = 0.35
	shooterShotRange      = 220.0
	shooterRegenRate      = 4.0  // health points regenerated per second
	shooterRegenDelay     = 3.0  // seconds without damage before regen resumes
	shooterMedkitHeal     = 50   // health restored by a medkit
	shooterFistRange      = 52.0 // melee reach when out of ammo (very short)
	shooterFistDamage     = 16   // damage of a bare-paws hit
	shooterFistCooldown   = 0.5  // seconds between punches
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

	// ─── Hub / room-tree ────────────────────────────────────────────────
	hubMode       = "hub" // a free-roam social room, no game logic
	hubWorldScale = 3.0   // legacy square size (still used for currentWorldSize)
	// The hub is a 2:1 plaza matching the hub_background.png art (1774×887).
	// Portal positions below are placed on the circles painted in that image.
	hubWorldW      = 2000.0
	hubWorldH      = 1000.0
	hubMaxPlayers  = 50              // soft cap on a single hub
	defaultRoomMax = 12              // fallback capacity for a game room
	emptyRoomGrace = 5 * time.Minute // idle non-persistent rooms are GC'd after this
)

// Game room types offered by every hub (one pooled room of each at boot).
// These map 1:1 onto the existing play modes; "hub" is handled separately.
var gameRoomTypes = []string{"classic", "bomb-pass", "hide-and-seek", "shooters", "zombies"}

// Human-readable names used to label pooled rooms ("Стрелялки №1").
var modeDisplayNames = map[string]string{
	"classic":       "Классика",
	"bomb-pass":     "Горячая бомба",
	"hide-and-seek": "Прятки",
	"shooters":      "Стрелялки",
	"zombies":       "Зомби",
	hubMode:         "Хаб",
}

// Per-type player capacity for pooled rooms.
var roomTypeMax = map[string]int{
	"classic":       12,
	"bomb-pass":     16,
	"hide-and-seek": 12,
	"shooters":      12,
	"zombies":       12,
}

func displayNameForMode(mode string) string {
	if name, ok := modeDisplayNames[mode]; ok {
		return name
	}
	return mode
}

func capacityForType(roomType string) int {
	if max, ok := roomTypeMax[roomType]; ok {
		return max
	}
	return defaultRoomMax
}

// ─── Hub portals (matchmaking pads) ─────────────────────────────────────────
const (
	portalRadius        = 100.0 // world units; must match the client HUB_PORTALS radius
	portalMinPlayers    = 2     // cats required to arm a portal
	portalCountdownSecs = 15.0  // countdown once armed
	portalCooldownSecs  = 3.0   // after a launch, ignore the pad briefly
	minPlayersToStart   = 2     // a game room won't begin a round with fewer than this
)

// ─── Zombie mode ────────────────────────────────────────────────────────────
const (
	zombieWorldScale = 3.0  // arena size (== hide-and-seek)
	zombieRoundSecs  = 60.0 // one minute to infect everyone
	zombieTouchDist  = 44.0 // world units for a tag
	// Zombie speed scales DOWN with the horde size: a lone zombie is slightly
	// faster than survivors (1.03), each extra zombie shaves a little off (down
	// to a floor), so zombies must gang up to corner the last runners.
	zombieBaseSpeed    = 1.03 // multiplier for a single zombie
	zombieSlowPerExtra = 0.03 // reduction per zombie beyond the first
	zombieSlowFloor    = 0.75 // minimum speed multiplier
)

type hubPortal struct {
	Type string
	X, Y float64
}

// Positions must match the client's HUB_PORTALS. Placed on the coloured circles
// painted in hub_background.png (world = 2000×1000, 2:1). Each pair is the circle
// centre as a fraction of the image mapped into world units.
var hubPortals = []hubPortal{
	{"classic", 999, 124},       // top       — green
	{"shooters", 1457, 388},     // right      — yellow
	{"bomb-pass", 1282, 832},    // bottom-right — red
	{"hide-and-seek", 710, 832}, // bottom-left — blue
	{"zombies", 540, 389},       // left       — green
}

// Hub collision: a 250×125 row-major bitmask (1 = walkable) over the 2000×1000
// plaza, hand-authored (~/Загрузки/hub_mask_guide.png). Collision tests the cat
// centre against this mask as-drawn (no cat-radius erosion). Cats are
// confined to the stone plaza + the 5 portal pads + the paths between them.
const (
	hubWalkCols = 250
	hubWalkRows = 125
)

const hubWalkMaskB64 = "AAAAAAAAAAAAAD/+AD9wHgfwAB//AAAAAAAAAAAAAAAAAAAAAAAAAAAP/4AP/B/3/AAH/+AAAAAAAAAAAAAAAAAAAAAAAAAAB//AAf/////AAf/4AAAAAAAAAAAAAAAAAAAAAAAAAAH/8AP//////AD//gAAAAAAAAAAAAAAAAAAAAAAAAAA//4A//////+AP/+AAAAAAAAAAAAAAAAAAAAAAAAAAD//AD//////4A//4AAAAAAAAAAAAAAAAAAAAAAAAAAf/8AP//////wH//gAAAAAAAAAAAAAAAAAAAAAAAAAD//wA///////Af/+AAAAAAAAAAAAAAAAAAAAAAAAAAP//AB//////+B//8BAAAAAAAAAAAAAAAAAAAAAAAAH//+AP///////P//8MAAAAAAAAAAAAAAAAAAAAAAAB///4B////////////8AAAAAAAAAAAAAAAAAAAAAAG////5////////////j8AAAAAAAAAAAAAAAAAAAAAD/////////////////+H4AAAAAAAAAAAAAAAAAAAAAf/////////////////gPgAAAAPAAAAAAAAAAAAAAAP/////////////////+A+AAAAB+AAAAAAAAAAHAAAD//////////////////4H8AAAAP8AAAAAAAAAA+AAB///////////////////AfwAAAB/wAAAAAAAAAH8AD////H//////////////8AfgAAAf/AAAAAAAAAAf4D///4YP//////////////wA+AAAD/8AAAAAAAAAB/w////AAf/////////////+AH/AACf/wAAAAAAAAAH/n///4AAP////////////nwAf/gAf/+AAAAAAAAAAf/////gAAR///////////+PAAf/AD//4AAAAAAAAAB/////8AAAD///////////4cAAP8Af//AAAAAAAAAAH/////wAAAP///////////AwAA/////4AAAAAAAAAAf/////AAAA///////////4AAAD/////AAAAAAAAAAB/////+AAAB///////////gAAAP////4AAAAAAAAAAP/////8AAAH//////////8AAAA/////AAAAAAAAAAB//////wAAAf//////////wCAAD////4AAAAAAAAAAP//////gAAB///////////gIAB/////AAAAAAAAAAB///////4AAH//////////+BgH/////4AAAAAAAAAAH////////AAf////////////j/////+AAAAAAAAAAA/////////wD///////////////////wAAAAAAAAAAH8f///////8f//////////////////8AAAAAAAAAAAfgf///////////////////////////wAAAAAAAAAAH8A////////////////////////////gAAAAAAAAAB/gB////////////////////////////AAAAAAAAAAf+AD///////////////////////////8AAAAAAAAAAfwAH///////////////////////////4AAAAAAAAAAPAAf///////////////////////////gAAAAAAAAAAEAD////////////////////////////AAAAAAAAAAAAAP///////////////////////////8AAAAAAAAAAAAA////////////////////////////wAAAAAAAAAAAAH////////////////////////////AAAAAAAAAAAAAf///////////////////////////8AAAAAAAAAAAAD////////////////////////////wAAAAAAAAAAAAf////////////////////////////gAAAAAAAAAAAD////////////////////////////+AAAAAAAAAAAAf////////////////////////////8AAAAAAAAAAA//////////////////////////////4AAAAAAAAAAH//////////////////////////////4AAAAAAAAAA///////////////////////////////gAAAAAAAAAD///////////////////////////////4AAAAAAAAAH///////////////////////////////gAAAAAAAAAf//////////////////////////////+AAAAAAAAABzf////////////////////////////4gAAAAAAAAAOA/////////////////////////////AAAAAAAAAAA4D////////////////////////////4AAAAAAAAAADAH////////////////////////////gAAAAAAAAAAMAf///////////////////////////8AAAAAAAAAABwA////////////////////////////gAAAAAAAAAAHgD///////////////////////////8AAAAAAAAAAAeAP///////////////////////////wAAAAAAAAAAD4A////////////////////////////AAAAAAAAAAAPgH///////////////////////////8AAAAAAAAAAB+A///n/////////////////////5//4AAAAAAAAAAH8H4/8P////////////////////8D//4AAAAAAAAAAf/+B/g/////////////////////gP/BwAAAAAAAAAD//wD8D////////////////////+Af4HwAAAAAAAAAP//APwH////////////////////4Afgf4AAAAAAAAA//8AeAf////////////////////gB8B/8AAAAAAAAH//gA4B////////////////////+ADwH/8AAAAAAAA///AHgH////////////////////4APAf/4AAAAAAAP//8AfA/////////////////////gA+D//xwAADgAA///4D+D////////////////////+AH/////+AAOAAH///wf//////////////////////4A//////8AA8AH////////////////////////////gP/8f///wADwB/////////////////////////////n//AP//+AAPgf///////////////////////////////wAf//wAA///////+f////////////////////////8AAf/+AAD/////4/wf////////////////////////AAAP/4AAP////8B/Af///////////////////////wAAAH/gAA/////AD8B///////////////////////+AAAAH8AAD4P//wADwH///////////////////////4AAAAPgAAOAf/8AAAAP///////////////////////gAAAAAAAAwA//AAAAB///////////////////////8AAAAAAAAAAA/gAAAAH///////////////////////wAAAAAAAAAAAAAAAAAf///////////////////////gAAAAAAAAAAAAAAAAB///////////////////////+AAAAAAAAAAAAAAAAAD///////////////////////wAAAAAAAAAAAAAAAAAB//////////////////////uAAAAAAAAAAAAAAAAAAA/////////////////////8AAAAAAAAAAAAAAAAAAAB/////////////////////wAAAAAAAAAAAAAAAAAAAD////////////////////+AAAAAAAAAAAAAAAAAAAAP////////////////////4AAAAAAAAAAAAAAAAAAAA/////////////////////AAAAAAAAAAAAAAAAAAAAH////////////////////8AAAAAAAAAAAAAAAAAAAAf////////////////////wAAAAAAAAAAAAAAAAAAAB/////////////////////AAAAAAAAAAAAAAAAAAAAH////////////////////8AAAAAAAAAAAAAAAAAAAAf////////////////////wAAAAAAAAAAAAAAAAAAAB/////////////////////gAAAAAAAAAAAAAAAAAAAH////////////////////+AAAAAAAAAAAAAAAAAAAA/////////////////////4AAAAAAAAAAAAAAAAAAAH/////////////////////wAAAAAAAAAAAAAAAAAAA//////////////////////gAAAAAAAAAAAAAAAAAA///////////////////////4AAAAAAAAAAAAAAAAAH/////////8/////////////gAAAAAAAAAAAAAAAAA//////////B////////////+AAAAAAAAAAAAAAAAAH/////////8H//4/////////4AAAAAAAAAAAAAAAAAf/////////gf//D////////gAAAAAAAAAAAAAAAAAH/////////8B//4P///////+AAAAAAAAAAAAAAAAAA/+P///////wP//g////////wAAAAAAAAAAAAAAAAAH/gf///////A//+D////////AAAAAAAAAAAAAAAAAA/8B///////4D//4A///////8AAAAAAAAAAAAAAAAAP/AD///////gP//gB///////wAAAAAAAAAAAAAAAAB/wAH//////8Af/+AD///////AAAAAAAAAAAAAAAAAf8AAf//////wAf/wAH//////4AAAAAAAAAAAAAAAAD/gAB//+AP/+AAAAAAP/AB///AAAAAAAAAAAAAAAAAf+AAP//4AP/wAAAAAAP8AH//8AAAAAAAAAAAAAAAAD/4AB///gA/+AAAAAAABwAf//4AAAAAAAAAAAAAAAAf/AAP//+AD/gAAAAAAAAAD///wAAAAAAAAAAAAAAAD/8AB///4AB+AAAAAAAAAAP///wAAAAAAAAAAAAAAAP/gAP///gAAAAAAAAAAAAB//j/4AAAAAAAAAAAAAAB/8AB/8/+AAAAAAAAAAAAAD/8H/wAAAAAAAAAAAAAAP/gAP/wfwAAAAAAAAAAAAAPyAP/AAAAAAAAAAA=="

// portalStatus is broadcast in the hub's game state so clients can render the
// live pad occupancy and countdown on each portal sign.
type portalStatus struct {
	Type      string  `json:"type"`
	Count     int     `json:"count"`
	Countdown float64 `json:"countdown"` // 0 when idle
	Min       int     `json:"min"`
}

// Per-weapon ammo capacity and shooting cooldown (seconds) for the shooter mode.
var weaponAmmo = map[string]int{
	"pistol": 12, "blaster": 8, "laser": 24, "plasma": 5,
}
var weaponCooldown = map[string]float64{
	"pistol": 0.45, "blaster": 0.7, "laser": 0.15, "plasma": 1.1,
}

// Armor pickups: three levels granting an absorbing buffer of points.
var armorPoints = map[string]int{
	"armor1": 30, "armor2": 60, "armor3": 100,
}

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
	Mode      string    `json:"mode,omitempty"`
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
	Health     int           `json:"health"`
	Weapon     string        `json:"weapon,omitempty"`
	Ammo       int           `json:"ammo"`
	Armor      int           `json:"armor"`
	Appearance catAppearance `json:"appearance"`
	Disguise   string        `json:"disguise,omitempty"`
	Zombie     bool          `json:"zombie,omitempty"`
	// Internal shooter timers (not serialized).
	ShootCD    float64 `json:"-"`
	RegenDelay float64 `json:"-"`
	RegenAccum float64 `json:"-"`
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
	HidePhase  string         `json:"hidePhase,omitempty"`
	ShootPhase string         `json:"shootPhase,omitempty"`
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
	Shots      []shotEvent    `json:"shots,omitempty"`
	Status     *statusEffect  `json:"statusEffect"`
	WinnerID   string         `json:"winnerId"`
	Golden     bool           `json:"goldenChainActive"`
	Portals    []portalStatus `json:"portals,omitempty"`
	// Location art + dimensions (constant per room; sent in the full snapshot).
	// Background names a client-side backdrop image ("" = default grass/water).
	// WorldW/WorldH give the arena size so a location can be non-square (the hub).
	Background string  `json:"background,omitempty"`
	WorldW     float64 `json:"worldW,omitempty"`
	WorldH     float64 `json:"worldH,omitempty"`
	TickIndex  uint32  `json:"tickIndex"`
	ServerTime int64   `json:"serverTime"`
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

type shotEvent struct {
	ShooterID string  `json:"shooterId"`
	FromX     float64 `json:"fromX"`
	FromY     float64 `json:"fromY"`
	ToX       float64 `json:"toX"`
	ToY       float64 `json:"toY"`
	Weapon    string  `json:"weapon,omitempty"` // weapon used ("fist" for bare-paws) — for the right sfx/visual
	Remaining float64 `json:"remaining"`
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
	Shoot      *bool         `json:"shoot,omitempty"`
	Message    *chatMessage  `json:"message,omitempty"`
	Appearance catAppearance `json:"appearance,omitempty"`
	State      *gameState    `json:"state,omitempty"`
	Patch      *statePatch   `json:"patch,omitempty"`
	Full       bool          `json:"full,omitempty"`
	Error      string        `json:"error,omitempty"`
	Binary     *bool         `json:"binary,omitempty"`
	// Hub / room-tree browsing (client → server: list_rooms; server → client: rooms).
	HubID    string           `json:"hubId,omitempty"`
	RoomType string           `json:"roomType,omitempty"`
	RoomID   string           `json:"roomId,omitempty"`
	Rooms    []map[string]any `json:"rooms,omitempty"`
	// Player report (client → server).
	TargetID string `json:"targetId,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type reportEntry struct {
	RoomID     string    `json:"roomId"`
	ReporterID string    `json:"reporterId"`
	TargetID   string    `json:"targetId"`
	TargetName string    `json:"targetName"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
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
	Health     *int          `json:"health,omitempty"`
	Weapon     *string       `json:"weapon,omitempty"`
	Ammo       *int          `json:"ammo,omitempty"`
	Armor      *int          `json:"armor,omitempty"`
	Appearance catAppearance `json:"appearance,omitempty"`
	Disguise   *string       `json:"disguise,omitempty"`
	Zombie     *bool         `json:"zombie,omitempty"`
}

type statePatch struct {
	Mode           *string        `json:"mode,omitempty"`
	Phase          *string        `json:"phase,omitempty"`
	Countdown      *float64       `json:"countdown,omitempty"`
	Remaining      *float64       `json:"remaining,omitempty"`
	HidePhase      *string        `json:"hidePhase,omitempty"`
	ShootPhase     *string        `json:"shootPhase,omitempty"`
	Message        *string        `json:"message,omitempty"`
	SeekerID       *string        `json:"seekerId,omitempty"`
	BombHolder     *string        `json:"bombHolder,omitempty"`
	BombTimer      *float64       `json:"bombTimer,omitempty"`
	WinnerID       *string        `json:"winnerId,omitempty"`
	Golden         *bool          `json:"goldenChainActive,omitempty"`
	Shots          []shotEvent    `json:"shots,omitempty"`
	Status         *statusEffect  `json:"statusEffect,omitempty"`
	Fish           *fishState     `json:"fish,omitempty"`
	PowerUp        *powerUpState  `json:"powerUp,omitempty"`
	PowerUps       []powerUpState `json:"powerUps,omitempty"`
	Walls          []wall         `json:"walls,omitempty"`
	Mines          []mine         `json:"mines,omitempty"`
	Players        []playerPatch  `json:"players,omitempty"`
	RemovedPlayers []string       `json:"removedPlayers,omitempty"`
	Portals        []portalStatus `json:"portals,omitempty"`
}

type chatMessage struct {
	PlayerID string `json:"playerId"`
	Name     string `json:"name"`
	Text     string `json:"text"`
	At       int64  `json:"at"`
}
