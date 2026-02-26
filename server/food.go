package main

import (
	"fmt"
	"math"
	"math/rand"
)

// Food represents a collectible item in the world.
// Level 1 = common, Level 3 = medium, Level 5 = death drop, Level 10 = rare moving food.
type Food struct {
	ID       string
	X        float64
	Y        float64
	Value    int
	Color    string
	Level    int  // 1, 3, 5, or 10
	IsMoving bool // true for level-10 rare moving food

	// Moving food fields (only used when IsMoving = true)
	MoveAngle float64 // radians, current travel direction
	MoveSpeed float64 // px per tick
	MoveTicks int     // ticks until next random direction change
}

// NewFood creates a food item at a random position inside the circular world.
// 90% chance level 1, 10% chance level 3.
func NewFood() *Food {
	x, y := randomCirclePoint(WorldCenterX, WorldCenterY, WorldRadius)
	level := FoodLevel1
	if rand.Float64() < 0.10 {
		level = FoodLevel3
	}
	return newFoodWithLevel(x, y, level, false)
}

// NewFoodAt creates a level-3 food item near a position (used on snake death).
// Scatters ±20px to spread food along the body instead of piling up.
func NewFoodAt(x, y float64) *Food {
	scatter := 20.0
	sx := x + (rand.Float64()*2-1)*scatter
	sy := y + (rand.Float64()*2-1)*scatter
	cx, cy := clampToCircle(sx, sy, WorldCenterX, WorldCenterY, WorldRadius)
	return newFoodWithLevel(cx, cy, FoodLevel3, false)
}

// NewMovingFood creates a level-10 moving food at a random position inside the world.
func NewMovingFood() *Food {
	x, y := randomCirclePoint(WorldCenterX, WorldCenterY, WorldRadius)
	f := newFoodWithLevel(x, y, FoodLevel10, true)
	f.MoveAngle = rand.Float64() * 2 * math.Pi
	f.MoveSpeed = MovingFoodSpeed
	f.MoveTicks = MovingFoodDirMinTicks + rand.Intn(MovingFoodDirMaxTicks-MovingFoodDirMinTicks)
	return f
}

// newFoodWithLevel is the internal constructor
func newFoodWithLevel(x, y float64, level int, isMoving bool) *Food {
	return &Food{
		ID:       newFoodID(),
		X:        x,
		Y:        y,
		Value:    level,
		Color:    foodColorForLevel(level),
		Level:    level,
		IsMoving: isMoving,
	}
}

// UpdateMoving advances moving food one tick: moves, bounces off boundary, counts down direction timer.
func (f *Food) UpdateMoving() {
	if !f.IsMoving {
		return
	}

	// Move food
	f.X += math.Cos(f.MoveAngle) * f.MoveSpeed
	f.Y += math.Sin(f.MoveAngle) * f.MoveSpeed

	// Bounce off circular boundary
	dx := f.X - WorldCenterX
	dy := f.Y - WorldCenterY
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist > WorldRadius {
		// Reflect angle off the normal at collision point
		// Normal points inward: (-dx/dist, -dy/dist)
		nx := -dx / dist
		ny := -dy / dist
		// Reflect: v' = v - 2(v·n)n
		vx := math.Cos(f.MoveAngle)
		vy := math.Sin(f.MoveAngle)
		dot := vx*nx + vy*ny
		vx = vx - 2*dot*nx
		vy = vy - 2*dot*ny
		f.MoveAngle = math.Atan2(vy, vx)
		// Push back inside boundary
		f.X = WorldCenterX + nx*(WorldRadius-1)
		f.Y = WorldCenterY + ny*(WorldRadius-1)
	}

	// Count down direction timer and randomize when expired
	f.MoveTicks--
	if f.MoveTicks <= 0 {
		f.MoveAngle = rand.Float64() * 2 * math.Pi
		f.MoveTicks = MovingFoodDirMinTicks + rand.Intn(MovingFoodDirMaxTicks-MovingFoodDirMinTicks)
	}
}

// ToDTO converts Food to a serializable DTO.
func (f *Food) ToDTO() FoodDTO {
	isMovingInt := 0
	if f.IsMoving {
		isMovingInt = 1
	}
	return FoodDTO{
		ID:       f.ID,
		X:        roundTo1(f.X),
		Y:        roundTo1(f.Y),
		Value:    f.Value,
		Color:    f.Color,
		Level:    f.Level,
		IsMoving: isMovingInt,
	}
}

// DistanceTo returns distance from food to a point
func (f *Food) DistanceTo(x, y float64) float64 {
	dx := f.X - x
	dy := f.Y - y
	return math.Sqrt(dx*dx + dy*dy)
}

var foodCounter int

func newFoodID() string {
	foodCounter++
	return fmt.Sprintf("f%d", foodCounter)
}

// foodColorForLevel returns a color keyed to food level
func foodColorForLevel(level int) string {
	switch level {
	case FoodLevel3:
		return randomFromSlice(foodColorsLevel3)
	case FoodLevel5:
		return randomFromSlice(foodColorsLevel5)
	case FoodLevel10:
		return "#ffd700" // gold for rare moving food
	default:
		return randomFromSlice(foodColorsLevel1)
	}
}

var foodColorsLevel1 = []string{
	"#ff6b6b", "#ffd93d", "#6bcb77", "#4d96ff", "#ff922b",
	"#cc5de8", "#20c997", "#f06595", "#74c0fc", "#a9e34b",
}

var foodColorsLevel3 = []string{
	"#f39c12", "#e67e22", "#d35400", "#c0392b", "#e74c3c",
}

var foodColorsLevel5 = []string{
	"#8e44ad", "#9b59b6", "#6c3483", "#a569bd", "#7d3c98",
}

func randomFromSlice(s []string) string {
	return s[rand.Intn(len(s))]
}

// NewFoodCluster creates a group of 5-12 food items clustered around a random center point.
// Cluster radius ~80-150px, making food visually grouped together.
func NewFoodCluster() []*Food {
	cx, cy := randomCirclePoint(WorldCenterX, WorldCenterY, WorldRadius-200)
	count := 5 + rand.Intn(8) // 5-12 items per cluster
	clusterRadius := 80.0 + rand.Float64()*70.0 // 80-150px spread

	foods := make([]*Food, count)
	for i := 0; i < count; i++ {
		// Scatter around cluster center
		angle := rand.Float64() * 2 * math.Pi
		r := clusterRadius * math.Sqrt(rand.Float64())
		fx := cx + r*math.Cos(angle)
		fy := cy + r*math.Sin(angle)
		fx, fy = clampToCircle(fx, fy, WorldCenterX, WorldCenterY, WorldRadius)

		level := FoodLevel1
		if rand.Float64() < 0.10 {
			level = FoodLevel3
		}
		foods[i] = newFoodWithLevel(fx, fy, level, false)
	}
	return foods
}

// randomCirclePoint returns a uniformly random point inside a circle with given center and radius.
// Uses polar coordinates with sqrt(r) for uniform distribution.
func randomCirclePoint(cx, cy, radius float64) (float64, float64) {
	r := radius * math.Sqrt(rand.Float64())
	angle := rand.Float64() * 2 * math.Pi
	return cx + r*math.Cos(angle), cy + r*math.Sin(angle)
}

// clampToCircle moves point (x,y) inside circle if it is outside.
func clampToCircle(x, y, cx, cy, radius float64) (float64, float64) {
	dx := x - cx
	dy := y - cy
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist <= radius {
		return x, y
	}
	// Project back onto boundary with small margin
	scale := (radius - 1) / dist
	return cx + dx*scale, cy + dy*scale
}

// roundTo1 rounds a float64 to 1 decimal place to save protocol bytes.
func roundTo1(v float64) float64 {
	return math.Round(v*10) / 10
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
