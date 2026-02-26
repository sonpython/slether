package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ipRateLimiter tracks last connection time per IP to prevent abuse
type ipRateLimiter struct {
	mu    sync.Mutex
	times map[string]time.Time
}

func newIPRateLimiter() *ipRateLimiter {
	rl := &ipRateLimiter{times: make(map[string]time.Time)}
	// Cleanup stale entries every 60s
	go func() {
		for range time.Tick(60 * time.Second) {
			rl.mu.Lock()
			cutoff := time.Now().Add(-time.Duration(IPCooldownSec) * time.Second)
			for ip, t := range rl.times {
				if t.Before(cutoff) {
					delete(rl.times, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

// allow returns true if this IP can connect, and records the attempt
func (rl *ipRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if last, ok := rl.times[ip]; ok {
		if time.Since(last) < time.Duration(IPCooldownSec)*time.Second {
			return false
		}
	}
	rl.times[ip] = time.Now()
	return true
}

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
	rateLimiter := newIPRateLimiter()

	// WebSocket handler
	http.HandleFunc(WebSocketPath, func(w http.ResponseWriter, r *http.Request) {
		// Extract client IP (handle X-Forwarded-For for reverse proxies)
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip, _, _ = net.SplitHostPort(r.RemoteAddr)
		}

		// Max player cap
		if conns.Count() >= MaxPlayers {
			http.Error(w, "server full", http.StatusServiceUnavailable)
			return
		}

		// IP rate limit (30s cooldown)
		if !rateLimiter.allow(ip) {
			http.Error(w, "too many connections, wait 30s", http.StatusTooManyRequests)
			return
		}

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
	staticDir := StaticDir
	if env := os.Getenv("SLETHER_STATIC_DIR"); env != "" {
		staticDir = env
	}
	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/", fs)

	// Start game loop in background
	go loop.Run()

	log.Printf("server listening on %s (circular world r=%.0f)", ServerPort, WorldRadius)
	if err := http.ListenAndServe(ServerPort, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
