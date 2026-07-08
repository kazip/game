package main

import (
	"encoding/json"
	"math"
	"time"
)

// ─── Persistent storage (forward/backward compatible) ───────────────────────
// All durable data lives in a single versioned JSON envelope on a persistent
// volume (see resolveDataPath / persistLocked). Compatibility rules, enforced by
// the helpers below:
//   - BACKWARD compat — new code reads old files: bump storageVersion and add a
//     migration block in migrateEnvelope; old files (Version 0, no field) migrate
//     up. Missing fields simply decode to their zero value.
//   - FORWARD compat — OLD code reads NEW files without data loss: unknown keys
//     are captured into an `Extra` bag on load and re-emitted on save, both at
//     the document level (storeEnvelope) and per record (catProfile,
//     playerRating). NEVER remove or repurpose an existing JSON field — schema
//     changes must be additive only.
const storageVersion = 1

// foldExtra merges preserved unknown keys into an already-marshalled object,
// without overwriting keys the object itself produced. Round-trips fields a
// newer server wrote through an older server's load→save cycle.
func foldExtra(base []byte, extra map[string]json.RawMessage) ([]byte, error) {
	if len(extra) == 0 {
		return base, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(base, &m); err != nil {
		return base, err
	}
	for k, v := range extra {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	return json.Marshal(m)
}

// splitExtra returns the keys of data that are NOT in `known` — the unknown
// fields to preserve for forward compatibility (nil if there are none).
func splitExtra(data []byte, known ...string) map[string]json.RawMessage {
	var m map[string]json.RawMessage
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	for _, k := range known {
		delete(m, k)
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// storeEnvelope is the whole on-disk document. Extra preserves unknown top-level
// collections a future server might add (e.g. "achievements").
type storeEnvelope struct {
	Version       int                        `json:"version"`
	Cats          map[string]catProfile      `json:"cats"`
	Scores        []scoreEntry               `json:"scores"`
	Reports       []reportEntry              `json:"reports"`
	Ratings       map[string]*playerRating   `json:"ratings"`
	RatingHistory []ratingHistoryEntry       `json:"ratingHistory"`
	Extra         map[string]json.RawMessage `json:"-"`
}

func (e storeEnvelope) MarshalJSON() ([]byte, error) {
	type alias storeEnvelope // shed methods to avoid recursion; drops Extra (json:"-")
	b, err := json.Marshal(alias(e))
	if err != nil {
		return nil, err
	}
	return foldExtra(b, e.Extra)
}

func (e *storeEnvelope) UnmarshalJSON(data []byte) error {
	type alias storeEnvelope
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*e = storeEnvelope(a)
	e.Extra = splitExtra(data, "version", "cats", "scores", "reports", "ratings", "ratingHistory")
	return nil
}

// migrateEnvelope upgrades an older on-disk schema to storageVersion in place.
// Add one block per version bump; keep them additive so older servers can still
// read the result (see the compatibility rules above).
func migrateEnvelope(e *storeEnvelope) {
	if e.Version < 1 {
		// v0 (original, unversioned) → v1: the {cats,scores,reports,ratings}
		// shape already maps directly; just stamp the version.
		e.Version = 1
	}
	// if e.Version < 2 { …migrate…; e.Version = 2 }
}

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
	resultsWindowSecs     = 30.0 // how long the end-of-round results table stays up

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
	Live      bool    `json:"live,omitempty"` // a game of this type is already running: even 1 cat can join (as spectator → next round)
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
	PlayerID   string                     `json:"playerId"`
	Name       string                     `json:"name"`
	Appearance catAppearance              `json:"appearance"`
	Extra      map[string]json.RawMessage `json:"-"` // preserve unknown fields (fwd compat)
}

func (p catProfile) MarshalJSON() ([]byte, error) {
	type alias catProfile
	b, err := json.Marshal(alias(p))
	if err != nil {
		return nil, err
	}
	return foldExtra(b, p.Extra)
}

func (p *catProfile) UnmarshalJSON(data []byte) error {
	type alias catProfile
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*p = catProfile(a)
	p.Extra = splitExtra(data, "playerId", "name", "appearance")
	return nil
}

type scoreEntry struct {
	PlayerID  string    `json:"playerId"`
	Name      string    `json:"name"`
	Score     int       `json:"score"`
	Mode      string    `json:"mode,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ─── Player rating / ранги (звания) ─────────────────────────────────────────
// Two ratings per player (see the RatingService — applyMatchResultLocked):
//   - HiddenMMR  — a real Elo (K=32) used for matchmaking & opponent strength.
//     Rises AND falls; NEVER shown to the player.
//   - PublicRating — shown to the player and only ever GROWS. Every match awards
//     a small amount by result × opponent strength (win > any loss;
//     loss-to-strong > loss-to-weak), so progress never goes back.
//
// Ранги (звания) are thresholds on PublicRating; a rank, once reached, is never
// lost (MaxRank). The ladder is mirrored VERBATIM in the Godot client
// (Main.gd RANK_TIERS) — keep them in sync.
type ratingTier struct {
	Min   int    // public rating needed to reach this звание
	Title string // Russian label shown near the nickname
}

var ratingTiers = []ratingTier{
	{0, "Котёнок"},
	{150, "Дворовый кот"},
	{500, "Уличный боец"},
	{1200, "Благородный кот"},
	{2500, "Кот-рыцарь"},
	{5000, "Барон"},
	{9000, "Граф"},
	{15000, "Герцог"},
	{25000, "Кот-император"},
}

// Rating model constants.
const (
	mmrDefault       = 1000 // starting hidden MMR
	mmrK             = 32   // Elo K-factor
	mmrEqualBand     = 200  // ±band (in MMR) that counts as an "equal" opponent
	ratingHistoryMax = 5000 // cap on retained rating-history entries
)

// tierForRating maps a public rating to its ladder index (0..len-1).
func tierForRating(public int) int {
	idx := 0
	for i, t := range ratingTiers {
		if public >= t.Min {
			idx = i
		} else {
			break
		}
	}
	return idx
}

// tierIndexOfName is the reverse lookup (0 if the name is unknown).
func tierIndexOfName(name string) int {
	for i, t := range ratingTiers {
		if t.Title == name {
			return i
		}
	}
	return 0
}

// eloExpected is the probability `player` beats `opponent` given their MMRs.
func eloExpected(playerMMR, oppMMR int) float64 {
	return 1.0 / (1.0 + math.Pow(10, float64(oppMMR-playerMMR)/400.0))
}

// mmrDelta is the hidden-Elo change: K*(result - expected). Beating a stronger
// opponent gains a lot; losing to a weaker one loses a lot.
func mmrDelta(playerMMR, oppMMR int, win bool) int {
	result := 0.0
	if win {
		result = 1.0
	}
	return int(math.Round(mmrK * (result - eloExpected(playerMMR, oppMMR))))
}

// oppStrength categorises the opponent by hidden MMR: -1 weaker, 0 equal, +1 stronger.
func oppStrength(playerMMR, oppMMR int) int {
	switch {
	case oppMMR <= playerMMR-mmrEqualBand:
		return -1
	case oppMMR >= playerMMR+mmrEqualBand:
		return 1
	default:
		return 0
	}
}

// publicPoints is the ONLY-grows public reward for a match. Any win beats any
// loss; a loss to a stronger opponent still pays more than a loss to a weaker one.
func publicPoints(playerMMR, oppMMR int, win bool) int {
	switch oppStrength(playerMMR, oppMMR) {
	case -1: // weaker opponent
		if win {
			return 8
		}
		return 1
	case 1: // stronger opponent
		if win {
			return 30
		}
		return 6
	default: // equal opponent
		if win {
			return 15
		}
		return 3
	}
}

// playerRating is the persistent, cross-session glory-point record for one
// player id. Stored in data.json under "ratings" and keyed by playerId.
type playerRating struct {
	PlayerID     string                     `json:"playerId"`
	Name         string                     `json:"name"`
	HiddenMMR    int                        `json:"hiddenMmr"`    // hidden Elo; rises & falls; NEVER sent to clients
	PublicRating int                        `json:"publicRating"` // shown to players; only ever grows
	Rank         string                     `json:"rank"`         // current звание name
	MaxRank      string                     `json:"maxRank"`      // highest звание ever reached (never lowered)
	Overall      int                        `json:"overall"`      // legacy mirror of PublicRating (kept for old readers)
	ByMode       map[string]int             `json:"byMode"`
	Wins         int                        `json:"wins"`
	Games        int                        `json:"games"`
	UpdatedAt    time.Time                  `json:"updatedAt"`
	Extra        map[string]json.RawMessage `json:"-"` // preserve unknown fields (fwd compat)
}

// ensureRatingDefaults fills in defaults and migrates a legacy record (one that
// only had the old glory-point "overall") to the hidden-MMR / public-rating
// model, keeping PublicRating and the legacy Overall mirror in sync.
func ensureRatingDefaults(pr *playerRating) {
	if pr.HiddenMMR == 0 {
		pr.HiddenMMR = mmrDefault
	}
	if pr.PublicRating == 0 && pr.Overall > 0 {
		pr.PublicRating = pr.Overall // carry legacy glory points forward
	}
	pr.Overall = pr.PublicRating // keep the legacy mirror aligned
	if pr.ByMode == nil {
		pr.ByMode = map[string]int{}
	}
	name := ratingTiers[tierForRating(pr.PublicRating)].Title
	if pr.Rank == "" {
		pr.Rank = name
	}
	if pr.MaxRank == "" || tierIndexOfName(pr.MaxRank) < tierForRating(pr.PublicRating) {
		pr.MaxRank = name
	}
}

func (p playerRating) MarshalJSON() ([]byte, error) {
	type alias playerRating
	b, err := json.Marshal(alias(p))
	if err != nil {
		return nil, err
	}
	return foldExtra(b, p.Extra)
}

func (p *playerRating) UnmarshalJSON(data []byte) error {
	type alias playerRating
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*p = playerRating(a)
	p.Extra = splitExtra(data, "playerId", "name", "hiddenMmr", "publicRating",
		"rank", "maxRank", "overall", "byMode", "wins", "games", "updatedAt")
	return nil
}

// ratingHistoryEntry records one match's rating change for analytics/debugging.
type ratingHistoryEntry struct {
	PlayerID          string    `json:"playerId"`
	OpponentID        string    `json:"opponentId"` // "field" — the FFA lobby average
	MatchID           string    `json:"matchId"`
	Mode              string    `json:"mode"`
	Result            string    `json:"result"` // "win" | "loss"
	OldHiddenMMR      int       `json:"oldHiddenMmr"`
	NewHiddenMMR      int       `json:"newHiddenMmr"`
	HiddenMMRDelta    int       `json:"hiddenMmrDelta"`
	OldPublicRating   int       `json:"oldPublicRating"`
	NewPublicRating   int       `json:"newPublicRating"`
	PublicRatingDelta int       `json:"publicRatingDelta"`
	OldRank           string    `json:"oldRank"`
	NewRank           string    `json:"newRank"`
	CreatedAt         time.Time `json:"createdAt"`
}

type playerState struct {
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
	Rating     int     `json:"rating"`     // public rating (only grows); cross-session
	Tier       int     `json:"tier"`       // rank-ladder index for Rating (звание)
	RatingGain int     `json:"ratingGain"` // public points earned in the just-finished match
	// Cumulative per-session stats for the end-of-round results screen. They
	// PERSIST across rounds within one room join and reset only on a fresh join
	// (re-entering from the hub) — deliberately NOT reset in beginRoundLocked.
	// They reach clients via the full snapshot forced at round end, so they need
	// no incremental-patch plumbing.
	Kills        int     `json:"kills"`
	Deaths       int     `json:"deaths"`
	FishCaught   int     `json:"fishCaught"`
	BombCarries  int     `json:"bombCarries"`
	BombPasses   int     `json:"bombPasses"`
	BombHoldTime float64 `json:"bombHoldTime"`
	RoundWins    int     `json:"roundWins"`
	// This-round-only stats (reset every round in beginRoundLocked). They let the
	// results screen separate "этот раунд" from the cumulative "всего за игру".
	RoundKills        int           `json:"roundKills"`
	RoundDeaths       int           `json:"roundDeaths"`
	RoundFish         int           `json:"roundFish"`
	RoundBombCarries  int           `json:"roundBombCarries"`
	RoundBombPasses   int           `json:"roundBombPasses"`
	RoundBombHoldTime float64       `json:"roundBombHoldTime"`
	Health            int           `json:"health"`
	Weapon            string        `json:"weapon,omitempty"`
	Ammo              int           `json:"ammo"`
	Armor             int           `json:"armor"`
	Appearance        catAppearance `json:"appearance"`
	Disguise          string        `json:"disguise,omitempty"`
	Zombie            bool          `json:"zombie,omitempty"`
	// Joined mid-round: watches this round, auto-promoted to a player at the next
	// beginRoundLocked. Excluded from all gameplay + the ready gate while true.
	Spectator bool `json:"spectator,omitempty"`
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
	Rating     *int          `json:"rating,omitempty"`
	Tier       *int          `json:"tier,omitempty"`
	Health     *int          `json:"health,omitempty"`
	Weapon     *string       `json:"weapon,omitempty"`
	Ammo       *int          `json:"ammo,omitempty"`
	Armor      *int          `json:"armor,omitempty"`
	Appearance catAppearance `json:"appearance,omitempty"`
	Disguise   *string       `json:"disguise,omitempty"`
	Zombie     *bool         `json:"zombie,omitempty"`
	Spectator  *bool         `json:"spectator,omitempty"`
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
