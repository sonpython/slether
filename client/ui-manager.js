// ui-manager.js — Join screen, death screen, leaderboard overlay, score display

export class UIManager {
  constructor() {
    this.joinScreen = document.getElementById('joinScreen');
    this.deathScreen = document.getElementById('deathScreen');
    this.leaderboard = document.getElementById('leaderboard');
    this.scoreDisplay = document.getElementById('scoreDisplay');
    this.connectionStatus = document.getElementById('connectionStatus');
    // Feature 2: Canvas reference for cursor class toggling
    this._canvas = document.getElementById('gameCanvas');

    this._nameInput = document.getElementById('nameInput');
    this._playBtn = document.getElementById('playBtn');
    this._respawnBtn = document.getElementById('respawnBtn');
    this._deathScoreEl = document.getElementById('deathScore');
    this._deathKillerEl = document.getElementById('deathKiller');
    this._scoreValueEl = document.getElementById('scoreValue');
    this._lbList = document.getElementById('lbList');
    this._connDot = document.getElementById('connDot');
    this._connLabel = document.getElementById('connLabel');

    this._onJoin = null;
    this._onRespawn = null;

    this._playBtn.addEventListener('click', () => this._handleJoin());
    this._respawnBtn.addEventListener('click', () => this._handleRespawn());
    this._nameInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') this._handleJoin();
    });

    // Load saved name
    const saved = localStorage.getItem('slether_name');
    if (saved) this._nameInput.value = saved;
  }

  // Callbacks
  onJoin(fn) { this._onJoin = fn; }
  onRespawn(fn) { this._onRespawn = fn; }

  _handleJoin() {
    const name = this._nameInput.value.trim() || 'Anonymous';
    localStorage.setItem('slether_name', name);
    if (this._onJoin) this._onJoin(name);
  }

  _handleRespawn() {
    const name = this._nameInput.value.trim() || 'Anonymous';
    if (this._onRespawn) this._onRespawn(name);
  }

  showJoinScreen() {
    this.joinScreen.classList.remove('hidden');
    this.deathScreen.classList.add('hidden');
    this.leaderboard.classList.add('hidden');
    this.scoreDisplay.classList.add('hidden');
    // Feature 2: default cursor on overlay screen
    this._canvas.classList.remove('gameplay');
    this._nameInput.focus();
  }

  showDeathScreen(score, killerName) {
    this._deathScoreEl.textContent = score;
    if (killerName) {
      this._deathKillerEl.innerHTML = `Killed by <span>${this._escape(killerName)}</span>`;
    } else {
      this._deathKillerEl.textContent = 'You ran into a wall';
    }
    this.deathScreen.classList.remove('hidden');
    this.leaderboard.classList.add('hidden');
    this.scoreDisplay.classList.add('hidden');
    // Feature 2: default cursor on overlay screen
    this._canvas.classList.remove('gameplay');
  }

  showGame() {
    this.joinScreen.classList.add('hidden');
    this.deathScreen.classList.add('hidden');
    this.leaderboard.classList.remove('hidden');
    this.scoreDisplay.classList.remove('hidden');
    // Feature 2: crosshair cursor during active gameplay
    this._canvas.classList.add('gameplay');
  }

  updateScore(score) {
    this._scoreValueEl.textContent = score;
  }

  // leaderboardEntries: [{id, name, score, color}], myId: string
  updateLeaderboard(entries, myId) {
    this._lbList.innerHTML = '';
    entries.forEach((entry, i) => {
      const li = document.createElement('li');
      if (entry.id === myId) li.classList.add('is-me');

      const rank = document.createElement('span');
      rank.className = 'rank';
      rank.textContent = i + 1;

      const dot = document.createElement('span');
      dot.className = 'lb-color';
      dot.style.background = entry.color || '#888';

      const nameEl = document.createElement('span');
      nameEl.className = 'lb-name';
      nameEl.textContent = entry.name;

      const scoreEl = document.createElement('span');
      scoreEl.className = 'lb-score';
      scoreEl.textContent = entry.score;

      li.appendChild(rank);
      li.appendChild(dot);
      li.appendChild(nameEl);
      li.appendChild(scoreEl);
      this._lbList.appendChild(li);
    });
  }

  showError(message) {
    // Show error on join screen with countdown
    this.showJoinScreen();
    this._connDot.classList.add('disconnected');
    this._connLabel.textContent = message;
    // Auto-clear after 30s
    let remaining = 30;
    const timer = setInterval(() => {
      remaining--;
      if (remaining <= 0) {
        clearInterval(timer);
        this._connLabel.textContent = 'Ready';
        this._connDot.classList.remove('disconnected');
        // Trigger reconnect
        window.dispatchEvent(new CustomEvent('slether-retry'));
      } else {
        this._connLabel.textContent = `Wait ${remaining}s...`;
      }
    }, 1000);
  }

  setConnectionStatus(connected) {
    if (connected) {
      this._connDot.classList.remove('disconnected');
      this._connLabel.textContent = 'Connected';
    } else {
      this._connDot.classList.add('disconnected');
      this._connLabel.textContent = 'Reconnecting...';
    }
  }

  _escape(str) {
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }
}
