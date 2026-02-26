package main

// Game configuration constants
const (
	// Server
	ServerPort    = ":8080"
	StaticDir     = "../client"
	WebSocketPath = "/ws"

	// World — circular map: center=(10500,10500), radius=10500
	// Boundary is death (not wrap). Diameter ~21000px.
	WorldCenterX = 10500.0
	WorldCenterY = 10500.0
	WorldRadius  = 10500.0
	// SpawnMargin keeps snakes away from the circular boundary on spawn
	SpawnMargin = 500.0

	// Game loop
	TickRate = 20 // ticks per second
	TickMS   = 1000 / TickRate

	// Snake
	SnakeNormalSpeed    = 3.0  // px per tick
	SnakeBoostSpeed     = 5.0  // px per tick
	SnakeBoostCostTicks = 3    // lose 1 length unit every N boost ticks
	SnakeInitSegments   = 10   // starting segments
	SnakeSegmentSpacing = 8.0  // px between segments
	SnakeHeadRadius     = 10.0 // collision radius for head
	SnakeBodyRadius     = 8.0  // collision radius for body segments
	SnakeMinSegments    = 3    // minimum segments before death from boost
	SnakeBaseWidth      = 10.0 // starting visual radius
	SnakeMaxWidth       = 28.0 // cap visual radius
	// Turn rate: max radians per tick the snake can rotate.
	// Bigger snakes turn slower. Formula: MaxTurnRate / (1 + segments * TurnScaleFactor)
	SnakeMaxTurnRate   = 0.18 // radians/tick at minimum size (~10 degrees)
	SnakeTurnScaleFactor = 0.008 // how much each segment reduces turn rate

	// Food
	InitialFoodCount = 12500
	TargetFoodCount  = 12500
	FoodRadius       = 5.0
	FoodBaseValue    = 1
	DeathFoodPerUnit = 2  // food items dropped per body segment on death
	FoodSpawnPerTick = 100 // max food respawn per tick to maintain target

	// Food levels
	// Level 1: value=1, common (90% of random spawns)
	// Level 3: value=3, medium (10% of random spawns)
	// Level 5: value=5, large (only from death drops)
	// Level 10: value=10, rare moving food
	FoodLevel1 = 1
	FoodLevel3 = 3
	FoodLevel5 = 5
	FoodLevel10 = 10

	// Moving food (level 10)
	MovingFoodSpawnInterval = 300 // ticks between moving food spawns (~15 sec at 20 tps)
	MovingFoodMaxCount      = 3   // max moving food in world at once
	MovingFoodSpeed         = 4.0 // px per tick
	// Moving food changes direction every 60-120 ticks (random in that range)
	MovingFoodDirMinTicks = 60
	MovingFoodDirMaxTicks = 120

	// Magnetic food attraction
	MagnetRadius = 16.0 // px — food within this radius gets pulled (1.6x head radius)
	MagnetSpeed  = 3.0  // px per tick — how fast food moves toward snake head

	// Viewport
	ViewportWidth  = 1536.0 // 1920 * 0.8
	ViewportHeight = 864.0  // 1080 * 0.8
	ViewportBuffer = 200.0

	// Spatial grid — covers bounding square of circular world (0..2*WorldRadius)
	GridCellSize = 200.0

	// Leaderboard
	LeaderboardSize = 10

	// Collision
	CollisionCheckRadius = 20.0 // radius for head-to-body collision check

	// Bot AI
	BotCount          = 50    // number of AI bots to maintain
	BotRespawnDelay   = 100   // ticks before respawning a dead bot (~5 sec at 20 tps)
	BotDangerRadius   = 80.0  // px — body segments closer than this trigger avoidance
	BotFoodSeekRadius = 500.0 // px — food within this range is targeted (was 200)
	BotChaseRadius    = 300.0 // px — smaller snake heads within this range are chased
	BotFleeRadius     = 200.0 // px — bigger snake heads within this range trigger flee
	BotBoundaryBuffer = 500.0 // px — steer toward center when this close to boundary
)

// Player colors palette
var PlayerColors = []string{
	"#e74c3c", "#3498db", "#2ecc71", "#f39c12", "#9b59b6",
	"#1abc9c", "#e67e22", "#e91e63", "#00bcd4", "#8bc34a",
	"#ff5722", "#607d8b", "#795548", "#673ab7", "#03a9f4",
	"#4caf50", "#ffeb3b", "#ff9800", "#f44336", "#9c27b0",
}
