package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
)

func floatChanged(a, b float64) bool { return math.Abs(a-b) > 0.0001 }

func stringPtr(v string) *string { return &v }

func boolPtr(v bool) *bool { return &v }

func floatPtr(v float64) *float64 { return &v }

func intPtr(v int) *int { return &v }

func roundFloat(v float64, decimals int) float64 {
	factor := math.Pow10(decimals)
	return math.Round(v*factor) / factor
}

func quantizeCoord(v float64) float64 { return math.Round(v) }

func quantizeWalkCycle(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return roundFloat(v*65535, 0) / 65535
}

func quantizeSeconds(v float64) float64 { return roundFloat(v, 3) }

func quantizePlayerForSend(p *playerState) {
	p.X = quantizeCoord(p.X)
	p.Y = quantizeCoord(p.Y)
	p.Size = quantizeCoord(p.Size)
	p.WalkCycle = quantizeWalkCycle(p.WalkCycle)
	p.StepAccum = roundFloat(p.StepAccum, 3)
}

func statusEqual(a, b *statusEffect) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Type == b.Type && a.PlayerID == b.PlayerID && !floatChanged(a.Remaining, b.Remaining)
}

func fishEqual(a, b fishState) bool {
	return !floatChanged(a.X, b.X) && !floatChanged(a.Y, b.Y) && !floatChanged(a.Size, b.Size) && a.Alive == b.Alive && a.Type == b.Type && a.Direction == b.Direction && a.Spawned == b.Spawned
}

func powerUpEqual(a, b powerUpState) bool {
	return !floatChanged(a.X, b.X) && !floatChanged(a.Y, b.Y) && !floatChanged(a.Size, b.Size) && !floatChanged(a.Remaining, b.Remaining) && a.Active == b.Active && a.Type == b.Type
}

func powerUpSlicesEqual(a, b []powerUpState) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !powerUpEqual(a[i], b[i]) {
			return false
		}
	}
	return true
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

func shotsEqual(a, b []shotEvent) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ShooterID != b[i].ShooterID {
			return false
		}
		if floatChanged(a[i].FromX, b[i].FromX) || floatChanged(a[i].FromY, b[i].FromY) || floatChanged(a[i].ToX, b[i].ToX) || floatChanged(a[i].ToY, b[i].ToY) {
			return false
		}
		if floatChanged(a[i].Remaining, b[i].Remaining) {
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

func clonePowerUps(src []powerUpState) []powerUpState {
	if len(src) == 0 {
		return nil
	}
	dst := make([]powerUpState, len(src))
	copy(dst, src)
	return dst
}

func cloneShots(src []shotEvent) []shotEvent {
	if len(src) == 0 {
		return nil
	}
	dst := make([]shotEvent, len(src))
	copy(dst, src)
	return dst
}

func cellKey(c gridCell) string { return fmt.Sprintf("%d,%d", c.Row, c.Col) }

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

func clampInt(v, min, max int) int {
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
