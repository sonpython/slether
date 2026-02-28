package main

import (
	"fmt"
	"math"
	"math/rand"
)

// botNames is a multilingual pool of snake/warrior themed names
var botNames = []string{
	// Vietnamese
	"Rắn Thần", "Sấm Sét", "Bão Tố", "Tia Chớp", "Ma Tốc Độ",
	"Rồng Lửa", "Bóng Đêm", "Sát Thủ", "Độc Xà", "Vua Rắn",
	"Hắc Mamba", "Kim Xà", "Thanh Xà", "Bạch Xà", "Thần Xà",
	"Hỏa Long", "Băng Xà", "Quỷ Xà", "Điện Xà", "Lôi Thần",
	// English
	"Viper", "Cobra", "Mamba", "Python", "Anaconda",
	"Sidewinder", "Rattlesnake", "Phantom", "Shadow", "Blaze",
	"Frostbite", "Venom", "Reaper", "Striker", "Apex",
	"Cyclone", "Tempest", "Havoc", "Wraith", "Spectre",
	// Japanese
	"蛇神", "雷蛇", "龍王", "鬼蛇", "忍者",
	"侍", "影", "嵐", "炎蛇", "氷龍",
	// Korean
	"독사왕", "번개뱀", "용의발톱", "그림자", "폭풍",
	"흑사", "천둥", "불뱀", "얼음독", "광전사",
	// Chinese
	"毒蛇王", "雷电蛇", "火龙", "冰蟒", "暗影",
	"狂蛇", "风暴", "霸蛇", "鬼火", "战神",
	// Spanish
	"Serpiente", "Víbora", "Trueno", "Tormenta", "Fuego",
	"Sombra", "Veneno", "Relámpago", "Fantasma", "Dragón",
	// Russian
	"Гадюка", "Кобра", "Гром", "Буря", "Тень",
	"Пламя", "Мороз", "Ужас", "Змей", "Дракон",
	// Arabic
	"الأفعى", "البرق", "العاصفة", "الظل", "النار",
	// Thai
	"พญานาค", "สายฟ้า", "มังกร", "เงา", "พิษ",
	// Hindi
	"नागराज", "बिजली", "तूफान", "अग्नि", "विष",
	// Portuguese
	"Serpente", "Raio", "Tempestade", "Sombra", "Veneno",
	// French
	"Vipère", "Éclair", "Tonnerre", "Ombre", "Flamme",
	// German
	"Schlange", "Blitz", "Donner", "Schatten", "Flamme",
}

// botUsedNames tracks names currently in use to prevent duplicates
var botUsedNames = map[string]bool{}

// Bot tracks per-bot AI state
type Bot struct {
	ID          string
	wanderTicks int     // ticks remaining before picking a new wander direction
	targetAngle float64 // angle the bot is currently steering toward
	boostTicks  int     // remaining ticks of intentional boost (flee/chase)
	respawnIn   int     // countdown ticks before respawning (0 = alive or ready)
	seekTicks   int     // ticks spent seeking food
	lastScore   int     // score last tick — detect if food was eaten
	lastFoodDist float64 // distance to target food last tick — detect orbiting
	orbitCount  int     // consecutive ticks where distance didn't decrease
	// Death food rush: when this bot kills another snake, rush to eat the dropped food
	deathFoodX    float64 // center of death food zone
	deathFoodY    float64
	deathFoodTicks int    // ticks remaining to rush toward death food (0 = inactive)
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
	name := pickBotName()
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

	// --- Priority 4.5: Rush to death food zone (after killing another snake) ---
	if bot.deathFoodTicks > 0 {
		bot.deathFoodTicks--
		ddx := bot.deathFoodX - head.X
		ddy := bot.deathFoodY - head.Y
		dist := math.Sqrt(ddx*ddx + ddy*ddy)
		if dist < 30 {
			// Arrived at death zone — stop rushing, normal food-seek will take over
			bot.deathFoodTicks = 0
		} else {
			bot.targetAngle = math.Atan2(ddy, ddx)
			// Boost toward death food if we can afford it
			if len(snake.Segments) > SnakeMinSegments+5 {
				boost = true
			}
			return bot.targetAngle, boost
		}
	}

	// --- Priority 5: Seek nearby food ---
	// Detect if bot ate something (score increased) → reset counters
	if snake.Score > bot.lastScore {
		bot.seekTicks = 0
		bot.orbitCount = 0
		bot.lastFoodDist = 0
	}
	bot.lastScore = snake.Score

	nearFoodIDs := w.Grid.NearbyFood(head.X, head.Y, BotFoodSeekRadius)
	if len(nearFoodIDs) > 0 && bot.seekTicks < 60 {
		// Find closest food ONLY in front of us (within ±90°)
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
			foodAngle := math.Atan2(fdy, fdx)
			angleDiff := math.Abs(normalizeAngle(foodAngle - currentAngle))
			// Skip food behind us entirely — chasing backward food causes orbits
			if angleDiff > math.Pi/2 {
				continue
			}
			if d < bestDist {
				bestDist = d
				bestFood = f
			}
		}
		if bestFood != nil {
			// Orbit detection: if distance to food isn't decreasing, we're circling
			if bot.lastFoodDist > 0 && bestDist >= bot.lastFoodDist-1.0 {
				bot.orbitCount++
			} else {
				bot.orbitCount = 0
			}
			bot.lastFoodDist = bestDist

			// If orbiting for 8+ ticks, abandon this food and break out
			if bot.orbitCount >= 8 {
				bot.orbitCount = 0
				bot.seekTicks = 0
				bot.lastFoodDist = 0
				bot.targetAngle = currentAngle + math.Pi/2 + rand.Float64()*math.Pi
				bot.wanderTicks = 30 + rand.Intn(40)
				return bot.targetAngle, false
			}

			// Steer directly at food
			bot.targetAngle = math.Atan2(bestFood.Y-head.Y, bestFood.X-head.X)
			bot.seekTicks++
			return bot.targetAngle, boost
		}
	}
	// If seek timed out (circling), force a new direction away from current heading
	if bot.seekTicks >= 60 {
		bot.seekTicks = 0
		bot.orbitCount = 0
		bot.lastFoodDist = 0
		bot.targetAngle = currentAngle + math.Pi/2 + rand.Float64()*math.Pi
		bot.wanderTicks = 30 + rand.Intn(40)
		return bot.targetAngle, false
	}

	// --- Priority 6: Roam uniformly across the entire map ---
	if bot.wanderTicks <= 0 {
		// Pick a random point anywhere in the world (uniform distribution)
		targetR := (WorldRadius - BotBoundaryBuffer) * math.Sqrt(rand.Float64())
		targetA := rand.Float64() * 2 * math.Pi
		tx := WorldCenterX + targetR*math.Cos(targetA)
		ty := WorldCenterY + targetR*math.Sin(targetA)
		bot.targetAngle = math.Atan2(ty-head.Y, tx-head.X)
		bot.wanderTicks = 40 + rand.Intn(60)
	}
	bot.wanderTicks--
	return bot.targetAngle, boost
}

// HandleDeaths scans for dead bot snakes (after game_loop processes deaths)
// and starts their respawn countdown. Also notifies killer bots to rush death food.
// Must be called while world.mu is held.
func (bm *BotManager) HandleDeaths(deaths map[string]string) {
	// Notify killer bots to rush to victim's death location
	for victimID, killerName := range deaths {
		victim, ok := bm.world.Snakes[victimID]
		if !ok {
			continue
		}
		// Find killer bot by name match
		for _, bot := range bm.bots {
			killerSnake, ok := bm.world.Snakes[bot.ID]
			if !ok || !killerSnake.Alive {
				continue
			}
			if killerSnake.Name == killerName {
				head := victim.Head()
				bot.deathFoodX = head.X
				bot.deathFoodY = head.Y
				bot.deathFoodTicks = 80 // rush for 4 seconds at 20tps
				break
			}
		}
	}

	// Start respawn countdown for dead bots
	for botID, bot := range bm.bots {
		snake, ok := bm.world.Snakes[botID]
		if !ok || !snake.Alive {
			if bot.respawnIn == 0 {
				bot.respawnIn = BotRespawnDelay
			}
		}
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
		// Release bot name before removing
		bm.world.mu.Lock()
		if s, ok := bm.world.Snakes[oldID]; ok {
			delete(botUsedNames, s.Name)
		}
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

// pickBotName returns a random unused name from the pool.
// If all names are taken, appends a number suffix to make it unique.
func pickBotName() string {
	// Shuffle and find first unused
	perm := rand.Perm(len(botNames))
	for _, i := range perm {
		name := botNames[i]
		if !botUsedNames[name] {
			botUsedNames[name] = true
			return name
		}
	}
	// All names taken — pick random + suffix
	base := botNames[rand.Intn(len(botNames))]
	for i := 2; ; i++ {
		name := fmt.Sprintf("%s %d", base, i)
		if !botUsedNames[name] {
			botUsedNames[name] = true
			return name
		}
	}
}

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
