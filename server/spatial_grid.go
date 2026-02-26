package main

import "math"

// cellKey uniquely identifies a grid cell
type cellKey struct {
	cx, cy int
}

// gridEntry holds a reference to food or snake segment in a cell
type gridEntry struct {
	foodID  string
	snakeID string
	segIdx  int
	x, y    float64
}

// SpatialGrid is a hash grid for fast proximity queries
type SpatialGrid struct {
	cells    map[cellKey][]gridEntry
	cellSize float64
}

// NewSpatialGrid creates an empty spatial grid
func NewSpatialGrid(cellSize float64) *SpatialGrid {
	return &SpatialGrid{
		cells:    make(map[cellKey][]gridEntry),
		cellSize: cellSize,
	}
}

// Clear resets all cells
func (g *SpatialGrid) Clear() {
	g.cells = make(map[cellKey][]gridEntry)
}

func (g *SpatialGrid) keyFor(x, y float64) cellKey {
	return cellKey{
		cx: int(math.Floor(x / g.cellSize)),
		cy: int(math.Floor(y / g.cellSize)),
	}
}

// InsertFood adds a food item to the grid
func (g *SpatialGrid) InsertFood(f *Food) {
	k := g.keyFor(f.X, f.Y)
	g.cells[k] = append(g.cells[k], gridEntry{foodID: f.ID, x: f.X, y: f.Y})
}

// InsertSnakeBody adds snake body segments (skipping head) to the grid
func (g *SpatialGrid) InsertSnakeBody(s *Snake) {
	// Start from index 1 to skip head (head checked separately)
	for i := 1; i < len(s.Segments); i++ {
		seg := s.Segments[i]
		k := g.keyFor(seg.X, seg.Y)
		g.cells[k] = append(g.cells[k], gridEntry{
			snakeID: s.ID,
			segIdx:  i,
			x:       seg.X,
			y:       seg.Y,
		})
	}
}

// NearbyFood returns food IDs within radius of (x,y)
func (g *SpatialGrid) NearbyFood(x, y, radius float64) []string {
	results := []string{}
	minCX := int(math.Floor((x - radius) / g.cellSize))
	maxCX := int(math.Floor((x + radius) / g.cellSize))
	minCY := int(math.Floor((y - radius) / g.cellSize))
	maxCY := int(math.Floor((y + radius) / g.cellSize))

	r2 := radius * radius
	for cx := minCX; cx <= maxCX; cx++ {
		for cy := minCY; cy <= maxCY; cy++ {
			for _, e := range g.cells[cellKey{cx, cy}] {
				if e.foodID == "" {
					continue
				}
				dx := e.x - x
				dy := e.y - y
				if dx*dx+dy*dy <= r2 {
					results = append(results, e.foodID)
				}
			}
		}
	}
	return results
}

// NearbySnakeBody returns (snakeID, segIdx) pairs within radius of (x,y),
// excluding the snake identified by excludeID
func (g *SpatialGrid) NearbySnakeBody(x, y, radius float64, excludeID string) []gridEntry {
	results := []gridEntry{}
	minCX := int(math.Floor((x - radius) / g.cellSize))
	maxCX := int(math.Floor((x + radius) / g.cellSize))
	minCY := int(math.Floor((y - radius) / g.cellSize))
	maxCY := int(math.Floor((y + radius) / g.cellSize))

	r2 := radius * radius
	for cx := minCX; cx <= maxCX; cx++ {
		for cy := minCY; cy <= maxCY; cy++ {
			for _, e := range g.cells[cellKey{cx, cy}] {
				if e.snakeID == "" || e.snakeID == excludeID {
					continue
				}
				dx := e.x - x
				dy := e.y - y
				if dx*dx+dy*dy <= r2 {
					results = append(results, e)
				}
			}
		}
	}
	return results
}

// FoodInViewport returns food items that fall within the given viewport rectangle
func (g *SpatialGrid) FoodInViewport(food map[string]*Food, vx, vy, vw, vh float64) []FoodDTO {
	result := []FoodDTO{}
	minCX := int(math.Floor(vx / g.cellSize))
	maxCX := int(math.Floor((vx + vw) / g.cellSize))
	minCY := int(math.Floor(vy / g.cellSize))
	maxCY := int(math.Floor((vy + vh) / g.cellSize))

	seen := map[string]bool{}
	for cx := minCX; cx <= maxCX; cx++ {
		for cy := minCY; cy <= maxCY; cy++ {
			for _, e := range g.cells[cellKey{cx, cy}] {
				if e.foodID == "" || seen[e.foodID] {
					continue
				}
				if f, ok := food[e.foodID]; ok {
					seen[e.foodID] = true
					result = append(result, f.ToDTO())
				}
			}
		}
	}
	return result
}
