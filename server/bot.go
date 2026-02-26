package main

import (
	"fmt"
	"math"
	"math/rand"
)

// botNames is the pool of Vietnamese-style names for AI bots
var botNames = []string{
	"Rắn Thần", "Sấm Sét", "Bão Tố", "Tia Chớp", "Ma Tốc Độ",
	"Rồng Lửa", "Bóng Đêm", "Sát Thủ", "Độc Xà", "Vua Rắn",
	"Hắc Mamba", "Kim Xà", "Thanh Xà", "Bạch Xà", "Viper",
	"Cobra", "Mamba", "Python", "Anaconda", "Sidewinder",
	"Thần Xà", "Hỏa Long", "Băng Xà", "Quỷ Xà", "Điện Xà",
}

// botNameCounter tracks how many bots have been created for unique naming
var botNameCounter int

// Bot tracks per-bot AI state
type Bot struct {
	ID          string
	wanderTicks int     // ticks remaining before picking a new wander direction
	targetAngle float64 // angle the bot is currently steering toward
	boostTicks  int     // remaining ticks of intentional boost (flee/chase)
	respawnIn   int     // countdown ticks before respawning (0 = alive or ready)
	seekTicks   int     // ticks spent seeking same food — if >40, give up to avoid circling
	lastScore   int     // score last tick — used to detect if food was eaten
}

// BotManager manages all AI bot snakes
type BotManager struct {
	world *World
	bots  map[string]*Bot // botID -> Bot
}

// NewBotManager creates a BotManager bound to the given world
func NewBotManager(world *World) *BotManager {
	return &BotManager{
		world: world,
		bots:  make(map[string]*Bot),
	}
}

// SpawnBot creates a new bot snake and registers it in the world.
// Caller must NOT hold world.mu — this method acquires the write lock.
func (bm *BotManager) SpawnBot() {
	id := fmt.Sprintf("bot-%d", rand.Int63())
	name := botNames[botNameCounter%len(botNames)]
	botNameCounter++
	color := PlayerColors[rand.Intn(len(PlayerColors))]

	snake := NewSnake(id, name, color)

	bm.world.mu.Lock()
	bm.world.AddSnake(snake)
	bm.world.mu.Unlock()

	bot := &Bot{
		ID:          id,
		targetAngle: snake.Angle,
		wanderTicks: randomWanderDuration(),
	}
	bm.bots[id] = bot
}

// Update runs AI logic for every bot. Must be called each tick while world.mu is held.
func (bm *BotManager) Update() {
	w := bm.world
	for _, bot := range bm.bots {
		snake, ok := w.Snakes[bot.ID]
		if !ok || !snake.Alive {
			continue
		}

		angle, boost := bm.decideBotInput(bot, snake)
		if dropped := snake.ApplyInput(angle, boost); dropped != nil {
			w.Food[dropped.ID] = dropped
		}
		outOfBounds := snake.Move()
		if outOfBounds {
			// Boundary death — drop food into world and mark dead
			dropped := snake.DropFood()
			w.AddFood(dropped)
		}
	}
}

// decideBotInput applies priority-based AI rules and returns (targetAngle, boost).
// Must be called while world.mu is held (at least read).
func (bm *BotManager) decideBotInput(bot *Bot, snake *Snake) (float64, bool) {
	w := bm.world
	head := snake.Head()
	currentAngle := snake.Angle
	boost := false

	// --- Priority 1: Boundary avoidance ---
	dx := head.X - WorldCenterX
	dy := head.Y - WorldCenterY
	distFromCenter := math.Sqrt(dx*dx + dy*dy)
	if distFromCenter > WorldRadius-BotBoundaryBuffer {
		// Steer toward world center
		bot.targetAngle = math.Atan2(WorldCenterY-head.Y, WorldCenterX-head.X)
		bot.wanderTicks = randomWanderDuration()
		return bot.targetAngle, false
	}

	// --- Priority 2: Danger avoidance — body segments within BotDangerRadius ahead ---
	nearby := w.Grid.NearbySnakeBody(head.X, head.Y, BotDangerRadius, snake.ID)
	for _, entry := range nearby {
		// Check if the segment is within ±45° of the current heading (in our path)
		segAngle := math.Atan2(entry.y-head.Y, entry.x-head.X)
		angleDiff := normalizeAngle(segAngle - currentAngle)
		if math.Abs(angleDiff) < math.Pi/4 {
			// Turn 90° away — choose left or right based on which avoids the obstacle
			if angleDiff >= 0 {
				bot.targetAngle = currentAngle - math.Pi/2
			} else {
				bot.targetAngle = currentAngle + math.Pi/2
			}
			bot.wanderTicks = randomWanderDuration()
			return bot.targetAngle, false
		}
	}

	// --- Priority 3: Flee bigger snakes ---
	biggerFound := false
	for _, other := range w.Snakes {
		if other.ID == snake.ID || !other.Alive {
			continue
		}
		otherHead := other.Head()
		ddx := otherHead.X - head.X
		ddy := otherHead.Y - head.Y
		dist := math.Sqrt(ddx*ddx + ddy*ddy)
		if dist < BotFleeRadius && other.Score > snake.Score {
			// Flee: steer directly away from the threat
			bot.targetAngle = math.Atan2(head.Y-otherHead.Y, head.X-otherHead.X)
			bot.boostTicks = 30 // boost for 30 ticks while fleeing
			bot.wanderTicks = randomWanderDuration()
			biggerFound = true
			break
		}
	}
	if biggerFound {
		if bot.boostTicks > 0 {
			bot.boostTicks--
			boost = true
		}
		return bot.targetAngle, boost
	}

	// Reset boost if not fleeing
	if bot.boostTicks > 0 {
		bot.boostTicks--
		boost = true
	}

	// --- Priority 4: Chase smaller snakes ---
	for _, other := range w.Snakes {
		if other.ID == snake.ID || !other.Alive {
			continue
		}
		otherHead := other.Head()
		ddx := otherHead.X - head.X
		ddy := otherHead.Y - head.Y
		dist := math.Sqrt(ddx*ddx + ddy*ddy)
		if dist < BotChaseRadius && other.Score < snake.Score {
			bot.targetAngle = math.Atan2(ddy, ddx)
			bot.wanderTicks = randomWanderDuration()
			// Boost toward smaller target only if we can afford it
			if len(snake.Segments) > SnakeMinSegments+5 {
				boost = true
			}
			return bot.targetAngle, boost
		}
	}

	// --- Priority 5: Seek nearby food ---
	// Detect if bot ate something (score increased) → reset seek counter
	if snake.Score > bot.lastScore {
		bot.seekTicks = 0
	}
	bot.lastScore = snake.Score

	nearFoodIDs := w.Grid.NearbyFood(head.X, head.Y, BotFoodSeekRadius)
	if len(nearFoodIDs) > 0 && bot.seekTicks < 60 {
		// Find closest food that's roughly in front of us (prefer reachable food)
		bestDist := math.MaxFloat64
		var bestFood *Food
		for _, fid := range nearFoodIDs {
			f, ok := w.Food[fid]
			if !ok {
				continue
			}
			fdx := f.X - head.X
			fdy := f.Y - head.Y
			d := math.Sqrt(fdx*fdx + fdy*fdy)
			// Prefer food in front — penalize food behind by 2x distance
			foodAngle := math.Atan2(fdy, fdx)
			angleDiff := math.Abs(normalizeAngle(foodAngle - currentAngle))
			if angleDiff > math.Pi/2 {
				d *= 2.0 // behind us, deprioritize
			}
			if d < bestDist {
				bestDist = d
				bestFood = f
			}
		}
		if bestFood != nil {
			bot.targetAngle = math.Atan2(bestFood.Y-head.Y, bestFood.X-head.X)
			bot.seekTicks++
			return bot.targetAngle, boost
		}
	}
	// If seek timed out (circling), force a new direction away from current heading
	if bot.seekTicks >= 60 {
		bot.seekTicks = 0
		// Turn ~90-180° to break orbit pattern
		bot.targetAngle = currentAngle + math.Pi/2 + rand.Float64()*math.Pi
		bot.wanderTicks = 30 + rand.Intn(40)
		return bot.targetAngle, false
	}

	// --- Priority 6: Proactive roam — move toward center bias (food denser there) ---
	if bot.wanderTicks <= 0 {
		// 80% chance: roam toward a random point closer to center (food-rich)
		// 20% chance: pure random direction for exploration
		if rand.Float64() < 0.8 {
			// Pick random point within inner 70% of world
			targetR := WorldRadius * 0.7 * math.Sqrt(rand.Float64())
			targetA := rand.Float64() * 2 * math.Pi
			tx := WorldCenterX + targetR*math.Cos(targetA)
			ty := WorldCenterY + targetR*math.Sin(targetA)
			bot.targetAngle = math.Atan2(ty-head.Y, tx-head.X)
		} else {
			bot.targetAngle = rand.Float64() * 2 * math.Pi
		}
		bot.wanderTicks = 20 + rand.Intn(30)
	}
	bot.wanderTicks--
	return bot.targetAngle, boost
}

// HandleDeaths scans for dead bot snakes (after game_loop processes deaths)
// and starts their respawn countdown. Must be called while world.mu is held.
func (bm *BotManager) HandleDeaths(deaths map[string]string) {
	for botID, bot := range bm.bots {
		snake, ok := bm.world.Snakes[botID]
		if !ok || !snake.Alive {
			if bot.respawnIn == 0 {
				// Start countdown only once
				bot.respawnIn = BotRespawnDelay
			}
		}
		_ = deaths // deaths map consulted implicitly via snake.Alive check
	}
}

// tickRespawns decrements respawn counters and triggers spawning when ready.
// Must be called while world.mu is NOT held (SpawnBot acquires the lock).
func (bm *BotManager) tickRespawns() {
	// Collect IDs to respawn (can't modify map during range)
	var toRespawn []string
	for botID, bot := range bm.bots {
		if bot.respawnIn <= 0 {
			continue
		}
		bot.respawnIn--
		if bot.respawnIn == 0 {
			toRespawn = append(toRespawn, botID)
		}
	}
	for _, oldID := range toRespawn {
		// Remove old dead snake from world + bot registry
		bm.world.mu.Lock()
		delete(bm.world.Snakes, oldID)
		bm.world.mu.Unlock()
		delete(bm.bots, oldID)
		bm.SpawnBot()
	}
}

// MaintainBotCount ensures exactly BotCount bots exist (alive + in-respawn).
// Must be called while world.mu is NOT held.
func (bm *BotManager) MaintainBotCount() {
	// tickRespawns first so dead bots count correctly
	bm.tickRespawns()

	if len(bm.bots) < BotCount {
		bm.SpawnBot()
	}
}

// --- helpers ---

// randomWanderDuration returns a tick count in [60, 120]
func randomWanderDuration() int {
	return 60 + rand.Intn(61)
}

// normalizeAngle wraps an angle into (-π, π]
func normalizeAngle(a float64) float64 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}
