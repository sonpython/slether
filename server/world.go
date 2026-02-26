package main

import (
	"sort"
	"sync"
)

// World holds all game state
type World struct {
	mu     sync.RWMutex
	Snakes map[string]*Snake
	Food   map[string]*Food
	Grid   *SpatialGrid
}

// NewWorld initializes the world with food
func NewWorld() *World {
	w := &World{
		Snakes: make(map[string]*Snake),
		Food:   make(map[string]*Food),
		Grid:   NewSpatialGrid(GridCellSize),
	}
	w.spawnInitialFood()
	return w
}

func (w *World) spawnInitialFood() {
	// Spawn ~70% as clusters, ~30% scattered
	clustered := int(float64(InitialFoodCount) * 0.7)
	scattered := InitialFoodCount - clustered

	for spawned := 0; spawned < clustered; {
		cluster := NewFoodCluster()
		for _, f := range cluster {
			if spawned >= clustered {
				break
			}
			w.Food[f.ID] = f
			spawned++
		}
	}
	for i := 0; i < scattered; i++ {
		f := NewFood()
		w.Food[f.ID] = f
	}
}

// AddSnake adds a new snake to the world (caller must hold mu.Lock)
func (w *World) AddSnake(s *Snake) {
	w.Snakes[s.ID] = s
}

// RemoveSnake removes a snake (caller must hold mu.Lock)
func (w *World) RemoveSnake(id string) {
	delete(w.Snakes, id)
}

// AddFood adds food items to the world (caller must hold mu.Lock)
func (w *World) AddFood(items []*Food) {
	for _, f := range items {
		w.Food[f.ID] = f
	}
}

// RemoveFood removes food by ID (caller must hold mu.Lock)
func (w *World) RemoveFood(id string) {
	delete(w.Food, id)
}

// RebuildGrid rebuilds the spatial grid from current state (caller must hold at least RLock)
func (w *World) RebuildGrid() {
	w.Grid.Clear()
	for _, f := range w.Food {
		w.Grid.InsertFood(f)
	}
	for _, s := range w.Snakes {
		if s.Alive {
			w.Grid.InsertSnakeBody(s)
		}
	}
}

// MaintainFoodCount spawns food up to TargetFoodCount (caller must hold mu.Lock).
// Moving food (level 10) is not counted against the normal food budget.
func (w *World) MaintainFoodCount() {
	normalCount := 0
	for _, f := range w.Food {
		if !f.IsMoving {
			normalCount++
		}
	}
	deficit := TargetFoodCount - normalCount
	if deficit <= 0 {
		return
	}
	spawn := deficit
	if spawn > FoodSpawnPerTick {
		spawn = FoodSpawnPerTick
	}
	// Spawn as cluster if deficit is large enough, otherwise individual
	for spawned := 0; spawned < spawn; {
		if spawn-spawned >= 5 {
			cluster := NewFoodCluster()
			for _, f := range cluster {
				if spawned >= spawn {
					break
				}
				w.Food[f.ID] = f
				spawned++
			}
		} else {
			f := NewFood()
			w.Food[f.ID] = f
			spawned++
		}
	}
}

// Leaderboard returns the top N snakes sorted by score
func (w *World) Leaderboard() []LeaderboardEntry {
	snakes := make([]*Snake, 0, len(w.Snakes))
	for _, s := range w.Snakes {
		if s.Alive {
			snakes = append(snakes, s)
		}
	}
	sort.Slice(snakes, func(i, j int) bool {
		return snakes[i].Score > snakes[j].Score
	})
	if len(snakes) > LeaderboardSize {
		snakes = snakes[:LeaderboardSize]
	}
	entries := make([]LeaderboardEntry, len(snakes))
	for i, s := range snakes {
		entries[i] = LeaderboardEntry{ID: s.ID, Name: s.Name, Score: s.Score}
	}
	return entries
}

// SnakesInViewport returns snake DTOs visible from a viewport centered on (cx,cy)
func (w *World) SnakesInViewport(cx, cy float64) []SnakeDTO {
	halfW := ViewportWidth/2 + ViewportBuffer
	halfH := ViewportHeight/2 + ViewportBuffer
	minX := cx - halfW
	maxX := cx + halfW
	minY := cy - halfH
	maxY := cy + halfH

	result := []SnakeDTO{}
	for _, s := range w.Snakes {
		if !s.Alive {
			continue
		}
		// Check if ANY segment is in viewport (not just head)
		visible := false
		for _, seg := range s.Segments {
			if seg.X >= minX && seg.X <= maxX && seg.Y >= minY && seg.Y <= maxY {
				visible = true
				break
			}
		}
		if visible {
			result = append(result, s.ToDTO(0))
		}
	}
	return result
}

// MinimapSnakes returns downsampled snake bodies for the minimap.
// Only includes snakes whose total body length is >= 1px on minimap.
// Segments are downsampled to keep wire size small.
func (w *World) MinimapSnakes() []MinimapSnake {
	const minimapDiameter = 160.0
	worldDiameter := WorldRadius * 2
	scale := minimapDiameter / worldDiameter
	// Minimum body length to appear on minimap (1px = ~131 world units = ~16 segments)
	minSegments := int(1.0 / (scale * SnakeSegmentSpacing))
	if minSegments < 2 {
		minSegments = 2
	}

	result := make([]MinimapSnake, 0)
	for _, s := range w.Snakes {
		if !s.Alive || len(s.Segments) < minSegments {
			continue
		}
		// Downsample: keep ~1 point per minimap pixel of body length
		step := minSegments
		segs := make([][2]float64, 0, len(s.Segments)/step+2)
		for i := 0; i < len(s.Segments); i += step {
			p := s.Segments[i]
			segs = append(segs, [2]float64{roundTo1(p.X), roundTo1(p.Y)})
		}
		// Always include last segment
		if len(s.Segments) > 0 {
			last := s.Segments[len(s.Segments)-1]
			lastPt := [2]float64{roundTo1(last.X), roundTo1(last.Y)}
			if len(segs) == 0 || segs[len(segs)-1] != lastPt {
				segs = append(segs, lastPt)
			}
		}
		if len(segs) >= 2 {
			result = append(result, MinimapSnake{
				Segments: segs,
				Color:    s.Color,
				Width:    roundTo1(s.Width),
			})
		}
	}
	return result
}

// FoodInViewport returns food DTOs visible from viewport centered on (cx,cy)
func (w *World) FoodInViewport(cx, cy float64) []FoodDTO {
	halfW := ViewportWidth/2 + ViewportBuffer
	halfH := ViewportHeight/2 + ViewportBuffer
	vx := cx - halfW
	vy := cy - halfH
	return w.Grid.FoodInViewport(w.Food, vx, vy, halfW*2, halfH*2)
}
