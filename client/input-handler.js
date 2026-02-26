// input-handler.js — Mouse/touch tracking, angle calculation, boost state, throttled input sending

export class InputHandler {
  constructor(canvas) {
    this.canvas = canvas;
    this.angle = 0;       // radians, direction snake should move
    this.boost = false;
    this.mouseX = 0;      // screen coords relative to canvas
    this.mouseY = 0;
    this._sendCallback = null;
    this._lastSendTime = 0;
    this._sendIntervalMs = 50; // 20 Hz
    this._bound = {};

    // Feature 2: Touch indicator element for mobile
    this._touchIndicator = document.getElementById('touchIndicator');

    this._attach();
  }

  // Register a callback that receives { angle, boost } at 20Hz
  onInput(fn) {
    this._sendCallback = fn;
  }

  _attach() {
    const canvas = this.canvas;

    this._bound.mouseMove = (e) => {
      const rect = canvas.getBoundingClientRect();
      this.mouseX = e.clientX - rect.left;
      this.mouseY = e.clientY - rect.top;
      this._updateAngle();
      this._trySend();
    };

    this._bound.mouseDown = (e) => {
      if (e.button === 0) {
        this.boost = true;
        this._trySend(true);
      }
    };

    this._bound.mouseUp = (e) => {
      if (e.button === 0) {
        this.boost = false;
        this._trySend(true);
      }
    };

    this._bound.keyDown = (e) => {
      if (e.code === 'Space' && !e.repeat) {
        e.preventDefault();
        this.boost = true;
        this._trySend(true);
      }
    };

    this._bound.keyUp = (e) => {
      if (e.code === 'Space') {
        this.boost = false;
        this._trySend(true);
      }
    };

    // Touch: direction from canvas center to touch point
    this._bound.touchStart = (e) => {
      e.preventDefault();
      this._handleTouch(e.touches[0]);
      // Double tap to boost is handled via touchend gap timing
      const now = Date.now();
      if (now - (this._lastTapTime || 0) < 280) {
        this.boost = true;
      }
      this._lastTapTime = now;
      // Feature 2: show touch indicator at touch position
      this._showTouchIndicator(e.touches[0]);
      this._trySend(true);
    };

    this._bound.touchMove = (e) => {
      e.preventDefault();
      this._handleTouch(e.touches[0]);
      // Feature 2: update touch indicator position
      this._showTouchIndicator(e.touches[0]);
      this._trySend();
    };

    this._bound.touchEnd = (e) => {
      e.preventDefault();
      this.boost = false;
      // Feature 2: hide touch indicator
      this._hideTouchIndicator();
      this._trySend(true);
    };

    canvas.addEventListener('mousemove', this._bound.mouseMove);
    canvas.addEventListener('mousedown', this._bound.mouseDown);
    canvas.addEventListener('mouseup', this._bound.mouseUp);
    window.addEventListener('keydown', this._bound.keyDown);
    window.addEventListener('keyup', this._bound.keyUp);
    canvas.addEventListener('touchstart', this._bound.touchStart, { passive: false });
    canvas.addEventListener('touchmove', this._bound.touchMove, { passive: false });
    canvas.addEventListener('touchend', this._bound.touchEnd, { passive: false });
  }

  _handleTouch(touch) {
    const rect = this.canvas.getBoundingClientRect();
    this.mouseX = touch.clientX - rect.left;
    this.mouseY = touch.clientY - rect.top;
    this._updateAngle();
  }

  _updateAngle() {
    const cx = this.canvas.width * 0.5;
    const cy = this.canvas.height * 0.5;
    const dx = this.mouseX - cx;
    const dy = this.mouseY - cy;
    // Only update angle if mouse is not dead center
    if (Math.abs(dx) > 2 || Math.abs(dy) > 2) {
      this.angle = Math.atan2(dy, dx);
    }
  }

  // Force-send or throttled send
  _trySend(force = false) {
    if (!this._sendCallback) return;
    const now = Date.now();
    if (force || now - this._lastSendTime >= this._sendIntervalMs) {
      this._lastSendTime = now;
      this._sendCallback({ angle: this.angle, boost: this.boost });
    }
  }

  // Tick called from game loop to ensure we always send at ~20Hz even without mouse movement
  tick() {
    this._trySend();
  }

  // Feature 2: Position and show the mobile touch indicator dot
  _showTouchIndicator(touch) {
    if (!this._touchIndicator) return;
    const size = 20;
    this._touchIndicator.style.left = `${touch.clientX - size / 2}px`;
    this._touchIndicator.style.top = `${touch.clientY - size / 2}px`;
    this._touchIndicator.style.display = 'block';
  }

  // Feature 2: Hide the mobile touch indicator dot
  _hideTouchIndicator() {
    if (!this._touchIndicator) return;
    this._touchIndicator.style.display = 'none';
  }

  destroy() {
    const canvas = this.canvas;
    canvas.removeEventListener('mousemove', this._bound.mouseMove);
    canvas.removeEventListener('mousedown', this._bound.mouseDown);
    canvas.removeEventListener('mouseup', this._bound.mouseUp);
    window.removeEventListener('keydown', this._bound.keyDown);
    window.removeEventListener('keyup', this._bound.keyUp);
    canvas.removeEventListener('touchstart', this._bound.touchStart);
    canvas.removeEventListener('touchmove', this._bound.touchMove);
    canvas.removeEventListener('touchend', this._bound.touchEnd);
  }
}
