// game-client.js — WebSocket connection, state management, interpolation, main game loop

import { Camera } from './camera.js';
import { GameRenderer } from './game-renderer.js';
import { InputHandler } from './input-handler.js';
import { UIManager } from './ui-manager.js';

const SERVER_TICK_MS = 50;       // 20Hz server tick — used for interpolation window
const RECONNECT_DELAY_MS = 2000;
const INPUT_HZ_MS = 50;          // 20Hz input send rate

export class GameClient {
  constructor() {
    this.canvas = document.getElementById('gameCanvas');
    this._resizeCanvas();

    this.camera = new Camera(this.canvas.width, this.canvas.height);
    this.renderer = new GameRenderer(this.canvas, this.camera);
    this.ui = new UIManager();
    this.input = new InputHandler(this.canvas);

    // Game state
    this.myId = null;
    this.playerName = 'Anonymous';
    this.alive = false;
    // Circular world: center=(worldRadius, worldRadius), radius=worldRadius
    this.worldRadius = 10500;

    // Interpolation: keep previous + current server state
    this._prevState = null;
    this._currState = null;
    this._lastStateTime = 0;  // timestamp when currState arrived

    // Loop
    this._rafId = null;
    this._lastFrameTime = 0;
    this._inputAccum = 0;

    // WebSocket
    this._ws = null;
    this._wsReady = false;
    this._reconnectTimer = null;
    this._intentionallyClosed = false;

    this._bindEvents();
  }

  start() {
    this.ui.showJoinScreen();
    this._connect();
    this._startLoop();
  }

  // ── WebSocket ─────────────────────────────────────────────────────────────

  _connect() {
    if (this._ws) {
      this._intentionallyClosed = true;
      this._ws.close();
    }

    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${proto}//${window.location.host}/ws`;

    try {
      this._ws = new WebSocket(url);
    } catch (e) {
      console.error('WebSocket construction failed:', e);
      this._scheduleReconnect();
      return;
    }

    this._intentionallyClosed = false;

    this._ws.addEventListener('open', () => {
      this._wsReady = true;
      this.ui.setConnectionStatus(true);
      console.log('WebSocket connected');
    });

    this._ws.addEventListener('message', (ev) => {
      try {
        this._handleMessage(JSON.parse(ev.data));
      } catch (e) {
        console.warn('Failed to parse message:', ev.data, e);
      }
    });

    this._ws.addEventListener('close', () => {
      this._wsReady = false;
      this.ui.setConnectionStatus(false);
      if (!this._intentionallyClosed) {
        this._scheduleReconnect();
      }
    });

    this._ws.addEventListener('error', () => {
      // close event will fire after error — handle there
    });
  }

  _scheduleReconnect() {
    if (this._reconnectTimer) return;
    this._reconnectTimer = setTimeout(() => {
      this._reconnectTimer = null;
      this._connect();
    }, RECONNECT_DELAY_MS);
  }

  _send(obj) {
    if (this._ws && this._wsReady && this._ws.readyState === WebSocket.OPEN) {
      this._ws.send(JSON.stringify(obj));
    }
  }

  // ── Message handling ──────────────────────────────────────────────────────

  _handleMessage(msg) {
    // Feature 7: Binary protocol — use single-char keys (msg.t = type)
    switch (msg.t) {
      case 'w':
        this._onWelcome(msg);
        break;
      case 's':
        this._onState(msg);
        break;
      case 'd':
        this._onDeath(msg);
        break;
      default:
        console.warn('Unknown message type:', msg.t);
    }
  }

  _onWelcome(msg) {
    // Feature 7: msg.i=id, msg.r=worldRadius, msg.c=color
    this.myId = msg.i;
    this.worldRadius = msg.r || 10500;
    this.renderer.setWorldRadius(this.worldRadius);
    console.log('Connected as', this.myId);
  }

  _onState(msg) {
    // Feature 7: msg.s=snakes, msg.f=food, msg.l=leaderboard
    // Snake segments arrive as [[x,y],[x,y]] arrays — convert to {x,y} objects
    const snakes = (msg.s || []).map(s => ({
      id: s.i,
      name: s.n,
      color: s.c,
      score: s.p,
      boosting: s.b === 1,
      width: s.w || 10,
      segments: (s.s || []).map(seg => ({ x: seg[0], y: seg[1] })),
    }));

    // Food: f.l=level, f.m=isMoving
    const food = (msg.f || []).map(f => ({
      id: f.i,
      x: f.x,
      y: f.y,
      value: f.v,
      color: f.c,
      level: f.l || 1,
      isMoving: f.m === 1,
    }));

    // Leaderboard: e.i=id, e.n=name, e.p=score
    const leaderboard = (msg.l || []).map(e => ({
      id: e.i,
      name: e.n,
      score: e.p,
    }));

    // Minimap dots: all alive snakes in world (head pos + color + width)
    const minimap = (msg.m || []).map(d => ({
      x: d.x, y: d.y, color: d.c, width: d.w || 10,
    }));

    this._prevState = this._currState;
    this._currState = { snakes, food, leaderboard, minimap };
    this._lastStateTime = performance.now();

    // Attach color from snake data into leaderboard entries
    const snakeColorMap = {};
    for (const s of snakes) {
      snakeColorMap[s.id] = s.color;
    }
    const lbWithColor = leaderboard.map(e => ({
      ...e,
      color: snakeColorMap[e.id] || '#888',
    }));
    this.ui.updateLeaderboard(lbWithColor, this.myId);

    // Update score display
    if (this.myId && this.alive) {
      const me = snakes.find(s => s.id === this.myId);
      if (me) {
        this.ui.updateScore(me.score);
        if (me.segments && me.segments.length > 0) {
          const head = me.segments[0];
          this.camera.setTarget(head.x, head.y);
          if (!this._prevState) {
            this.camera.snapTo(head.x, head.y);
          }
        }
      }
    }
  }

  _onDeath(msg) {
    // Feature 7: msg.k=killer, msg.p=score
    this.alive = false;
    this.ui.showDeathScreen(msg.p, msg.k);
  }

  // ── Input ─────────────────────────────────────────────────────────────────

  _bindEvents() {
    // UI callbacks
    this.ui.onJoin((name) => {
      this.playerName = name;
      this.alive = true;
      this._prevState = null;
      this._currState = null;
      this.ui.showGame();
      // Feature 7: join uses {t:"j", n:name}
      this._send({ t: 'j', n: name });
    });

    this.ui.onRespawn((name) => {
      this.playerName = name;
      this.alive = true;
      this._prevState = null;
      this._currState = null;
      this.ui.showGame();
      // Feature 7: respawn uses {t:"r", n:name}
      this._send({ t: 'r', n: name });
    });

    // Input → server
    this.input.onInput(({ angle, boost }) => {
      if (this.alive && this._wsReady) {
        // Feature 7: input uses {t:"i", a:angle, b:boost?1:0}
        this._send({ t: 'i', a: angle, b: boost ? 1 : 0 });
      }
    });

    // Canvas resize
    window.addEventListener('resize', () => {
      this._resizeCanvas();
      this.camera.resize(this.canvas.width, this.canvas.height);
    });
  }

  // ── Render loop ───────────────────────────────────────────────────────────

  _startLoop() {
    const loop = (now) => {
      this._rafId = requestAnimationFrame(loop);
      const dt = now - (this._lastFrameTime || now);
      this._lastFrameTime = now;

      this.camera.update(dt);

      // Compute interpolation alpha between prev and curr server states
      const timeSinceState = now - this._lastStateTime;
      const alpha = Math.min(1, timeSinceState / SERVER_TICK_MS);

      // Build render state — interpolate snakes
      const renderState = {
        prev: this._prevState ? this._prevState.snakes : null,
        curr: this._currState ? this._currState.snakes : [],
        food: this._currState ? this._currState.food : [],
        minimap: this._currState ? this._currState.minimap : [],
      };

      this.renderer.render(renderState, this.myId, alpha, now);

      // Throttled input tick (ensures 20Hz even without mouse move)
      this._inputAccum += dt;
      if (this._inputAccum >= INPUT_HZ_MS) {
        this._inputAccum = 0;
        this.input.tick();
      }
    };

    this._rafId = requestAnimationFrame(loop);
  }

  _resizeCanvas() {
    this.canvas.width = window.innerWidth;
    this.canvas.height = window.innerHeight;
  }

  destroy() {
    if (this._rafId) cancelAnimationFrame(this._rafId);
    this._intentionallyClosed = true;
    if (this._ws) this._ws.close();
    if (this._reconnectTimer) clearTimeout(this._reconnectTimer);
    this.input.destroy();
  }
}
