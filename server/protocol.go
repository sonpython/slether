package main

// Protocol uses single-character JSON keys to minimize wire size.
// All x,y coordinates are rounded to 1 decimal place.
//
// Message type constants (value of "t" field):
//   Client → Server:
//     "j" = join    {"t":"j","n":"PlayerName"}
//     "i" = input   {"t":"i","a":1.57,"b":1}   (a=angle radians, b=boost 0/1)
//     "r" = respawn {"t":"r","n":"PlayerName"}
//   Server → Client:
//     "w" = welcome {"t":"w","i":"id","r":10500,"c":"#color"}  (r=world radius)
//     "s" = state   {"t":"s","s":[snakes],"f":[food],"l":[leaderboard]}
//     "d" = death   {"t":"d","k":"KillerName","p":score}
//
// SnakeDTO: {"i":"id","n":"name","s":[[x,y],...],"c":"#color","p":score}
// FoodDTO:  {"i":"id","x":1.0,"y":2.0,"v":1,"c":"#f00","l":1,"m":0}
//   l=level (1/3/5/10), m=isMoving (0/1)
// LeaderboardEntry: {"i":"id","n":"name","p":score}

// Message type identifiers — single-char for compact protocol
const (
	MsgJoin    = "j"
	MsgInput   = "i"
	MsgRespawn = "r"
	MsgWelcome = "w"
	MsgState   = "s"
	MsgDeath   = "d"
)

// ClientMessage is the base incoming message from the browser.
// Uses single-char keys matching the compact protocol.
//   {"t":"j","n":"name"}          join / respawn
//   {"t":"i","a":1.57,"b":1}      input (a=angle, b=boost)
type ClientMessage struct {
	Type  string  `json:"t"`
	Name  string  `json:"n,omitempty"`
	Angle float64 `json:"a,omitempty"`
	Boost int     `json:"b,omitempty"` // 0 or 1 (client sends int, not bool)
}

// WelcomeMsg is sent to a player immediately on WebSocket connect.
// r = world radius (circular map, center is always WorldCenterX/Y = 10500,10500)
// {"t":"w","i":"uuid","r":10500,"c":"#hexcolor"}
type WelcomeMsg struct {
	Type        string  `json:"t"`
	ID          string  `json:"i"`
	WorldRadius float64 `json:"r"`
	Color       string  `json:"c"`
}

// SnakeDTO is the compact snake for per-tick state updates.
// Segments are encoded as flat [x,y] float64 pairs to save bytes vs {"x":..,"y":..} objects.
// {"i":"id","n":"name","s":[[x,y],[x,y]],"c":"#color","p":score}
type SnakeDTO struct {
	ID       string       `json:"i"`
	Name     string       `json:"n"`
	Segments [][2]float64 `json:"s"`
	Color    string       `json:"c"`
	Score    int          `json:"p"`
	Boosting int          `json:"b,omitempty"` // 1 if boosting, omitted if not
	Width    float64      `json:"w"`           // visual radius
}

// FoodDTO is the compact food item for per-tick state updates.
// l = level (1/3/5/10), m = isMoving (0 or 1 integer for JSON compactness)
// {"i":"id","x":1.0,"y":2.0,"v":1,"c":"#f00","l":1,"m":0}
type FoodDTO struct {
	ID       string  `json:"i"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Value    int     `json:"v"`
	Color    string  `json:"c"`
	Level    int     `json:"l"`
	IsMoving int     `json:"m"` // 0 or 1
}

// LeaderboardEntry is a single leaderboard row.
// {"i":"id","n":"name","p":score}
type LeaderboardEntry struct {
	ID    string `json:"i"`
	Name  string `json:"n"`
	Score int    `json:"p"`
}

// MinimapDot is a lightweight snake position for the minimap.
// {"x":1.0,"y":2.0,"c":"#fff","w":10}
type MinimapDot struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Color string  `json:"c"`
	Width float64 `json:"w"` // snake width for proportional dot size
}

// StateMsg is the per-tick state update sent to each client.
// {"t":"s","s":[snakes],"f":[food],"l":[leaderboard],"m":[minimap dots]}
type StateMsg struct {
	Type        string             `json:"t"`
	Snakes      []SnakeDTO         `json:"s"`
	Food        []FoodDTO          `json:"f"`
	Leaderboard []LeaderboardEntry `json:"l"`
	Minimap     []MinimapDot       `json:"m,omitempty"`
}

// DeathMsg is sent to a player when their snake dies.
// k = killer name (or "Boundary"), p = final score
// {"t":"d","k":"KillerName","p":42}
type DeathMsg struct {
	Type   string `json:"t"`
	Killer string `json:"k"`
	Score  int    `json:"p"`
}
