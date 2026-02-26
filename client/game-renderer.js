// game-renderer.js — Canvas rendering: grid background, food, snakes, minimap, circular world boundary

const GRID_SIZE = 60;           // world-space grid cell size in px
const SEGMENT_RADIUS = 10;      // body segment draw radius
const HEAD_RADIUS = 10;         // head draw radius (same as body)
const MINIMAP_SIZE = 160;       // minimap is a circle with this diameter
const MINIMAP_MARGIN = 16;

// Food level → base radius mapping
const FOOD_RADIUS = { 1: 4, 3: 7, 5: 10, 10: 14 };
// Food level → neon pulse params { frequency, minBlur, maxBlur, minAlpha, maxAlpha }
const FOOD_PULSE = {
  1:  { frequency: 2, minBlur: 3,  maxBlur: 10,  minAlpha: 0.75, maxAlpha: 1.0 },
  3:  { frequency: 3, minBlur: 5,  maxBlur: 16,  minAlpha: 0.7,  maxAlpha: 1.0 },
  5:  { frequency: 4, minBlur: 8,  maxBlur: 24,  minAlpha: 0.65, maxAlpha: 1.0 },
  10: { frequency: 6, minBlur: 12, maxBlur: 36,  minAlpha: 0.6,  maxAlpha: 1.0 },
};

// Max trail history kept per moving food id
const TRAIL_LENGTH = 4;

export class GameRenderer {
  constructor(canvas, camera) {
    this.canvas = canvas;
    this.ctx = canvas.getContext('2d');
    this.camera = camera;
    // Feature 1: Circular world — center=(worldRadius, worldRadius)
    this.worldRadius = 10500;

    // Feature 6: Track previous positions of moving food for trail rendering
    // Map<foodId, Array<{x,y}>>
    this._movingFoodTrails = new Map();
  }

  // Feature 1: Replace setWorldSize with setWorldRadius
  setWorldRadius(r) {
    this.worldRadius = r;
  }

  // Main render entry called every frame
  // now = performance.now() timestamp for neon pulse animation
  render(state, myId, alpha, now = 0) {
    this._now = now;
    const ctx = this.ctx;
    const W = this.canvas.width;
    const H = this.canvas.height;

    // Clear
    ctx.fillStyle = '#0d0d18';
    ctx.fillRect(0, 0, W, H);

    this._drawGrid();
    this._drawHazardZone();          // Feature 1: fading red ring hazard zone
    this._drawWorldBoundary();       // Feature 1: circular boundary
    this._drawFood(state.food, now); // Feature 3 & 6: multi-size + neon blink + trail
    this._drawSnakes(state.prev, state.curr, myId, alpha);
    this._drawMinimap(state.curr, myId);
  }

  // ── Grid ──────────────────────────────────────────────────────────────────

  _drawGrid() {
    const ctx = this.ctx;
    const cam = this.camera;
    const vp = cam.getViewport();
    const W = this.canvas.width;
    const H = this.canvas.height;

    ctx.save();
    ctx.strokeStyle = 'rgba(255,255,255,0.045)';
    ctx.lineWidth = 1;

    // Vertical lines
    const startX = Math.floor(vp.x / GRID_SIZE) * GRID_SIZE;
    for (let wx = startX; wx < vp.x + vp.width + GRID_SIZE; wx += GRID_SIZE) {
      const sx = cam.worldToScreen(wx, 0).x;
      ctx.beginPath();
      ctx.moveTo(sx, 0);
      ctx.lineTo(sx, H);
      ctx.stroke();
    }

    // Horizontal lines
    const startY = Math.floor(vp.y / GRID_SIZE) * GRID_SIZE;
    for (let wy = startY; wy < vp.y + vp.height + GRID_SIZE; wy += GRID_SIZE) {
      const sy = cam.worldToScreen(0, wy).y;
      ctx.beginPath();
      ctx.moveTo(0, sy);
      ctx.lineTo(W, sy);
      ctx.stroke();
    }

    ctx.restore();
  }

  // ── Feature 1: Hazard zone — fading red ring inside boundary edge ─────────

  _drawHazardZone() {
    const ctx = this.ctx;
    const cam = this.camera;
    const cx = this.worldRadius;
    const cy = this.worldRadius;
    const r = this.worldRadius;
    const hazardDepth = 200; // world-space px of hazard band

    const center = cam.worldToScreen(cx, cy);
    const edgePt = cam.worldToScreen(cx + r, cy);
    const screenR = edgePt.x - center.x;
    const innerR = screenR * ((r - hazardDepth) / r);

    if (screenR <= 0) return;

    ctx.save();
    // Radial gradient from transparent at innerR to red at outerR
    const grad = ctx.createRadialGradient(
      center.x, center.y, Math.max(0, innerR),
      center.x, center.y, screenR
    );
    grad.addColorStop(0, 'rgba(255,30,30,0)');
    grad.addColorStop(0.5, 'rgba(255,30,30,0.04)');
    grad.addColorStop(1, 'rgba(255,30,30,0.18)');

    ctx.beginPath();
    ctx.arc(center.x, center.y, screenR, 0, Math.PI * 2);
    ctx.fillStyle = grad;
    ctx.fill();
    ctx.restore();
  }

  // ── Feature 1: Circular world boundary ───────────────────────────────────

  _drawWorldBoundary() {
    const ctx = this.ctx;
    const cam = this.camera;
    const cx = this.worldRadius;
    const cy = this.worldRadius;
    const r = this.worldRadius;

    const center = cam.worldToScreen(cx, cy);
    const edgePt = cam.worldToScreen(cx + r, cy);
    const screenR = edgePt.x - center.x;

    if (screenR <= 0) return;

    ctx.save();
    ctx.strokeStyle = 'rgba(255, 80, 80, 0.75)';
    ctx.lineWidth = 4;
    ctx.shadowColor = '#ff3333';
    ctx.shadowBlur = 18;
    ctx.beginPath();
    ctx.arc(center.x, center.y, screenR, 0, Math.PI * 2);
    ctx.stroke();
    ctx.restore();
  }

  // ── Feature 3 & 6: Food with multi-size, neon blink, moving food trail ───

  _drawFood(foodList, now) {
    if (!foodList || foodList.length === 0) return;
    const ctx = this.ctx;
    const cam = this.camera;
    const timeSec = now / 1000;

    // Feature 6: Update trails for moving food before rendering
    this._updateMovingFoodTrails(foodList);

    ctx.save();
    for (const food of foodList) {
      const level = food.level || 1;
      const baseRadius = FOOD_RADIUS[level] || 4;

      if (!cam.isVisible(food.x, food.y, baseRadius + 40)) continue;

      // Feature 6: Draw trail for moving food first (behind the food)
      if (food.isMoving) {
        this._drawMovingFoodTrail(ctx, cam, food);
      }

      const s = cam.worldToScreen(food.x, food.y);
      const pulse = FOOD_PULSE[level] || FOOD_PULSE[1];

      // Unique phase offset per food: hash from id string or position
      const phaseOffset = this._foodPhaseOffset(food);

      // Smooth sine pulse: oscillates 0..1
      const sinVal = (Math.sin(timeSec * pulse.frequency * Math.PI * 2 + phaseOffset) + 1) * 0.5;
      const currentBlur = pulse.minBlur + (pulse.maxBlur - pulse.minBlur) * sinVal;
      const currentAlpha = pulse.minAlpha + (pulse.maxAlpha - pulse.minAlpha) * sinVal;
      const color = food.color || '#ff6b6b';

      ctx.globalAlpha = currentAlpha;
      ctx.shadowColor = color;
      ctx.shadowBlur = currentBlur;

      ctx.beginPath();
      ctx.arc(s.x, s.y, baseRadius, 0, Math.PI * 2);
      ctx.fillStyle = color;
      ctx.fill();

      // Extra inner bright core for higher level food
      if (level >= 5) {
        ctx.globalAlpha = 0.6 * currentAlpha;
        ctx.shadowBlur = 0;
        ctx.fillStyle = '#ffffff';
        ctx.beginPath();
        ctx.arc(s.x, s.y, baseRadius * 0.35, 0, Math.PI * 2);
        ctx.fill();
      }

      ctx.globalAlpha = 1;
      ctx.shadowBlur = 0;
    }
    ctx.restore();
  }

  // Returns a stable phase offset in radians derived from food id or position
  _foodPhaseOffset(food) {
    if (food.id) {
      let hash = 0;
      const str = String(food.id);
      for (let i = 0; i < str.length; i++) {
        hash = (hash * 31 + str.charCodeAt(i)) & 0xffffffff;
      }
      return (hash & 0xff) / 255 * Math.PI * 2;
    }
    // Fallback: position-based
    return ((food.x * 7 + food.y * 13) % 628) / 100;
  }

  // Feature 6: Update stored trail history for moving food
  _updateMovingFoodTrails(foodList) {
    const currentIds = new Set();
    for (const food of foodList) {
      if (!food.isMoving) continue;
      currentIds.add(food.id);
      const trail = this._movingFoodTrails.get(food.id) || [];
      // Only push if position actually changed (avoid duplicate entries)
      const last = trail[trail.length - 1];
      if (!last || last.x !== food.x || last.y !== food.y) {
        trail.push({ x: food.x, y: food.y });
        if (trail.length > TRAIL_LENGTH) trail.shift();
      }
      this._movingFoodTrails.set(food.id, trail);
    }
    // Clean up trails for food that no longer exists
    for (const id of this._movingFoodTrails.keys()) {
      if (!currentIds.has(id)) this._movingFoodTrails.delete(id);
    }
  }

  // Feature 6: Draw fading trail circles behind moving food
  _drawMovingFoodTrail(ctx, cam, food) {
    const trail = this._movingFoodTrails.get(food.id);
    if (!trail || trail.length < 2) return;

    const baseRadius = FOOD_RADIUS[10];
    const color = food.color || '#ff6b6b';

    // Render from oldest to newest (behind the food dot)
    for (let i = 0; i < trail.length - 1; i++) {
      const t = (i + 1) / trail.length; // 0 = oldest, ~1 = newest
      const alpha = t * 0.35;
      const r = baseRadius * (0.3 + t * 0.5);
      const pos = cam.worldToScreen(trail[i].x, trail[i].y);

      ctx.globalAlpha = alpha;
      ctx.shadowColor = color;
      ctx.shadowBlur = 8 * t;
      ctx.fillStyle = color;
      ctx.beginPath();
      ctx.arc(pos.x, pos.y, r, 0, Math.PI * 2);
      ctx.fill();
    }
    ctx.globalAlpha = 1;
    ctx.shadowBlur = 0;
  }

  // ── Snakes ────────────────────────────────────────────────────────────────

  _drawSnakes(prevSnakes, currSnakes, myId, alpha) {
    if (!currSnakes) return;

    // Build prev lookup for interpolation
    const prevMap = {};
    if (prevSnakes) {
      for (const s of prevSnakes) prevMap[s.id] = s;
    }

    // Draw other snakes first, then player on top
    const others = currSnakes.filter(s => s.id !== myId);
    const me = currSnakes.find(s => s.id === myId);

    for (const snake of others) {
      this._drawSnake(snake, prevMap[snake.id], alpha, false);
    }
    if (me) {
      this._drawSnake(me, prevMap[me.id], alpha, true);
    }
  }

  _lerpSegments(prev, curr, alpha) {
    if (!prev || !prev.segments || prev.segments.length === 0) return curr.segments;
    const out = [];
    const len = Math.min(prev.segments.length, curr.segments.length);
    for (let i = 0; i < len; i++) {
      out.push({
        x: prev.segments[i].x + (curr.segments[i].x - prev.segments[i].x) * alpha,
        y: prev.segments[i].y + (curr.segments[i].y - prev.segments[i].y) * alpha,
      });
    }
    // Append extra segments from curr (snake grew)
    for (let i = len; i < curr.segments.length; i++) {
      out.push(curr.segments[i]);
    }
    return out;
  }

  _drawSnake(snake, prev, alpha, isMe) {
    const cam = this.camera;
    const segments = this._lerpSegments(prev, snake, alpha);
    if (!segments || segments.length === 0) return;

    const ctx = this.ctx;
    const color = snake.color || '#4fc3f7';
    const boosting = snake.boosting;
    const r = snake.width || SEGMENT_RADIUS; // server-driven width

    // Culling: skip if neither head nor tail visible
    const head = segments[0];
    const tailCheck = segments[segments.length - 1];
    if (!cam.isVisible(head.x, head.y, r + 40) && !cam.isVisible(tailCheck.x, tailCheck.y, r + 40)) return;

    ctx.save();

    // Draw body segments (tail → head so head overlaps)
    for (let i = segments.length - 1; i >= 1; i--) {
      const seg = segments[i];
      if (!cam.isVisible(seg.x, seg.y, r + 8)) continue;
      const s = cam.worldToScreen(seg.x, seg.y);

      const segAlpha = 0.85 + (isMe ? 0.15 : 0);

      // Boost: neon glow + pulsing ripple ring around each segment
      if (boosting) {
        const pulse = (Math.sin((this._now || 0) / 80 + i * 0.5) + 1) * 0.5;
        ctx.shadowColor = color;
        ctx.shadowBlur = 24 + pulse * 12;
        ctx.beginPath();
        ctx.arc(s.x, s.y, r + 2 + pulse * 4, 0, Math.PI * 2);
        ctx.strokeStyle = this._alphaColor(color, 0.3 + pulse * 0.4);
        ctx.lineWidth = 2;
        ctx.stroke();
        ctx.shadowBlur = 18;
      } else {
        ctx.shadowColor = 'transparent';
        ctx.shadowBlur = 0;
      }

      // Base segment circle
      ctx.beginPath();
      ctx.arc(s.x, s.y, r, 0, Math.PI * 2);
      ctx.fillStyle = this._alphaColor(color, segAlpha);
      ctx.fill();

      // Crescent scale pattern
      this._drawSegmentCrescent(ctx, s, r, color, i, segments);
    }

    // Draw head (same width as body)
    this._drawHead(ctx, cam, segments, color, isMe, snake.name, boosting, r);

    ctx.restore();
  }

  // Stacked crescent pattern — two overlapping crescents per segment for clear distinction
  _drawSegmentCrescent(ctx, screenPos, radius, color, segIndex, segments) {
    if (segIndex >= segments.length - 1) return;

    // Direction from this segment to previous (toward head) for crescent orientation
    const prev = segments[segIndex - 1] || segments[0];
    const cam = this.camera;
    const prevS = cam.worldToScreen(prev.x, prev.y);
    const angle = Math.atan2(prevS.y - screenPos.y, prevS.x - screenPos.x);

    const isAlt = (segIndex % 2) === 0;

    ctx.save();
    ctx.translate(screenPos.x, screenPos.y);
    ctx.rotate(angle);

    // Upper crescent — lighter arc near the "forward" edge
    const r1 = radius * 0.75;
    ctx.beginPath();
    ctx.arc(radius * 0.15, 0, r1, -0.7, 0.7, false);
    ctx.arc(radius * 0.15, 0, r1 * 0.5, 0.7, -0.7, true);
    ctx.closePath();
    ctx.fillStyle = this._lightenColor(color, isAlt ? 0.5 : 0.3, isAlt ? 0.4 : 0.25);
    ctx.fill();

    // Lower crescent — darker accent for depth
    ctx.beginPath();
    ctx.arc(-radius * 0.1, 0, r1 * 0.6, -0.5, 0.5, false);
    ctx.arc(-radius * 0.1, 0, r1 * 0.25, 0.5, -0.5, true);
    ctx.closePath();
    ctx.fillStyle = this._alphaColor(color, isAlt ? 0.15 : 0.1);
    ctx.fill();

    ctx.restore();
  }

  _drawHead(ctx, cam, segments, color, isMe, name, boosting = false, width = HEAD_RADIUS) {
    const head = segments[0];
    const s = cam.worldToScreen(head.x, head.y);
    const hr = width; // head radius matches body width

    // Movement direction from head to next segment (reversed = facing forward)
    let angle = 0;
    if (segments.length >= 2) {
      const next = segments[1];
      angle = Math.atan2(head.y - next.y, head.x - next.x);
    }

    ctx.save();
    ctx.translate(s.x, s.y);
    ctx.rotate(angle);

    // Head circle — strong neon glow when boosting
    ctx.beginPath();
    ctx.arc(0, 0, hr, 0, Math.PI * 2);
    ctx.fillStyle = color;
    if (boosting) {
      ctx.shadowColor = color;
      ctx.shadowBlur = 32;
    } else {
      ctx.shadowColor = 'transparent';
      ctx.shadowBlur = 0;
    }
    ctx.fill();

    // Head scale highlight
    ctx.beginPath();
    ctx.arc(hr * 0.15, -hr * 0.2, hr * 0.65, 0.15 * Math.PI, 0.85 * Math.PI, false);
    ctx.closePath();
    ctx.fillStyle = this._lightenColor(color, 0.5, 0.35);
    ctx.fill();

    // Eyes — scale with head size
    const eyeScale = hr / 10; // normalize to base size
    const eyeOffset = hr * 0.45;
    const eyeForward = hr * 0.3;
    const eyeR = 3.5 * eyeScale;
    const pupilR = 2 * eyeScale;

    for (const sign of [-1, 1]) {
      // White of eye
      ctx.beginPath();
      ctx.arc(eyeForward, sign * eyeOffset, eyeR, 0, Math.PI * 2);
      ctx.fillStyle = '#ffffff';
      ctx.shadowBlur = 0;
      ctx.fill();
      // Pupil
      ctx.beginPath();
      ctx.arc(eyeForward + 1.2, sign * eyeOffset, pupilR, 0, Math.PI * 2);
      ctx.fillStyle = '#111';
      ctx.fill();
    }

    ctx.restore();

    // Name label above head
    if (name) {
      ctx.save();
      ctx.font = 'bold 11px -apple-system, sans-serif';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'bottom';
      const nameY = s.y - hr - 5;
      ctx.fillStyle = 'rgba(0,0,0,0.45)';
      const tw = ctx.measureText(name).width;
      ctx.fillRect(s.x - tw / 2 - 3, nameY - 13, tw + 6, 14);
      ctx.fillStyle = isMe ? '#a5d8ff' : '#ffffff';
      ctx.fillText(name, s.x, nameY);
      ctx.restore();
    }
  }

  // ── Feature 1: Minimap — circular world ──────────────────────────────────

  _drawMinimap(snakes, myId) {
    const ctx = this.ctx;
    const W = this.canvas.width;
    const H = this.canvas.height;
    const r = MINIMAP_SIZE / 2;
    const cx = W - r - MINIMAP_MARGIN;
    const cy = H - r - MINIMAP_MARGIN;
    const worldR = this.worldRadius;
    // Scale: minimap circle radius maps to world circle radius
    const scale = r / worldR;

    ctx.save();

    // Clip to circular minimap area
    ctx.beginPath();
    ctx.arc(cx, cy, r, 0, Math.PI * 2);
    ctx.clip();

    // Background
    ctx.fillStyle = 'rgba(8, 8, 20, 0.78)';
    ctx.fill();

    // World boundary circle on minimap
    ctx.strokeStyle = 'rgba(255,80,80,0.5)';
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.arc(cx, cy, r - 1, 0, Math.PI * 2);
    ctx.stroke();

    // Draw snake dots — positions are world coords, center=(worldR,worldR)
    if (snakes) {
      for (const snake of snakes) {
        if (!snake.segments || snake.segments.length === 0) continue;
        const head = snake.segments[0];
        // Convert world pos relative to world center, then to minimap
        const dx = head.x - worldR;
        const dy = head.y - worldR;
        const sx = cx + dx * scale;
        const sy = cy + dy * scale;
        const isMe = snake.id === myId;

        ctx.beginPath();
        ctx.arc(sx, sy, isMe ? 4 : 2.5, 0, Math.PI * 2);
        ctx.fillStyle = isMe ? '#ffffff' : (snake.color || '#888');
        if (isMe) {
          ctx.shadowColor = '#ffffff';
          ctx.shadowBlur = 6;
        }
        ctx.fill();
        ctx.shadowBlur = 0;
      }
    }

    // Restore clip, draw minimap border ring on top
    ctx.restore();

    ctx.save();
    ctx.strokeStyle = 'rgba(255,255,255,0.12)';
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.arc(cx, cy, r, 0, Math.PI * 2);
    ctx.stroke();

    // Player viewport indicator — approximate rectangle clipped to circle
    const cam = this.camera;
    const vp = cam.getViewport();
    const vpX = cx + (vp.x - worldR) * scale;
    const vpY = cy + (vp.y - worldR) * scale;
    const vpW = vp.width * scale;
    const vpH = vp.height * scale;
    ctx.strokeStyle = 'rgba(255,255,255,0.2)';
    ctx.lineWidth = 1;
    ctx.strokeRect(vpX, vpY, vpW, vpH);

    ctx.restore();
  }

  // ── Helpers ───────────────────────────────────────────────────────────────

  _alphaColor(hex, alpha) {
    const r = parseInt(hex.slice(1, 3), 16);
    const g = parseInt(hex.slice(3, 5), 16);
    const b = parseInt(hex.slice(5, 7), 16);
    return `rgba(${r},${g},${b},${Math.max(0, Math.min(1, alpha))})`;
  }

  // Returns a lighter version of a hex color blended toward white, with given alpha
  _lightenColor(hex, amount, alpha) {
    const r = parseInt(hex.slice(1, 3), 16);
    const g = parseInt(hex.slice(3, 5), 16);
    const b = parseInt(hex.slice(5, 7), 16);
    const lr = Math.round(r + (255 - r) * amount);
    const lg = Math.round(g + (255 - g) * amount);
    const lb = Math.round(b + (255 - b) * amount);
    return `rgba(${lr},${lg},${lb},${Math.max(0, Math.min(1, alpha))})`;
  }
}
