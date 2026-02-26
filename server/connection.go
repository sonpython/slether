package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// PlayerInput holds the latest input from a client
type PlayerInput struct {
	Angle float64
	Boost bool
}

// Conn manages a single WebSocket player session
type Conn struct {
	ID     string
	Name   string
	ws     *websocket.Conn
	input  PlayerInput
	mu     sync.Mutex // protects input and ws writes
	closed bool
}

// NewConn creates a new connection wrapper
func NewConn(ws *websocket.Conn) *Conn {
	return &Conn{
		ID: uuid.New().String(),
		ws: ws,
	}
}

// Send serializes msg to JSON and writes it to the WebSocket
func (c *Conn) Send(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	return c.ws.WriteMessage(websocket.TextMessage, data)
}

// GetInput returns the current input snapshot
func (c *Conn) GetInput() PlayerInput {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.input
}

// setInput updates input under lock
func (c *Conn) setInput(angle float64, boost bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.input.Angle = angle
	c.input.Boost = boost
}

// Close marks connection closed
func (c *Conn) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.ws.Close()
}

// ConnManager manages all active connections
type ConnManager struct {
	mu    sync.RWMutex
	conns map[string]*Conn
}

// NewConnManager creates an empty connection manager
func NewConnManager() *ConnManager {
	return &ConnManager{conns: make(map[string]*Conn)}
}

// Add registers a connection
func (m *ConnManager) Add(c *Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conns[c.ID] = c
}

// Remove unregisters a connection
func (m *ConnManager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.conns, id)
}

// Get returns a connection by ID
func (m *ConnManager) Get(id string) (*Conn, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.conns[id]
	return c, ok
}

// Count returns the number of active connections
func (m *ConnManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.conns)
}

// Snapshot returns a copy of all current connections
func (m *ConnManager) Snapshot() []*Conn {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*Conn, 0, len(m.conns))
	for _, c := range m.conns {
		list = append(list, c)
	}
	return list
}

// ReadLoop handles incoming messages for a connection until it disconnects.
// Compact protocol: single-char "t" field for message type.
//   "j" = join, "i" = input, "r" = respawn
// onJoin is called when a join/respawn message is received.
// onDisconnect is called when the connection closes.
func (c *Conn) ReadLoop(
	world *World,
	onJoin func(conn *Conn, name string),
	onDisconnect func(conn *Conn),
) {
	defer func() {
		onDisconnect(c)
		c.Close()
	}()

	for {
		_, raw, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("ws read error for %s: %v", c.ID, err)
			}
			return
		}

		var msg ClientMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("bad message from %s: %v", c.ID, err)
			continue
		}

		switch msg.Type {
		case MsgJoin, MsgRespawn: // "j" or "r"
			name := msg.Name
			if name == "" {
				name = "Player"
			}
			c.Name = name
			onJoin(c, name)

		case MsgInput: // "i"
			c.setInput(msg.Angle, msg.Boost == 1)
		}
	}
}

// randomColor picks a random color from the palette
func randomColor() string {
	return PlayerColors[rand.Intn(len(PlayerColors))]
}
