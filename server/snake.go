package main

import (
	"math"
	"math/rand"
)

// Point is a 2D coordinate
type Point struct {
	X float64
	Y float64
}

// Snake represents a player's snake in the world
type Snake struct {
	ID          string
	Name        string
	Segments    []Point // index 0 = head
	Angle       float64 // radians, direction of movement
	Speed       float64
	Score       int
	Color       string
	Alive       bool
	BoostActive bool
	BoostTicks  int     // ticks spent boosting this cycle
	Width       float64 // visual width (radius), starts at SnakeBaseWidth
}

// NewSnake creates a snake at a random position inside the circular world,
// keeping SpawnMargin px away from the boundary.
func NewSnake(id, name, color string) *Snake {
	spawnRadius := WorldRadius - SpawnMargin
	r := spawnRadius * math.Sqrt(rand.Float64())
	spawnAngle := rand.Float64() * 2 * math.Pi
	x := WorldCenterX + r*math.Cos(spawnAngle)
	y := WorldCenterY + r*math.Sin(spawnAngle)

	angle := rand.Float64() * 2 * math.Pi

	segments := make([]Point, SnakeInitSegments)
	for i := 0; i < SnakeInitSegments; i++ {
		segments[i] = Point{
			X: x - float64(i)*SnakeSegmentSpacing*math.Cos(angle),
			Y: y - float64(i)*SnakeSegmentSpacing*math.Sin(angle),
		}
	}

	return &Snake{
		ID:       id,
		Name:     name,
		Segments: segments,
		Angle:    angle,
		Speed:    SnakeNormalSpeed,
		Score:    SnakeInitSegments,
		Color:    color,
		Alive:    true,
		Width:    SnakeBaseWidth,
	}
}

// Head returns the head segment of the snake
func (s *Snake) Head() Point {
	return s.Segments[0]
}

// Move advances the snake one tick in its current direction.
// Returns true if the snake crossed the circular boundary (caller should kill it).
func (s *Snake) Move() bool {
	head := s.Head()

	newX := head.X + s.Speed*math.Cos(s.Angle)
	newY := head.Y + s.Speed*math.Sin(s.Angle)

	// Check circular boundary — boundary crossing = death
	dx := newX - WorldCenterX
	dy := newY - WorldCenterY
	outOfBounds := (dx*dx + dy*dy) > WorldRadius*WorldRadius

	newHead := Point{X: newX, Y: newY}

	// Shift segments: prepend new head, drop last
	s.Segments = append([]Point{newHead}, s.Segments[:len(s.Segments)-1]...)

	return outOfBounds
}

// Grow adds segments at the tail and increases width with diminishing returns.
// Width gain = foodValue / totalSegments (longer snake → less width gain per food).
func (s *Snake) Grow(amount int) {
	tail := s.Segments[len(s.Segments)-1]
	for i := 0; i < amount; i++ {
		s.Segments = append(s.Segments, tail)
	}
	s.Score += amount
	// Width grows proportionally: 4 * food_value / total_length (4x multiplier for visible growth)
	widthGain := 4.0 * float64(amount) / float64(len(s.Segments))
	s.Width += widthGain
	if s.Width > SnakeMaxWidth {
		s.Width = SnakeMaxWidth
	}
}

// ApplyInput updates the snake's angle and boost state from client input.
// Turn rate is limited based on snake size — bigger snakes must arc wider to reverse.
// Returns level-3 food dropped from tail when boosting (nil if none dropped).
func (s *Snake) ApplyInput(angle float64, boost bool) *Food {
	// Calculate max turn rate for this snake's size
	maxTurn := SnakeMaxTurnRate / (1.0 + float64(len(s.Segments))*SnakeTurnScaleFactor)

	// Calculate shortest angular difference (handles wrapping around -π/π)
	diff := angle - s.Angle
	// Normalize to [-π, π]
	for diff > math.Pi {
		diff -= 2 * math.Pi
	}
	for diff < -math.Pi {
		diff += 2 * math.Pi
	}
	// Clamp to max turn rate
	if diff > maxTurn {
		diff = maxTurn
	} else if diff < -maxTurn {
		diff = -maxTurn
	}
	s.Angle += diff

	s.BoostActive = boost

	if boost {
		s.Speed = SnakeBoostSpeed
		s.BoostTicks++
		// Lose a segment every N boost ticks to "cost" boost
		if s.BoostTicks%SnakeBoostCostTicks == 0 && len(s.Segments) > SnakeMinSegments {
			tail := s.Segments[len(s.Segments)-1]
			s.Segments = s.Segments[:len(s.Segments)-1]
			s.Score--
			// Shrink width proportionally when losing segments
			widthLoss := 4.0 / float64(len(s.Segments)+1)
			s.Width -= widthLoss
			if s.Width < SnakeBaseWidth {
				s.Width = SnakeBaseWidth
			}
			// Only drop food 50% of the time (50% pure cost)
			if rand.Float64() < 0.5 {
				f := newFoodWithLevel(tail.X, tail.Y, FoodLevel3, false)
				f.Color = s.Color
				return f
			}
			return nil
		}
	} else {
		s.Speed = SnakeNormalSpeed
		s.BoostTicks = 0
	}
	return nil
}

// DropFood converts the snake body into food items and marks it dead.
// Only drops 70% of body segments as food to act as a score sink.
func (s *Snake) DropFood() []*Food {
	s.Alive = false
	totalDrops := len(s.Segments) / DeathFoodPerUnit
	dropCount := int(float64(totalDrops) * 0.7)
	food := make([]*Food, 0, dropCount+1)
	for i, seg := range s.Segments {
		if i%DeathFoodPerUnit == 0 {
			if len(food) >= dropCount {
				break
			}
			food = append(food, NewFoodAt(seg.X, seg.Y))
		}
	}
	return food
}

// ToDTO converts snake to serializable form, trimming segments to maxSegs.
// If maxSegs <= 0 all segments are included.
// Coordinates are rounded to 1 decimal place to reduce wire size.
func (s *Snake) ToDTO(maxSegs int) SnakeDTO {
	segs := s.Segments
	if maxSegs > 0 && len(segs) > maxSegs {
		segs = segs[:maxSegs]
	}
	// Encode as flat [x,y] pairs (2-element float64 arrays) to minimize JSON size
	pairs := make([][2]float64, len(segs))
	for i, p := range segs {
		pairs[i] = [2]float64{roundTo1(p.X), roundTo1(p.Y)}
	}
	boostInt := 0
	if s.BoostActive {
		boostInt = 1
	}
	return SnakeDTO{
		ID:       s.ID,
		Name:     s.Name,
		Segments: pairs,
		Score:    s.Score,
		Color:    s.Color,
		Boosting: boostInt,
		Width:    roundTo1(s.Width),
	}
}
