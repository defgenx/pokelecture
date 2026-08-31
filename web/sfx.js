'use strict';

/* Game sounds, synthesised on the fly with Web Audio — no asset files, nothing
   to download, and they never collide with the spoken voice because they are
   short and sit on a separate gain node.

   Sound is most of what makes a game feel like a game to a five-year-old: the
   rising chime on a right answer does more for motivation than any amount of
   text. */

const Sfx = {
  ctx: null,
  bus: null,
  muted: false,

  // Must be called from a user gesture (the COMMENCER tap).
  unlock() {
    if (this.ctx) return this.resume();
    const AC = window.AudioContext || window.webkitAudioContext;
    if (!AC) return;
    this.ctx = new AC();
    this.bus = this.ctx.createGain();
    this.bus.gain.value = 0.22; // well under the voice, which must stay clearest
    this.bus.connect(this.ctx.destination);
    this.resume();

    /* Android suspends the context whenever the page goes to the background —
       and on a tablet that happens constantly: the screen times out mid-game,
       the child switches app, the dock takes over. A suspended context never
       resumes on its own, so without these two hooks every game sound dies
       silently the first time the tablet sleeps. */
    document.addEventListener('visibilitychange', () => {
      if (!document.hidden) this.resume();
    });
    document.addEventListener('pointerdown', () => this.resume(), { capture: true });
  },

  resume() {
    if (!this.ctx || this.ctx.state === 'running') return;
    const p = this.ctx.resume();
    if (p && p.catch) p.catch(() => {});
  },

  // tone plays one shaped oscillator note.
  tone(freq, start, dur, { type = 'triangle', gain = 1, slideTo = null } = {}) {
    if (!this.ctx || this.muted) return;
    this.resume();
    const t0 = this.ctx.currentTime + start;
    const osc = this.ctx.createOscillator();
    const env = this.ctx.createGain();
    osc.type = type;
    osc.frequency.setValueAtTime(freq, t0);
    if (slideTo) osc.frequency.exponentialRampToValueAtTime(slideTo, t0 + dur);

    // Short attack, exponential tail: reads as "plucked" rather than "beep".
    env.gain.setValueAtTime(0.0001, t0);
    env.gain.exponentialRampToValueAtTime(gain, t0 + 0.012);
    env.gain.exponentialRampToValueAtTime(0.0001, t0 + dur);

    osc.connect(env).connect(this.bus);
    osc.start(t0);
    osc.stop(t0 + dur + 0.02);
  },

  // noise plays a filtered burst, for impacts and whooshes.
  noise(start, dur, { freq = 1200, q = 1, gain = 1, slideTo = null } = {}) {
    if (!this.ctx || this.muted) return;
    this.resume();
    const t0 = this.ctx.currentTime + start;
    const n = Math.floor(this.ctx.sampleRate * dur);
    const buf = this.ctx.createBuffer(1, n, this.ctx.sampleRate);
    const d = buf.getChannelData(0);
    for (let i = 0; i < n; i++) d[i] = (Math.random() * 2 - 1) * (1 - i / n);

    const src = this.ctx.createBufferSource();
    src.buffer = buf;
    const flt = this.ctx.createBiquadFilter();
    flt.type = 'bandpass';
    flt.frequency.setValueAtTime(freq, t0);
    flt.Q.value = q;
    if (slideTo) flt.frequency.exponentialRampToValueAtTime(slideTo, t0 + dur);
    const env = this.ctx.createGain();
    env.gain.value = gain;

    src.connect(flt).connect(env).connect(this.bus);
    src.start(t0);
  },

  tap()     { this.tone(440, 0, 0.07, { type: 'sine', gain: 0.5 }); },
  correct() { this.tone(660, 0, 0.11); this.tone(880, 0.09, 0.14); this.tone(1320, 0.19, 0.2, { gain: 0.7 }); },
  wrong()   { this.tone(200, 0, 0.16, { type: 'sawtooth', gain: 0.35, slideTo: 120 }); },
  merge()   { this.tone(330, 0, 0.1, { slideTo: 660 }); this.tone(880, 0.1, 0.16, { gain: 0.6 }); },
  attack()  { this.noise(0, 0.22, { freq: 2400, slideTo: 300, gain: 0.9 }); this.tone(140, 0.05, 0.2, { type: 'square', gain: 0.4, slideTo: 70 }); },
  faint()   { this.tone(520, 0, 0.5, { type: 'sine', gain: 0.5, slideTo: 90 }); },

  // A little arpeggio for the catch — the single most rewarding moment.
  catch_()  { [523, 659, 784, 1047].forEach((f, i) => this.tone(f, i * 0.11, 0.3, { gain: 0.8 })); },

  // Critical hit: the attack whoosh plus a bright double ping on top.
  crit() {
    this.attack();
    this.tone(1568, 0.12, 0.18, { gain: 0.9 });
    this.tone(2093, 0.24, 0.26, { gain: 0.8 });
  },

  // The foe's counterattack on a miss: a low thud, no melody — menace, not defeat.
  thud() {
    this.noise(0, 0.16, { freq: 240, q: 0.8, gain: 0.9, slideTo: 90 });
    this.tone(90, 0, 0.18, { type: 'square', gain: 0.35, slideTo: 55 });
  },

  // Card flip in the bonus memory game.
  flip() { this.tone(520, 0, 0.06, { type: 'sine', gain: 0.45, slideTo: 760 }); },

  // A sprite pops out of its hole in the chase game.
  pop() { this.tone(340, 0, 0.09, { type: 'sine', gain: 0.55, slideTo: 620 }); },

  // Badge ceremony: a solemn rising fourth then the shine.
  badge() {
    [[392, 0], [523, 0.16], [659, 0.32], [784, 0.44], [1047, 0.56]].forEach(([f, t]) =>
      this.tone(f, t, 0.4, { gain: 0.8 }));
  },

  fanfare() {
    [[523, 0], [523, 0.12], [784, 0.24], [1047, 0.42]].forEach(([f, t]) =>
      this.tone(f, t, 0.34, { gain: 0.85 }));
  },

  // Rumble under a legendary's entrance.
  legendary() {
    this.noise(0, 1.1, { freq: 90, q: 0.6, gain: 1 });
    this.tone(70, 0, 1.0, { type: 'sawtooth', gain: 0.5, slideTo: 45 });
    [392, 523, 659, 880].forEach((f, i) => this.tone(f, 0.45 + i * 0.1, 0.5, { gain: 0.6 }));
  },
};
