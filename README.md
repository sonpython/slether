# Slether

A high-performance [Slither.io](http://slither.io) clone written in Go + vanilla HTML5 Canvas.

Single binary server, zero dependencies, handles **5,000+ concurrent players** on **1 vCPU / 512MB RAM**.

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-green)

## Features

- **Circular world** — 21,000px diameter arena with boundary death
- **50 AI bots** — multilingual names, priority-based AI (flee, chase, seek food, wander)
- **Boost mechanic** — spend body length for speed, drops colored food trail
- **Multi-level food** — common (L1), medium (L3), death drops (L3), rare moving food (L10)
- **Magnetic food attraction** — food pulls toward snake head, scales with width
- **Snake width growth** — eating increases width with diminishing returns
- **Neon boost glow** — 2-pass rendering with glow layer behind body
- **Slither.io-style body** — alternating light/dark bands with ridge grooves
- **Minimap** — proportional snake body rendering, filtered by visibility
- **Leaderboard** — top 10, transparent overlay
- **Viewport culling** — server only sends visible snakes/food per player
- **Spatial hash grid** — O(1) collision and proximity queries
- **Compact JSON protocol** — single-char keys, coordinate rounding
- **Per-message WebSocket compression** — RFC 7692 deflate
- **Rate limiting** — 8,000 max connections, 30s per-IP cooldown

## Performance

| Metric | Value |
|--------|-------|
| Server tick rate | 20 Hz |
| Client render | 60 FPS |
| Binary size | ~9 MB |
| Memory (50 bots, 12.5K food) | ~40 MB |
| CPU (50 bots idle) | <2% single core |
| Estimated 5K CCU | 1 vCPU, 512 MB RAM |
| Max connections | 8,000 (configurable) |

## Quick Start

```bash
# Clone
git clone https://github.com/sonpython/slether.git
cd slether

# Build and run
cd server
go build -o slether-server .
./slether-server

# Open browser
open http://localhost:8080
```

## Docker

```bash
# Build
docker build -t slether .

# Run
docker run -d -p 8080:8080 --name slether slether

# Or with docker compose
docker compose up -d
```

## Project Structure

```
slether/
├── server/                 # Go game server
│   ├── main.go             # HTTP/WebSocket server, rate limiting
│   ├── game_loop.go        # Fixed-timestep game loop (20 Hz)
│   ├── world.go            # Game state, viewport culling, minimap
│   ├── snake.go            # Snake physics, growth, boost, collision
│   ├── food.go             # Food spawning, clusters, moving food
│   ├── bot.go              # AI bot system (50 bots, priority-based)
│   ├── spatial_grid.go     # Spatial hash grid for O(1) queries
│   ├── connection.go       # WebSocket connection manager
│   ├── protocol.go         # Wire protocol DTOs
│   └── config.go           # All game constants
├── client/                 # Vanilla HTML5 Canvas client
│   ├── index.html
│   ├── game-client.js      # WebSocket, interpolation, game loop
│   ├── game-renderer.js    # Canvas 2D rendering, minimap, effects
│   ├── camera.js           # Smooth camera with lerp
│   ├── input-handler.js    # Mouse/touch input
│   ├── ui-manager.js       # Join/death screens, leaderboard
│   └── style.css
├── Dockerfile
├── docker-compose.yml
└── .github/workflows/
    └── deploy.yml          # CI/CD for self-hosted runner
```

## Configuration

All game constants are in [`server/config.go`](server/config.go). Key settings:

| Constant | Default | Description |
|----------|---------|-------------|
| `ServerPort` | `:8080` | HTTP listen port |
| `WorldRadius` | `10500` | Circular world radius (px) |
| `TickRate` | `20` | Server updates per second |
| `BotCount` | `50` | Number of AI bots |
| `InitialFoodCount` | `12500` | Food items in world |
| `MaxPlayers` | `8000` | Max WebSocket connections |
| `IPCooldownSec` | `30` | Seconds between connections per IP |

## Architecture

- **Server-authoritative** — all game logic runs server-side
- **Client interpolation** — smooth 60fps rendering between 20Hz server ticks
- **Spatial hash grid** — partitions world into 200px cells for fast proximity queries
- **Viewport culling** — each player only receives data for their visible area
- **Zero external dependencies** — just `gorilla/websocket` and `google/uuid`

## Deploy with Cloudflare Tunnel

```bash
# Install cloudflared
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o /usr/local/bin/cloudflared
chmod +x /usr/local/bin/cloudflared

# Create tunnel
cloudflared tunnel login
cloudflared tunnel create slether
cloudflared tunnel route dns slether yourdomain.com

# Config: /etc/cloudflared/config.yml
# tunnel: <ID>
# credentials-file: /root/.cloudflared/<ID>.json
# ingress:
#   - hostname: yourdomain.com
#     service: http://localhost:8080
#   - service: http_status:404
```

## License

MIT
