package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for development; tighten in production
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// Enable per-message deflate compression (RFC 7692)
	EnableCompression: true,
}

func main() {
	world := NewWorld()
	conns := NewConnManager()
	loop := NewGameLoop(world, conns)

	// WebSocket handler
	http.HandleFunc(WebSocketPath, func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("ws upgrade error: %v", err)
			return
		}

		// Enable per-message write compression at best-speed level
		ws.EnableWriteCompression(true)

		conn := NewConn(ws)
		conns.Add(conn)
		log.Printf("player connected: %s", conn.ID)

		// Send welcome immediately so client knows its ID and world dimensions
		_ = conn.Send(WelcomeMsg{
			Type:        MsgWelcome,
			ID:          conn.ID,
			WorldRadius: WorldRadius,
			Color:       randomColor(),
		})

		onJoin := func(c *Conn, name string) {
			world.mu.Lock()
			// Drop old snake if reconnecting / respawning
			if old, exists := world.Snakes[c.ID]; exists {
				if old.Alive {
					dropped := old.DropFood()
					world.AddFood(dropped)
				}
			}
			color := randomColor()
			snake := NewSnake(c.ID, name, color)
			world.AddSnake(snake)
			world.mu.Unlock()
			log.Printf("snake joined: %s (%s)", name, c.ID)
		}

		onDisconnect := func(c *Conn) {
			conns.Remove(c.ID)
			world.mu.Lock()
			if snake, exists := world.Snakes[c.ID]; exists {
				if snake.Alive {
					dropped := snake.DropFood()
					world.AddFood(dropped)
				}
				world.RemoveSnake(c.ID)
			}
			world.mu.Unlock()
			log.Printf("player disconnected: %s", c.ID)
		}

		// Blocking read loop — runs until client disconnects
		conn.ReadLoop(world, onJoin, onDisconnect)
	})

	// Serve static client files
	fs := http.FileServer(http.Dir(StaticDir))
	http.Handle("/", fs)

	// Start game loop in background
	go loop.Run()

	log.Printf("server listening on %s (circular world r=%.0f)", ServerPort, WorldRadius)
	if err := http.ListenAndServe(ServerPort, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
