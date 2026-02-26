// camera.js — Camera that follows player snake with smooth lerp and world-to-screen transforms

const ZOOM = 0.8; // <1 = zoomed in (shows less world), reduces lag by rendering fewer entities

export class Camera {
  constructor(canvasWidth, canvasHeight) {
    this.x = 0; // world position at center of screen
    this.y = 0;
    this.width = canvasWidth;
    this.height = canvasHeight;
    this.lerpFactor = 0.12;
    this.targetX = 0;
    this.targetY = 0;
    // Visible world dimensions (smaller than canvas due to zoom)
    this.viewW = canvasWidth * ZOOM;
    this.viewH = canvasHeight * ZOOM;
  }

  resize(w, h) {
    this.width = w;
    this.height = h;
    this.viewW = w * ZOOM;
    this.viewH = h * ZOOM;
  }

  setTarget(worldX, worldY) {
    this.targetX = worldX;
    this.targetY = worldY;
  }

  snapTo(worldX, worldY) {
    this.x = worldX;
    this.y = worldY;
    this.targetX = worldX;
    this.targetY = worldY;
  }

  update(dt) {
    const t = Math.min(1, this.lerpFactor * (dt / 16.67));
    this.x += (this.targetX - this.x) * t;
    this.y += (this.targetY - this.y) * t;
  }

  // Convert world coordinate to canvas screen coordinate (with zoom)
  worldToScreen(worldX, worldY) {
    const scale = this.width / this.viewW;
    return {
      x: (worldX - this.x) * scale + this.width * 0.5,
      y: (worldY - this.y) * scale + this.height * 0.5,
    };
  }

  screenToWorld(screenX, screenY) {
    const scale = this.viewW / this.width;
    return {
      x: (screenX - this.width * 0.5) * scale + this.x,
      y: (screenY - this.height * 0.5) * scale + this.y,
    };
  }

  // Returns the visible world rectangle
  getViewport() {
    return {
      x: this.x - this.viewW * 0.5,
      y: this.y - this.viewH * 0.5,
      width: this.viewW,
      height: this.viewH,
    };
  }

  // Check if a world point (with radius) is visible on screen
  isVisible(worldX, worldY, radius = 0) {
    const vp = this.getViewport();
    return (
      worldX + radius > vp.x &&
      worldX - radius < vp.x + vp.width &&
      worldY + radius > vp.y &&
      worldY - radius < vp.y + vp.height
    );
  }
}
