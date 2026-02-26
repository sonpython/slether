package main

import (
	"log"
	"math"
	"math/rand"
	"time"
)

// GameLoop drives the game at a fixed tick rate
type GameLoop struct {
	world        *World
	conns        *ConnManager
	bots         *BotManager
	killMap      map[string]string // victimID -> killerName
	tickCount    int               // total ticks elapsed, used for moving food spawn timing
}

// NewGameLoop creates a game loop bound to world and conn manager.
// It also creates and pre-populates the BotManager with BotCount initial bots.
func NewGameLoop(world *World, conns *ConnManager) *GameLoop {
	bm := NewBotManager(world)
	// Pre-spawn initial bots before the game loop starts
	for i := 0; i < BotCount; i++ {
		bm.SpawnBot()
	}
	return &GameLoop{
		world:   world,
		conns:   conns,
		bots:    bm,
		killMap: make(map[string]string),
	}
}

// Run starts the fixed-timestep loop. Blocks until process exits.
func (gl *GameLoop) Run() {
	ticker := time.NewTicker(time.Second / TickRate)
	defer ticker.Stop()
	log.Printf("game loop started at %d ticks/sec", TickRate)

	for range ticker.C {
		gl.tick()
	}
}

// tick executes a single game update
func (gl *GameLoop) tick() {
	gl.tickCount++
	w := gl.world
	w.mu.Lock()

	// 1. Update moving food positions (before collision so magnets see updated pos)
	gl.updateMovingFood()

	// 2a. Update bot AI — bots decide input and move themselves inside Update()
	boundaryDeaths := map[string]bool{}
	gl.bots.Update()
	// Collect any boundary-crossing bot snakes (marked dead by bot.Update)
	for botID := range gl.bots.bots {
		if s, ok := w.Snakes[botID]; ok && !s.Alive {
			boundaryDeaths[botID] = true
		}
	}

	// 2b. Apply player inputs and move player snakes; detect boundary crossings
	conns := gl.conns.Snapshot()
	for _, c := range conns {
		snake, ok := w.Snakes[c.ID]
		if !ok || !snake.Alive {
			continue
		}
		inp := c.GetInput()
		if dropped := snake.ApplyInput(inp.Angle, inp.Boost); dropped != nil {
			w.Food[dropped.ID] = dropped
		}
		outOfBounds := snake.Move()
		if outOfBounds {
			boundaryDeaths[snake.ID] = true
		}
	}

	// 3. Rebuild spatial grid after movement
	w.RebuildGrid()

	// 4. Collision detection (head-to-body, head-to-head)
	gl.killMap = make(map[string]string)
	deaths := gl.detectCollisions()

	// 5. Merge boundary deaths into deaths map
	for id := range boundaryDeaths {
		if _, alreadyDead := deaths[id]; !alreadyDead {
			deaths[id] = "Boundary"
		}
	}

	// 6. Process deaths — drop food, record killer names
	for victimID, killerName := range deaths {
		snake := w.Snakes[victimID]
		if snake == nil || !snake.Alive {
			continue
		}
		dropped := snake.DropFood()
		w.AddFood(dropped)
		gl.killMap[victimID] = killerName
		log.Printf("snake %s (%s) died to %s, dropped %d food", snake.Name, victimID, killerName, len(dropped))
	}

	// 6b. Notify bot manager of deaths so it can start respawn countdowns
	gl.bots.HandleDeaths(deaths)

	// 7. Apply magnetic food attraction then collect food
	gl.applyFoodMagnet()
	gl.collectFood()

	// 8. Spawn moving food if conditions are met
	gl.maybeSpawnMovingFood()

	// 9. Maintain total food count
	w.MaintainFoodCount()

	leaderboard := w.Leaderboard()

	w.mu.Unlock()

	// 10a. Tick bot respawn countdowns and spawn replacements (acquires lock internally)
	gl.bots.MaintainBotCount()

	// 10b. Broadcast viewport-culled state to all connected players
	gl.broadcast(leaderboard)

	// 11. Send death messages to dead players
	for victimID, killerName := range gl.killMap {
		conn, ok := gl.conns.Get(victimID)
		if !ok {
			continue
		}
		w.mu.RLock()
		score := 0
		if s, exists := w.Snakes[victimID]; exists {
			score = s.Score
		}
		w.mu.RUnlock()

		_ = conn.Send(DeathMsg{
			Type:   MsgDeath,
			Killer: killerName,
			Score:  score,
		})
	}
}

// updateMovingFood advances all level-10 moving food items one tick.
// Caller must hold w.mu.Lock.
func (gl *GameLoop) updateMovingFood() {
	w := gl.world
	for _, f := range w.Food {
		if f.IsMoving {
			f.UpdateMoving()
		}
	}
}

// maybeSpawnMovingFood spawns a new level-10 moving food every MovingFoodSpawnInterval ticks,
// if fewer than MovingFoodMaxCount exist. Caller must hold w.mu.Lock.
func (gl *GameLoop) maybeSpawnMovingFood() {
	if gl.tickCount%MovingFoodSpawnInterval != 0 {
		return
	}
	w := gl.world
	// Count existing moving food
	count := 0
	for _, f := range w.Food {
		if f.IsMoving {
			count++
		}
	}
	if count >= MovingFoodMaxCount {
		return
	}
	mf := NewMovingFood()
	w.Food[mf.ID] = mf
	log.Printf("spawned moving food %s (total moving: %d)", mf.ID, count+1)
}

// applyFoodMagnet pulls food within MagnetRadius toward each alive snake head.
// Food within actual eating radius is left for collectFood to handle.
// Caller must hold w.mu.Lock.
func (gl *GameLoop) applyFoodMagnet() {
	w := gl.world
	for _, snake := range w.Snakes {
		if !snake.Alive {
			continue
		}
		head := snake.Head()
		// Use spatial grid to find nearby food within MagnetRadius
		nearFoodIDs := w.Grid.NearbyFood(head.X, head.Y, MagnetRadius)
		for _, fid := range nearFoodIDs {
			food, ok := w.Food[fid]
			if !ok {
				continue
			}
			dx := head.X - food.X
			dy := head.Y - food.Y
			dist := math.Sqrt(dx*dx + dy*dy)
			// Already within eating radius — collectFood will handle it
			if dist <= SnakeHeadRadius+FoodRadius {
				continue
			}
			// Move food toward head by MagnetSpeed (don't overshoot)
			moveBy := MagnetSpeed
			if moveBy > dist {
				moveBy = dist
			}
			food.X += (dx / dist) * moveBy
			food.Y += (dy / dist) * moveBy
		}
	}
}

// detectCollisions checks head-to-body and head-to-head collisions.
// Returns map of victimID -> killerName.
func (gl *GameLoop) detectCollisions() map[string]string {
	w := gl.world
	deaths := map[string]string{}

	// Collect alive snakes for head-to-head check
	aliveSnakes := make([]*Snake, 0, len(w.Snakes))
	for _, s := range w.Snakes {
		if s.Alive {
			aliveSnakes = append(aliveSnakes, s)
		}
	}

	for _, snake := range aliveSnakes {
		if _, dead := deaths[snake.ID]; dead {
			continue
		}
		head := snake.Head()

		// Head vs body of other snakes
		nearby := w.Grid.NearbySnakeBody(head.X, head.Y, CollisionCheckRadius, snake.ID)
		for _, entry := range nearby {
			other := w.Snakes[entry.snakeID]
			if other == nil || !other.Alive {
				continue
			}
			dist := math.Sqrt(
				(head.X-entry.x)*(head.X-entry.x) +
					(head.Y-entry.y)*(head.Y-entry.y),
			)
			if dist < SnakeHeadRadius+SnakeBodyRadius {
				if _, alreadyDead := deaths[snake.ID]; !alreadyDead {
					deaths[snake.ID] = other.Name
				}
			}
		}
	}

	// Head-to-head: check all pairs
	for i := 0; i < len(aliveSnakes); i++ {
		for j := i + 1; j < len(aliveSnakes); j++ {
			a := aliveSnakes[i]
			b := aliveSnakes[j]
			if _, dead := deaths[a.ID]; dead {
				continue
			}
			if _, dead := deaths[b.ID]; dead {
				continue
			}
			ha := a.Head()
			hb := b.Head()
			dx := ha.X - hb.X
			dy := ha.Y - hb.Y
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < SnakeHeadRadius*2 {
				// Smaller snake dies; if equal both die
				if a.Score >= b.Score {
					deaths[b.ID] = a.Name
				}
				if b.Score >= a.Score {
					deaths[a.ID] = b.Name
				}
			}
		}
	}

	return deaths
}

// collectFood checks each alive snake head for food within eating radius and consumes it.
// Caller must hold w.mu.Lock.
func (gl *GameLoop) collectFood() {
	w := gl.world
	for _, snake := range w.Snakes {
		if !snake.Alive {
			continue
		}
		head := snake.Head()
		nearFoodIDs := w.Grid.NearbyFood(head.X, head.Y, SnakeHeadRadius+FoodRadius)
		for _, fid := range nearFoodIDs {
			food, ok := w.Food[fid]
			if !ok {
				continue
			}
			w.RemoveFood(fid)
			snake.Grow(food.Value)
		}
	}
}

// broadcast sends viewport-culled state to each connected player.
func (gl *GameLoop) broadcast(leaderboard []LeaderboardEntry) {
	w := gl.world
	conns := gl.conns.Snapshot()

	for _, c := range conns {
		w.mu.RLock()
		snake, hasSnake := w.Snakes[c.ID]

		var cx, cy float64
		if hasSnake && snake.Alive {
			head := snake.Head()
			cx, cy = head.X, head.Y
		} else {
			// Send empty state for dead/unjoined players
			w.mu.RUnlock()
			_ = c.Send(StateMsg{
				Type:        MsgState,
				Snakes:      []SnakeDTO{},
				Food:        []FoodDTO{},
				Leaderboard: leaderboard,
			})
			continue
		}

		snakeDTOs := w.SnakesInViewport(cx, cy)
		foodDTOs := w.FoodInViewport(cx, cy)
		w.mu.RUnlock()

		msg := StateMsg{
			Type:        MsgState,
			Snakes:      snakeDTOs,
			Food:        foodDTOs,
			Leaderboard: leaderboard,
		}
		if err := c.Send(msg); err != nil {
			log.Printf("send error to %s: %v", c.ID, err)
		}
	}
}

// randIntn is a helper to avoid direct rand.Intn calls in tests
var randIntn = rand.Intn
