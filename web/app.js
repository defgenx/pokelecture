'use strict';

/* Pokélecture — frontend.
   No build step and no framework on purpose: the whole point is that a content
   change at 22h is visible on the tablet at 22h01. */

const $ = (sel, root = document) => root.querySelector(sel);

function el(tag, attrs, ...kids) {
  const n = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (k === 'class') n.className = v;
    else if (k === 'html') n.innerHTML = v;
    else if (k.startsWith('on')) n.addEventListener(k.slice(2), v);
    else if (v !== null && v !== undefined && v !== false) n.setAttribute(k, v);
  }
  for (const kid of kids.flat()) {
    if (kid === null || kid === undefined || kid === false) continue;
    n.append(kid instanceof Node ? kid : document.createTextNode(String(kid)));
  }
  return n;
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function api(path, body) {
  const opts = body
    ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }
    : {};
  const res = await fetch(path, opts);
  if (!res.ok) {
    const err = new Error(`${path}: ${res.status} ${await res.text()}`);
    err.status = res.status; // 409 on a game call: no player picked on this device
    throw err;
  }
  return res.json();
}

/* ------------------------------------------------------------------ voice ---
   Every line is a pre-generated MP3 in one voice (ElevenLabs, Jeanne). A
   missing file plays nothing — never the browser's own voice. */

// A 40 ms silent WAV. Played inside the COMMENCER tap it grants the one audio
// element permission to play for the rest of the session — Android only trusts
// an element that was started from a real gesture.
const SILENCE = 'data:audio/wav;base64,UklGRjAAAABXQVZFZm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQwAAAAAAAAAAAAAAAAAAAA=';

const Voice = {
  el: null,
  missing: new Set(), // URLs the server does not have; never wait on them twice
  primed: new Set(),  // URLs already pushed into the HTTP cache
  leading: null, // the instruction being read out; other lines queue behind it
  gen: 0,        // bumped on every screen change, so a queued line never plays late

  // prime() warms the HTTP cache for the screen's audio before the child taps
  // anything: on a slow tablet the first play then starts instantly instead of
  // stalling mid-exercise.
  prime(urls) {
    for (const u of urls || []) {
      if (!u || this.primed.has(u) || this.missing.has(u)) continue;
      this.primed.add(u);
      fetch(u).then((r) => { if (!r.ok) this.missing.add(u); }).catch(() => {});
    }
  },

  unlock() {
    /* One element for the whole game, primed here. Android caps how many media
       players a page may hold, and the previous one-element-per-line cache blew
       past that cap on a long session; a single reused element also means the
       autoplay check is passed exactly once. */
    if (!this.el) {
      this.el = new Audio();
      this.el.preload = 'auto';
    }
    this.el.src = SILENCE;
    const p = this.el.play();
    if (p && p.catch) p.catch(() => {});
  },

  // stop() silences everything and drops queued lines: the screen changed.
  stop() {
    this.gen++;
    this.leading = null;
    this.halt();
  },

  // lead() reads a screen's instruction first: the child cannot read it, and a
  // word or syllable the screen plays meanwhile waits for it instead of
  // cutting it off.
  lead(url, text) {
    this.stop();
    const p = this.play(url, text);
    this.leading = p;
    p.finally(() => { if (this.leading === p) this.leading = null; });
    return p;
  },

  halt() {
    if (this.el) {
      try { this.el.pause(); } catch (_) {}
    }
  },

  play(url, text) {
    if (this.leading) {
      const gen = this.gen;
      return this.leading.then(() => (gen === this.gen ? this.play(url, text) : undefined));
    }
    this.halt();
    // No recorded track (or a missing one): straight to the browser voice, with
    // no 404 round trip in front of every single line.
    if (!url || !this.el || this.missing.has(url)) return this.speak(text);

    return new Promise((resolve) => {
      const a = this.el;
      let settled = false;
      const finish = (fallback) => {
        if (settled) return;
        settled = true;
        a.removeEventListener('ended', onEnd);
        a.removeEventListener('error', onErr);
        if (fallback) {
          this.missing.add(url);
          this.speak(text).then(resolve);
        } else resolve();
      };
      const onEnd = () => finish(false);
      // Only a load error means the clip is missing. A rejected play() is
      // usually the next line interrupting this one (AbortError): taking it
      // for a missing file used to silence a good clip for the whole session.
      const onErr = () => { if (a.src.endsWith(url)) finish(true); };

      a.addEventListener('ended', onEnd);
      a.addEventListener('error', onErr);
      a.src = url;
      const p = a.play();
      if (p && p.catch) p.catch(() => finish(false));
      // Safety net: a stalled element must not freeze the game. Scaled to the
      // text (every line here is a few seconds at most) — the old flat 12 s
      // was itself the freeze it was guarding against.
      setTimeout(() => finish(false), Math.min(2500 + (text || '').length * 90, 8000));
    });
  },

  // speak() is what a line without a clip gets: silence, and a warning. The
  // whole voice is pre-generated MP3 (one voice, Jeanne); the browser's own
  // synthesis is never used — it reads French syllables in another voice,
  // often an English one. The tests make sure no clip is missing.
  speak(text) {
    if (text) console.warn(`[voix] pas de clip pour « ${text} »`);
    return Promise.resolve();
  },
};

/* -------------------------------------------------------------------- fx --- */

function burst(symbol) {
  const fx = $('#fx');
  const b = el('div', { class: 'burst' }, symbol);
  fx.append(b);
  setTimeout(() => b.remove(), 800);
}

// A short confetti shower. Pure DOM: a handful of nodes that clean themselves up.
function confetti(n = 44) {
  const fx = $('#fx');
  const colours = ['#ee1515', '#3b4cca', '#ffcb05', '#63bb5b', '#ec8fe6', '#4d90d5'];
  for (let i = 0; i < n; i++) {
    const c = el('i', { class: 'confetti' });
    c.style.left = `${Math.random() * 100}%`;
    c.style.background = colours[i % colours.length];
    c.style.animationDelay = `${Math.random() * 0.35}s`;
    c.style.transform = `rotate(${Math.random() * 360}deg)`;
    fx.append(c);
    setTimeout(() => c.remove(), 2600);
  }
}

const PRAISE = ['Bravo !', 'Super !', 'Génial !', 'Bien joué !'];
let praiseIdx = 0;

// One picto per region on the maps; regions come from the episode data.
const REGION_EMOJI = {
  'Forêt Émeraude': '🌲',
  'Colline Azur': '⛰️',
  'Grotte Violette': '🦇',
  'Volcan Rougeoyant': '🌋',
  'Côte Saphir': '🌊',
  'Vallée Arc-en-ciel': '🌈',
  'Îles du Levant': '🏝',
  'Îles du Couchant': '🌅',
  'Plateau de la Ligue': '🏆',
  'Terres Nouvelles': '🧭',
  'Failles Ultra': '🌌',
};

// say() prefers the recorded phrase — from the session payload, or from the
// state payload outside a session (home, Pokédex) — and degrades to the browser
// voice when the audio track has not been generated yet.
function say(phrase) {
  const url = (Session.data && Session.data.ui && Session.data.ui[phrase])
    || (App.state && App.state.ui && App.state.ui[phrase])
    || null;
  return Voice.play(url, phrase);
}

// hint() is a screen's instruction: shown, read out as the screen opens, and
// tappable to hear again. Every literal passed here must be in UIPhrases
// (internal/curriculum/session.go) — speech_coverage_test.go enforces it.
function hint(text, speak = true) {
  const label = el('span', {}, text);
  const node = el('button', { class: 'hint hint-voice', onclick: () => say(label.textContent) }, '🔊 ', label);
  node.setText = (t) => { label.textContent = t; };
  if (speak && text) Voice.lead(uiAudio(text), text);
  return node;
}

function uiAudio(phrase) {
  return (Session.data && Session.data.ui && Session.data.ui[phrase])
    || (App.state && App.state.ui && App.state.ui[phrase])
    || null;
}

function praise() {
  burst(['⭐', '🎉', '👏', '💫'][praiseIdx % 4]);
  Sfx.correct();
  cheerCompanion();
  return say(PRAISE[praiseIdx++ % PRAISE.length]);
}

/* --------------------------------------------------------------- buddy ---
   His partner (world.js) follows him from screen to screen and jumps when he
   gets one right; before one is chosen, his latest catch, then Pikachu. */

function companionId() {
  const partner = App.state && App.state.partner;
  if (partner) return partner.species;
  const caught = ((App.state && App.state.pokedex) || []).filter((p) => p.caught);
  return caught.length ? caught[caught.length - 1].id : 25;
}

function companion(cls) {
  return el('img', { class: cls || 'mascot', src: `sprites/art/${companionId()}.png`, alt: '' });
}

function cheerCompanion() {
  document.querySelectorAll('.mascot, .ally').forEach((n) => {
    n.classList.remove('cheer');
    void n.offsetWidth;
    n.classList.add('cheer');
  });
}

function pokeball(extra) {
  return el('span', { class: `pokeball ${extra || ''}`.trim(), 'aria-hidden': 'true' });
}

/* Keep the tablet screen awake during a session: a child thinking about a word
   is exactly when the screen would time out. Secure contexts only (production
   HTTPS); over plain LAN http this is a silent no-op. */
const WakeLock = {
  sentinel: null,
  async on() {
    if (!('wakeLock' in navigator)) return;
    try { this.sentinel = await navigator.wakeLock.request('screen'); } catch (_) {}
  },
  off() {
    if (this.sentinel) {
      this.sentinel.release().catch(() => {});
      this.sentinel = null;
    }
  },
};
// The lock dies whenever the tablet sleeps or switches app: re-take it.
// Back on the map from another tab (the admin page), reread the save.
document.addEventListener('visibilitychange', () => {
  if (document.hidden) return;
  if (Session.data) WakeLock.on();
  else if (App.state && $('.ow-map')) App.home();
});

/* ------------------------------------------------------------- fullscreen ---
   The installed app is already fullscreen (manifest "display": "fullscreen");
   in a browser tab, the address bar and tabs eat the screen. Fullscreen is
   asked for on the COMMENCER tap (browsers only allow it from a gesture),
   and a button in the top bar toggles it. Leaving fullscreen is remembered on
   the device, so the game does not force it back at the next launch. iPhone
   Safari has no fullscreen for pages: the button simply does not show. */

const Fullscreen = {
  KEY: 'pk-fullscreen',

  supported() {
    const d = document.documentElement;
    return !!(d.requestFullscreen || d.webkitRequestFullscreen);
  },

  installed() {
    return window.matchMedia('(display-mode: fullscreen), (display-mode: standalone)').matches
      || window.navigator.standalone === true;
  },

  active() { return !!(document.fullscreenElement || document.webkitFullscreenElement); },

  wanted() {
    try { return localStorage.getItem(this.KEY) !== 'off'; } catch (_) { return true; }
  },

  remember(on) {
    try { localStorage.setItem(this.KEY, on ? 'on' : 'off'); } catch (_) {}
  },

  async enter() {
    const d = document.documentElement;
    try {
      if (d.requestFullscreen) await d.requestFullscreen({ navigationUI: 'hide' });
      else d.webkitRequestFullscreen();
    } catch (_) {}
  },

  async exit() {
    try {
      if (document.exitFullscreen) await document.exitFullscreen();
      else document.webkitExitFullscreen();
    } catch (_) {}
  },

  toggle() {
    const on = !this.active();
    this.remember(on);
    if (on) this.enter(); else this.exit();
  },

  // Four corners pointing out (enter) or in (leave): an SVG, because the
  // emoji for it is missing from older Android fonts.
  icon() {
    const out = 'M4 9V4h5M15 4h5v5M20 15v5h-5M9 20H4v-5';
    const inn = 'M9 4v5H4M20 9h-5V4M15 20v-5h5M4 15h5v5';
    return `<svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="${this.active() ? inn : out}"/></svg>`;
  },

  button() {
    if (!this.supported() || this.installed()) return null;
    return el('button', {
      class: 'btn-icon fs-btn', title: 'Plein écran', 'aria-label': 'Plein écran',
      html: this.icon(), onclick: () => this.toggle(),
    });
  },

  paint() { document.querySelectorAll('.fs-btn').forEach((b) => { b.innerHTML = this.icon(); }); },
};

// Android (and iPadOS) drop fullscreen whenever the game leaves the screen —
// another app, the screen locking — and only a tap may ask for it again. So
// on the way back, the first tap restores it, unless it was turned off.
document.addEventListener('visibilitychange', () => {
  if (document.hidden || !Fullscreen.supported() || Fullscreen.installed()) return;
  if (!Fullscreen.wanted() || Fullscreen.active() || !$('#gate').hidden) return;
  document.addEventListener('pointerdown', () => {
    if (Fullscreen.wanted() && !Fullscreen.active()) Fullscreen.enter();
  }, { once: true, capture: true });
});
document.addEventListener('fullscreenchange', () => Fullscreen.paint());
document.addEventListener('webkitfullscreenchange', () => Fullscreen.paint());
document.addEventListener('keydown', (e) => {
  if ((e.key === 'f' || e.key === 'F') && !e.metaKey && !e.ctrlKey && !e.altKey && Fullscreen.supported()) {
    Fullscreen.toggle();
  }
});

/* ------------------------------------------------------------------ shell --- */

const App = {
  state: null,

  async boot() {
    try {
      this.state = await api('api/state');
    } catch (err) {
      if (err.status === 409) return Players.pick();
      throw err;
    }
    Players.next();
  },

  screen(...children) {
    const app = $('#app');
    app.innerHTML = '';
    app.append(...children.flat());
    window.scrollTo(0, 0);
  },

  // The bar is the Pokédex shell: blue lens, three status lights, red plastic.
  topbar(...right) {
    return el('div', { class: 'topbar' },
      el('span', { class: 'lens', 'aria-hidden': 'true' }),
      el('span', { class: 'lights', 'aria-hidden': 'true' }, el('i'), el('i'), el('i')),
      el('span', { class: 'grow' }),
      ...right,
      Fullscreen.button(),
    );
  },

  /* ------------------------------------------------------------ home ------ */

  // home is the overworld (overworld.js): the map he walks on. The list of
  // routes, badges and buttons below is the menu (☰ on the map).
  //
  // The save is reread every time: the admin page (another tab, another
  // device) can reset or repair it, and a stale state here used to undo
  // nothing — the old partner and Poké Balls stayed on screen.
  async home() {
    Session.data = null; // no session running: the map is the resting state
    try {
      this.state = await api('api/state');
    } catch (err) {
      if (err.status === 409) return Players.pick(); // the player was removed
    }
    if (!this.state.partner) return Partner.choose();
    Overworld.open();
  },

  menu() {
    Voice.stop();
    WakeLock.off();
    // The welcome line is read once per launch, not on every return home.
    const greeted = this.greeted;
    this.greeted = true;
    const s = this.state;
    const next = s.episodes.find((e) => e.id === s.next_episode);
    const caught = s.pokedex.filter((p) => p.caught).length;
    const shinies = s.pokedex.filter((p) => p.shiny).length;

    this.screen(
      this.topbar(
        el('span', { class: 'chip' }, `⭐ ${s.stars}`),
        el('span', { class: 'chip' }, pokeball('pokeball-xs'), ` ${s.balls || 0}`),
        shinies > 0 ? el('span', { class: 'chip' }, `✨ ${shinies}`) : null,
        s.streak > 1 ? el('span', { class: 'chip' }, `🔥 ${s.streak}`) : null,
      ),
      el('div', { class: 'page' },
        el('div', { class: 'home-top' },
          Partner.card(),
          el('div', { class: 'act' },
            el('h2', {}, s.all_done ? '🏆 Tu es le Champion !' : next ? next.title : 'Bravo !'),
            el('button', { class: 'btn btn-huge', onclick: () => Session.start(s.next_episode) },
              next && next.done > 0 ? 'REJOUER ▶' : 'JOUER ▶'),
            hint('Touche JOUER pour continuer l\'aventure !', !greeted),
            el('div', { class: 'home-row' },
              el('button', { class: 'btn btn-ghost', onclick: () => this.worldMap() }, '🗺️ Carte'),
              el('button', { class: 'btn btn-ghost', onclick: () => this.pokedex() },
                `📘 Pokédex ${caught}/${s.pokedex.length}`),
              s.tower ? el('button', { class: 'btn btn-ghost', onclick: () => Session.start('tour') },
                `🗼 Tour · Étage ${s.tower_floor + 1}`) : null,
              s.tower ? el('button', { class: 'btn btn-ghost', onclick: () => Arcade.hub() },
                '🎮 Mini-jeux') : null,
              s.tower && s.wild && s.wild.total ? el('button', { class: 'btn btn-ghost btn-grass', onclick: () => Wild.enter() },
                `🌿 Hautes herbes ${s.wild.caught}/${s.wild.available}`) : null,
            ),
          ),
          this.badgeCase(),
        ),
        this.map(s),
        el('button', { class: 'btn btn-ghost switch-player', onclick: () => Players.pick() },
          s.players > 1 ? '👥 Changer de joueur' : '👥 Joueurs'),
        // For the parents: the admin asks for a Google sign-in anyway.
        el('a', { class: 'parent-link', href: 'admin' }, '🔧 Espace parents'),
      ),
    );
  },

  // The route list: episodes grouped by their data-declared region, with the
  // next episode glowing — a five-year-old should always see where the
  // adventure continues.
  map(s) {
    const box = el('div', { class: 'routes', style: 'margin-top:26px' });
    let current = null;
    s.episodes.forEach((e) => {
      if (e.region !== current) {
        current = e.region;
        box.append(el('div', { class: 'map-section' }, `${REGION_EMOJI[e.region] || '📍'} ${e.region}`));
      }
      box.append(this.routeCard(e, e.id === s.next_episode));
    });
    return box;
  },

  /* The world map: one themed card per region, its episodes as a winding trail
     of Poké Ball waypoints — cleared in colour, the next one glowing, the rest
     waiting in grey. Tap a waypoint to set off. */
  worldMap() {
    Voice.stop();
    const s = this.state;
    const regions = [];
    s.episodes.forEach((e) => {
      let r = regions[regions.length - 1];
      if (!r || r.name !== e.region) {
        r = { name: e.region, eps: [] };
        regions.push(r);
      }
      r.eps.push(e);
    });

    this.screen(
      this.topbar(el('button', { class: 'btn-icon', onclick: () => this.home() }, '✕')),
      el('div', { class: 'page' },
        el('h2', { style: 'text-align:center' }, '🗺️ Carte du monde'),
        el('div', { class: 'world' },
          regions.map((r, ri) => el('div', { class: `region theme-${ri % 6}` },
            el('div', { class: 'region-name' }, `${REGION_EMOJI[r.name] || '📍'} ${r.name}`),
            el('div', { class: 'trail' },
              r.eps.map((e) => {
                const isNext = e.id === s.next_episode;
                const node = el('button', {
                  class: `waypoint ${e.done > 0 ? 'done' : ''} ${isNext ? 'next' : ''} ${e.unlocked ? '' : 'locked'}`,
                  title: e.title,
                  onclick: () => (e.unlocked ? Session.start(e.id) : say('Termine la route précédente !')),
                },
                  pokeball('pokeball-sm'),
                  el('img', { class: 'wp-reward', src: `sprites/art/${e.reward_pokemon}.png`, alt: '', loading: 'lazy' }),
                  el('small', {}, e.legendary ? '★' : e.route || '★'),
                );
                return node;
              }),
            ),
          )),
        ),
      ),
    );
  },

  // The badge case: every arena badge and the champion trophy, greyed out while
  // still to be won — the walk-to-the-league progress meter, straight from the
  // real games' trainer card.
  badgeCase() {
    const badges = (this.state.badges || []);
    if (!badges.length) return null;
    const box = el('div', { class: 'badgecase' });
    badges.forEach((b) => {
      box.append(el('span', { class: 'badgeslot', title: b.name },
        badgeEl(b.id, b.done),
        el('small', {}, b.done ? b.name.replace(/^(Badge|Trophée du)\s+/, '') : '?'),
      ));
    });
    return box;
  },

  routeCard(e, isNext) {
    const stars = '★'.repeat(e.stars) + '☆'.repeat(3 - e.stars);
    const conseil = e.legendary && !e.badge;
    return el('button', {
      class: `route ${e.unlocked ? '' : 'locked'} ${e.legendary ? 'legend' : ''} ${isNext ? 'next' : ''}`,
      onclick: () => (e.unlocked ? Session.start(e.id) : say('Termine la route précédente !')),
    },
      el('span', { class: 'no' }, e.legendary ? '★' : e.route),
      el('img', { src: `sprites/art/${e.reward_pokemon}.png`, alt: '', loading: 'lazy' }),
      el('div', { class: 't' },
        el('b', {}, e.unlocked ? e.title
          : conseil ? 'Conseil des 4 — 🔒'
          : e.legendary ? 'Arène légendaire — 🔒'
          : `Route ${e.route} — 🔒`),
        el('div', { class: 'sounds' }, (e.sounds || []).map((g) => el('i', {}, g))),
      ),
      e.badge ? el('span', { class: 'route-badge' }, badgeEl(e.badge, e.done > 0)) : null,
      el('span', { class: 'stars' }, e.unlocked ? stars : '🔒'),
    );
  },

  /* --------------------------------------------------------- pokédex ------ */

  pokedex(tab) {
    Voice.stop();
    const s = this.state;
    const caught = s.pokedex.filter((p) => p.caught).length;
    const wild = s.wild || { caught: 0, total: 0 };
    const tabs = el('div', { class: 'dex-tabs' },
      el('button', { class: `dex-tab ${tab === 'wild' ? '' : 'on'}`, onclick: () => this.pokedex() },
        `🗺️ Aventure ${caught}/${s.pokedex.length}`),
      wild.total ? el('button', { class: `dex-tab ${tab === 'wild' ? 'on' : ''}`, onclick: () => this.pokedex('wild') },
        `🌿 Sauvages ${wild.caught}/${wild.total}`) : null,
    );
    if (tab === 'wild') {
      const grid = el('div', { class: 'dexgrid' }, el('div', { class: 'loading' }, pokeball('spin-fast')));
      this.screen(
        this.topbar(el('button', { class: 'btn-icon', onclick: () => this.home() }, '✕')),
        el('div', { class: 'page' }, el('h2', { style: 'text-align:center;color:#e3350d' }, 'Pokédex'), tabs, grid),
      );
      wildDex(grid);
      return;
    }
    this.screen(
      this.topbar(el('button', { class: 'btn-icon', onclick: () => this.home() }, '✕')),
      el('div', { class: 'page' },
        el('h2', { style: 'text-align:center;color:#e3350d' }, 'Pokédex'),
        tabs,
        el('div', { class: 'dexgrid' },
          this.state.pokedex.map((p) => el('button', {
            class: `dexcell ${p.caught ? '' : 'locked'}`,
            onclick: () => (p.caught ? this.pokemonDetail(p)
              : p.where ? Voice.play(p.where.audio, p.where.text) : say('Celui-là, tu ne l\'as pas encore attrapé !')),
          },
            el('img', { class: p.shiny ? 'shiny' : '', src: p.sprite, alt: '', loading: 'lazy' }),
            p.shiny ? el('span', { class: 'shinymark' }, '✨') : null,
            el('b', {}, p.name),
            !p.caught && p.where ? el('small', { class: 'dex-where' }, p.where.text.replace(/^Il t'attend /, '').replace(/ !$/, '')) : null,
          )),
        ),
      ),
    );
  },

  pokemonDetail(p) {
    this.screen(
      this.topbar(el('button', { class: 'btn-icon', onclick: () => this.pokedex() }, '✕')),
      el('div', { class: 'page' },
        el('div', { class: 'act reveal' },
          el('img', { class: `art ${p.shiny ? 'shiny' : ''}`, src: p.art, alt: '' }),
          p.shiny ? el('span', { class: 'shinymark' }, '✨ chromatique') : null,
          syllableWord(p.syllables, p.name, p.syllable_audio),
          el('div', { class: 'types' }, (p.types || []).map((t) => el('span', { 'data-type': t }, t))),
          el('div', { class: 'dex' }, p.dex),
          el('button', { class: 'btn btn-ghost', onclick: () => Voice.play(p.dex_audio, p.dex) }, '🔊 Réécouter'),
        ),
      ),
    );
    Voice.play(p.name_audio, p.name).then(() => Voice.play(p.dex_audio, p.dex));
  },
};

// ballsWon is the "+3 Poké Balls" line on a result screen.
function ballsWon(n) {
  if (!n) return null;
  const row = el('div', { class: 'ball-row small' });
  for (let i = 0; i < n; i++) row.append(pokeball('ball-pop'));
  return el('div', { class: 'balls-won' }, row, el('b', {}, `+${n} Poké Ball${n > 1 ? 's' : ''}`));
}

/* "Qui joue ?": each child taps their own partner — they cannot read the
   names yet, they know their Pokémon. The device then remembers the player. */
const Players = {
  next() {
    if (!App.state.named) return this.askName('name');
    if (!App.state.partner) return Partner.choose();
    Gift.maybe();
  },

  /* "Comment tu t'appelles ?" — said aloud; an adult types it on the
     tablet's keyboard. mode "name" names this device's unnamed player,
     "new" creates another player and makes it this device's. */
  askName(mode) {
    const input = el('input', {
      class: 'name-input', type: 'text', maxlength: '30', autocomplete: 'off',
      autocapitalize: 'words', spellcheck: 'false', 'aria-label': 'Prénom',
    });
    const go = el('button', { class: 'btn btn-huge btn-go', disabled: true }, 'C\'EST MOI ! ▶');
    input.addEventListener('input', () => { go.disabled = !input.value.trim(); });
    input.addEventListener('keydown', (e) => { if (e.key === 'Enter' && input.value.trim()) go.click(); });
    go.addEventListener('click', async () => {
      const name = input.value.trim();
      if (!name) return;
      go.disabled = true;
      try {
        if (mode === 'new') await api('api/players', { name });
        else await api('api/name', { name });
        App.state = await api('api/state');
      } catch (err) {
        go.disabled = false;
        return;
      }
      Sfx.catch_();
      burst('🎉');
      this.next();
    });
    App.screen(
      App.topbar(mode === 'new' ? el('button', { class: 'btn-icon', onclick: () => this.pick() }, '✕') : null),
      el('div', { class: 'page' },
        el('div', { class: 'act' },
          el('h2', {}, 'Comment tu t\'appelles ?'),
          hint('Comment tu t\'appelles ? Demande à un grand de t\'aider à l\'écrire.'),
          input, go,
        ),
      ),
    );
    setTimeout(() => input.focus(), 300);
  },

  async pick() {
    Voice.stop();
    let data;
    try { data = await api('api/players'); } catch (_) { return; }
    const grid = el('div', { class: 'player-grid' },
      data.players.map((p) => el('button', {
        class: `player-card ${p.id === data.current ? 'current' : ''}`,
        onclick: async () => {
          Sfx.pop();
          try { await api('api/players/select', { id: p.id }); } catch (_) { return; }
          App.state = await api('api/state');
          this.next();
        },
      },
        el('img', { src: `sprites/art/${p.avatar}.png`, alt: '' }),
        el('b', {}, p.name),
        p.trainer_level ? el('small', {}, `🎖 N.${p.trainer_level} · ⭐ ${p.stars}`) : null,
      )),
      el('button', { class: 'player-card new', onclick: () => this.askName('new') },
        el('span', { class: 'player-plus' }, '➕'), el('b', {}, 'Nouveau joueur')),
    );
    App.screen(App.topbar(), el('div', { class: 'page' },
      el('div', { class: 'act' }, el('h2', {}, 'Qui joue ?'), hint('Qui joue ? Touche ton Pokémon !'), grid)));
  },
};

/* progressPips shows "where am I in this exercise" as little filling Poké
   Balls — borrowed from every mobile learning app worth copying. */
function progressPips(i, n) {
  const box = el('div', { class: 'pips' });
  for (let k = 0; k < n; k++) box.append(el('i', { class: k < i ? 'got' : '' }));
  return box;
}

/* The combo streak: consecutive right answers heat up a 🔥 counter, with a
   little celebration every third — momentum made visible, the cheapest and
   most effective trick in the genre. */
function comboChip() {
  const chip = el('span', { class: 'chip combo', hidden: true });
  return chip;
}

function comboUp(chip) {
  Session.combo++;
  if (chip) {
    chip.hidden = Session.combo < 2;
    chip.textContent = `🔥 x${Session.combo}`;
    chip.classList.remove('pop');
    void chip.offsetWidth;
    chip.classList.add('pop');
  }
  if (Session.combo > 0 && Session.combo % 3 === 0) {
    confetti(20);
    burst('🔥');
  }
}

function comboBreak(chip) {
  Session.combo = 0;
  if (chip) chip.hidden = true;
}

/* Renders a word as coloured syllables, each one tappable to hear it alone —
   the help affordance for a word he stalls on. */
function syllableWord(syllables, fallbackText, audio) {
  const parts = (syllables && syllables.length) ? syllables : [fallbackText];
  const box = el('div', { class: 'word' });
  parts.forEach((syl, n) => {
    box.append(el('span', {
      class: 'syl',
      onclick: (ev) => { ev.stopPropagation(); Voice.play(audio ? audio[n] : null, syl); },
    }, syl));
  });
  return box;
}

/* ---------------------------------------------------------------- session --- */

const Session = {
  data: null,
  i: 0,
  answers: [],
  right: 0,
  wrong: 0,
  shiny: false, // this session's reward was rolled shiny
  fightStats: null, // {perfect, rounds} from the last combat, for tower scoring
  combo: 0, // consecutive correct answers across the drill screens

  async start(id) {
    Voice.stop();
    App.screen(el('div', { class: 'loading' }, pokeball('spin-fast')));
    try {
      this.data = await api(`api/session/${id}`);
    } catch (err) {
      App.home();
      return;
    }
    this.i = 0;
    this.answers = [];
    this.right = 0;
    this.wrong = 0;
    this.shiny = false;
    this.fightStats = null;
    this.combo = 0;
    WakeLock.on();
    this.render();
  },

  record(id, correct) {
    if (!id) return;
    this.answers.push({ id, correct });
    if (correct) this.right++; else this.wrong++;
  },

  next() {
    this.i++;
    if (this.i >= this.data.activities.length) this.finish();
    else this.render();
  },

  dots() {
    const d = el('div', { class: 'dots' });
    this.data.activities.forEach((_, n) => {
      d.append(el('i', { class: n < this.i ? 'on' : n === this.i ? 'now' : '' }));
    });
    return d;
  },

  // Every audio URL an activity can play, for prefetching.
  audioUrls(a) {
    const urls = [a.audio];
    const item = (it) => {
      urls.push(it.audio, ...(it.syllable_audio || []));
      (it.choices || []).forEach((c) => urls.push(c.audio));
    };
    (a.items || []).forEach(item);
    (a.pairs || []).forEach((p) => urls.push(p.left_audio, p.right_audio, p.audio));
    if (a.grapheme) urls.push(a.grapheme.sound_audio, a.grapheme.mnemo_audio, a.grapheme.example_audio);
    (a.graphemes || []).forEach((g) => urls.push(g.sound_audio, g.mnemo_audio, g.example_audio));
    if (a.fight) { urls.push(a.fight.taunt_audio); a.fight.rounds.forEach(item); }
    if (a.reward) urls.push(a.reward.name_audio, a.reward.dex_audio);
    if (a.badge) urls.push(a.badge.audio);
    return urls.filter(Boolean);
  },

  render() {
    Voice.stop();
    const a = this.data.activities[this.i];
    // Warm the cache for this screen and the next, so no tap waits on the network.
    Voice.prime(this.audioUrls(a));
    const upcoming = this.data.activities[this.i + 1];
    if (upcoming) Voice.prime(this.audioUrls(upcoming));
    const body = el('div', { class: 'act' });

    App.screen(
      App.topbar(
        this.dots(),
        el('span', { class: 'grow' }),
        el('button', { class: 'btn-icon', onclick: () => App.home() }, '✕'),
      ),
      el('div', { class: 'page' }, body),
    );

    // The buddy tags along on the drill screens; the battle, the catch and the
    // badge ceremony have their own staging and would only be cluttered by it.
    if (!['combat', 'recompense', 'story', 'badge'].includes(a.kind)) {
      $('#app').append(companion());
    }

    const done = () => this.next();
    switch (a.kind) {
      case 'story': return renderStory(a, body, done);
      case 'decouverte': return renderDecouverte(a, body, done);
      case 'galerie': return renderGalerie(a, body, done);
      case 'fusion': return renderFusion(a, body, done);
      case 'combat': return renderCombat(a, body, done);
      case 'recompense': return renderReward(a, body, done);
      case 'badge': return renderBadge(a, body, done);
      case 'phrase': return renderPhrase(a, body, done);
      default: return renderQuiz(a, body, done);
    }
  },

  async finish() {
    const total = this.right + this.wrong;
    const ratio = total ? this.right / total : 1;
    const stars = ratio >= 0.9 ? 3 : ratio >= 0.7 ? 2 : 1;
    const towerInfo = this.data.tower;

    // The tower is practice with a ladder: its answers feed the review boxes,
    // its points decide whether the floor is cleared — but it never completes
    // an episode and never awards stars.
    const stats = this.fightStats || { perfect: 0, rounds: 0 };
    const points = stats.rounds + stats.perfect;
    const res = await api(`api/session/${this.data.episode_id}/result`, {
      items: this.answers,
      stars: towerInfo ? 0 : stars,
      completed: !towerInfo,
      shiny: this.shiny,
      floor: towerInfo ? towerInfo.floor : 0,
      rounds: stats.rounds,
      points,
    });
    App.state = res.state;

    if (this.data.trainer) {
      const won = res.trainer_won;
      App.screen(
        App.topbar(),
        el('div', { class: 'page' },
          el('div', { class: 'act' },
            el('h2', {}, won ? '🏆 Tu as battu le Dresseur !' : '⚔️ Combat de Dresseur'),
            ballsWon(res.balls_won),
            Partner.gainRow(res.partner_event),
            el('button', { class: 'btn btn-huge btn-go', onclick: () => App.home() }, 'CARTE 🗺️'),
          ),
        ),
      );
      if (won) { burst('🏆'); Sfx.fanfare(); confetti(80); await say('Tu as battu le Dresseur ! Voici ta récompense.'); }
      await celebrateAll(res);
      return;
    }

    if (towerInfo) {
      const passed = res.tower_passed;
      const catches = res.new_catches || [];
      App.screen(
        App.topbar(),
        el('div', { class: 'page' },
          el('div', { class: 'act' },
            el('h2', {}, passed ? `🗼 ÉTAGE ${towerInfo.floor} TERMINÉ !` : `🗼 Étage ${towerInfo.floor}`),
            el('div', { class: 'score' }, `${points} point${points > 1 ? 's' : ''} sur ${res.tower_need} demandés`),
            passed
              ? el('div', { class: 'score' }, `L'étage ${towerInfo.floor + 1} t'attend !`)
              : hint('Il faut lire au moins la moitié des mots du premier coup. Réessaie !'),
            catches.length ? el('div', { class: 'newcatch' },
              el('span', { class: 'score' }, '🎉 Un Pokémon rare de la Tour rejoint ton Pokédex !'),
              el('div', { class: 'catchrow' },
                catches.map((id) => el('img', { src: `sprites/art/${id}.png`, alt: '' }))),
            ) : null,
            res.new_shiny ? el('div', { class: 'newcatch' },
              el('span', { class: 'score' }, '✨ Ton légendaire devient chromatique !'),
              el('div', { class: 'catchrow' }, el('img', { class: 'shiny', src: `sprites/art/${res.new_shiny}.png`, alt: '' })),
            ) : null,
            ballsWon(res.balls_won),
            Partner.gainRow(res.partner_event),
            el('button', { class: 'btn btn-huge', onclick: () => Session.start('tour') },
              passed ? `ÉTAGE ${towerInfo.floor + 1} ▶` : 'RÉESSAYER 🔄'),
            el('button', { class: 'btn btn-go', onclick: () => App.home() }, 'CARTE 🗺️'),
          ),
        ),
      );
      burst(passed ? '🏅' : '🗼');
      if (passed) { Sfx.fanfare(); confetti(); } else { Sfx.wrong(); }
      await say(passed ? 'Super !' : 'Essaie encore, tu y es presque.');
      if (catches.length) await say('Un Pokémon rare de la Tour rejoint ton Pokédex !');
      if (res.new_shiny) { confetti(120); await say('Incroyable ! Ton Pokémon légendaire devient chromatique !'); }
      if (res.balls_won) await say('Tu gagnes des Poké Balls !');
      await celebrateAll(res);
      return;
    }

    const catches = res.new_catches || [];
    App.screen(
      App.topbar(),
      el('div', { class: 'page' },
        el('div', { class: 'act' },
          el('div', { class: 'bigstars' }, '★'.repeat(stars) + '☆'.repeat(3 - stars)),
          el('div', { class: 'score' }, `${this.right} bonne${this.right > 1 ? 's' : ''} réponse${this.right > 1 ? 's' : ''} sur ${total}`),
          catches.length ? el('div', { class: 'newcatch' },
            el('span', { class: 'score' }, catches.length > 1
              ? `🎉 ${catches.length} nouveaux Pokémon dans le Pokédex !`
              : '🎉 Nouveau Pokémon dans le Pokédex !'),
            el('div', { class: 'catchrow' },
              catches.map((id) => el('img', { src: `sprites/art/${id}.png`, alt: '' }))),
          ) : null,
          ballsWon(res.balls_won),
          Partner.gainRow(res.partner_event),
          el('button', { class: 'btn btn-huge btn-go', onclick: () => App.home() }, 'CARTE 🗺️'),
          el('button', { class: 'btn', onclick: () => Arcade.hub() }, '🎮 MINI-JEUX'),
        ),
      ),
    );
    burst('🏆');
    Sfx.fanfare();
    confetti();
    await say(stars === 3 ? 'Super !' : 'Bravo !');
    if (catches.length) await say('Tu as attrapé un nouveau Pokémon !');
    if (res.balls_won) await say('Tu gagnes des Poké Balls !');
    await celebrateAll(res);
    await say('À demain !');
  },
};

/* ------------------------------------------------------------- activities --- */

function renderStory(a, body, done) {
  body.append(
    el('h2', {}, a.title),
    el('div', { class: 'dex', style: 'max-width:34ch' }, a.text),
    el('button', { class: 'btn btn-huge btn-go', onclick: done }, "C'EST PARTI ▶"),
  );
  Voice.play(a.audio, a.text);
}

function renderDecouverte(a, body, done) {
  const g = a.grapheme;
  const letter = el('button', {
    class: 'letter',
    onclick: () => { letter.classList.remove('ping'); void letter.offsetWidth; letter.classList.add('ping'); Voice.play(g.sound_audio, g.display); },
  }, g.display);

  body.append(
    el('h2', {}, a.title),
    letter,
    hint(a.instruction),
    g.example ? el('button', {
      class: 'example',
      onclick: () => Voice.play(g.example_audio, g.example),
    },
      g.example_sprite ? el('img', { class: 'em-sprite', src: g.example_sprite, alt: '' })
        : g.example_icon ? iconEl(g.example_icon, 'em-icon')
        : el('span', { class: 'em' }, g.example_emoji || '🔊'),
      g.example,
    ) : null,
    g.mnemo ? el('button', { class: 'btn btn-ghost', onclick: () => Voice.play(g.mnemo_audio, g.mnemo) }, '💡 Astuce') : null,
    el('button', { class: 'btn btn-go', onclick: done }, 'SUIVANT →'),
  );
  Voice.play(g.sound_audio, g.display);
}

// renderGalerie is the fast review sweep: every sound must be touched once
// before the exit button lights up, which keeps it active instead of skippable.
function renderGalerie(a, body, done) {
  const remaining = new Set(a.graphemes.map((g) => g.id));
  const go = el('button', { class: 'btn btn-go', disabled: true, onclick: done }, 'SUIVANT →');
  const counter = hint(a.instruction);

  const grid = el('div', { class: 'grid' });
  a.graphemes.forEach((g) => {
    const tile = el('button', { class: 'tile' }, g.display);
    tile.addEventListener('click', () => {
      Voice.play(g.sound_audio, g.display);
      tile.classList.add('good');
      remaining.delete(g.id);
      if (remaining.size === 0) {
        go.disabled = false;
        counter.setText('Bravo ! Tu peux continuer.');
        say('Bravo ! Tu peux continuer.');
      } else {
        counter.setText(`Encore ${remaining.size} son${remaining.size > 1 ? 's' : ''} à toucher.`);
      }
    });
    grid.append(tile);
  });

  body.append(el('h2', {}, a.title), counter, grid, go);
}

function renderFusion(a, body, done) {
  let idx = 0;
  const stage = el('div', { class: 'fusion-row' });
  body.append(el('h2', {}, a.title), hint(a.instruction), stage);

  const step = () => {
    if (idx >= a.pairs.length) return done();
    const p = a.pairs[idx];
    let lit = 0;
    stage.innerHTML = '';

    const tile = (txt, audio) => {
      const b = el('button', { class: 'fusion-tile' }, txt);
      b.addEventListener('click', () => {
        if (b.classList.contains('lit')) return;
        b.classList.add('lit');
        Voice.play(audio, txt);
        if (++lit === 2) setTimeout(merge, 600);
      });
      return b;
    };

    const merge = async () => {
      stage.innerHTML = '';
      stage.append(el('div', { class: 'fusion-result' }, p.syllable));
      await Voice.play(p.audio, p.syllable);
      await sleep(350);
      idx++;
      step();
    };

    stage.append(
      tile(p.left, p.left_audio),
      el('span', { class: 'fusion-plus' }, '+'),
      tile(p.right, p.right_audio),
    );
  };
  step();
}

/* renderQuiz drives revision / ecoute / lecture / phrase. Two shapes:
   - a syllable item has no picture: the child hears it and points at it;
   - a word or sentence item has one (emoji OR Pokémon sprite) and shows its
     text: the child decodes it and points at the matching picture. The audio
     only unlocks after he answers, so it stays a reading task, never a
     listening one — getting this test wrong silently inverts the exercise. */
function renderQuiz(a, body, done) {
  let idx = 0;
  const head = el('h2', {}, a.title);
  const hintEl = hint(a.instruction);
  const combo = comboChip();
  const pipsBox = el('div', { class: 'pips-holder' });
  const stage = el('div', { class: 'act', style: 'width:100%' });
  body.append(head, hintEl, el('div', { class: 'meter-row' }, pipsBox, combo), stage);

  const step = () => {
    pipsBox.innerHTML = '';
    pipsBox.append(progressPips(idx, a.items.length));
    if (idx >= a.items.length) return done();
    const item = a.items[idx];
    // Icon counts as a picture too — without it, an icon-only word ("Morsure")
    // silently turned the reading exercise into a listening one.
    const isText = !!(item.emoji || item.sprite || item.icon) || a.kind === 'phrase';
    let answered = false;

    stage.innerHTML = '';

    let replay;
    if (isText) {
      const prompt = a.kind === 'phrase'
        ? el('div', { class: 'sentence' }, item.prompt)
        : syllableWord(item.syllables, item.prompt, item.syllable_audio);
      replay = el('button', { class: 'btn btn-ghost', disabled: true, onclick: () => Voice.play(item.audio, item.prompt) }, '🔊 Vérifier');
      stage.append(prompt, replay);
    } else {
      const listen = el('button', { class: 'btn btn-huge', onclick: () => Voice.play(item.audio, item.prompt) }, '🔊 ÉCOUTE');
      stage.append(listen);
      Voice.play(item.audio, item.prompt);
    }

    const grid = el('div', { class: `grid ${item.choices.length === 2 ? 'two' : item.choices.length >= 4 ? 'four' : ''}` });
    item.choices.forEach((c) => {
      // Pokémon → sprite, abstract Pokémon term → our own icon, else emoji.
      const face = c.sprite ? el('img', { src: c.sprite, alt: '' })
        : c.icon ? iconEl(c.icon)
        : (c.emoji || c.label);
      const kind = c.sprite ? 'sprite' : c.icon ? 'ico' : c.emoji ? 'emoji' : '';
      const tile = el('button', { class: `tile ${kind}` }, face);
      tile.addEventListener('click', async () => {
        if (tile.disabled) return;
        if (c.correct) {
          [...grid.children].forEach((t) => { if (t !== tile) t.classList.add('dim'); t.disabled = true; });
          tile.classList.add('good');
          if (!answered) { Session.record(item.id, true); comboUp(combo); }
          answered = true;
          if (replay) replay.disabled = false;
          if (isText) {
            // The reward for decoding is hearing the word confirmed — playing a
            // spoken "Bravo !" first doubled the wait on every single answer.
            burst(['⭐', '🎉', '👏', '💫'][praiseIdx++ % 4]);
            Sfx.correct();
            cheerCompanion();
            await Voice.play(item.audio, item.prompt);
          } else {
            await praise();
          }
          await sleep(150);
          idx++;
          step();
        } else {
          if (!answered) { Session.record(item.id, false); answered = true; comboBreak(combo); }
          tile.classList.add('bad');
          tile.disabled = true;
          setTimeout(() => tile.classList.remove('bad'), 450);
          Sfx.wrong();
          await say('Presque ! Écoute encore.');
          await Voice.play(item.audio, item.prompt);
        }
      });
      grid.append(tile);
    });
    stage.append(grid);
  };
  step();
}

/* "Lis la phrase", the CP way: read the sentence aloud (any word he stalls
   on can be tapped and heard), then "J'AI LU !" reads it back with each word
   lighting up as the voice reaches it — he checks himself against it — and
   finally the comprehension question: which picture goes with the sentence?
   Only reading the whole sentence answers it. */
function renderPhrase(a, body, done) {
  let idx = 0;
  const head = el('h2', {}, a.title);
  const hintEl = hint(a.instruction);
  const combo = comboChip();
  const pipsBox = el('div', { class: 'pips-holder' });
  const stage = el('div', { class: 'act', style: 'width:100%' });
  body.append(head, hintEl, el('div', { class: 'meter-row' }, pipsBox, combo), stage);

  const step = () => {
    pipsBox.innerHTML = '';
    pipsBox.append(progressPips(idx, a.items.length));
    if (idx >= a.items.length) return done();
    const item = a.items[idx];
    const words = item.syllables; // the sentence's words, in order
    stage.innerHTML = '';

    const chips = words.map((w, k) => el('button', {
      class: 'read-word',
      onclick: () => Voice.play(item.syllable_audio ? item.syllable_audio[k] : null, w),
    }, w));
    const sentence = el('div', { class: 'read-sentence' }, chips);
    const readBtn = el('button', { class: 'btn btn-huge btn-go' }, 'J\'AI LU ! 🔊');
    readBtn.addEventListener('click', async () => {
      readBtn.remove();
      await karaoke(item.audio, item.prompt, chips);
      ask();
    }, { once: true });
    stage.append(sentence, readBtn);

    const ask = () => {
      let first = true;
      const grid = el('div', { class: `grid ${item.choices.length === 3 ? 'three' : 'two'} phrase-pics` });
      item.choices.forEach((c) => {
        const face = c.sprite ? el('img', { src: c.sprite, alt: '' })
          : c.icon ? iconEl(c.icon) : el('span', {}, c.emoji);
        const tile = el('button', { class: `tile ${c.sprite ? 'sprite' : c.icon ? 'ico' : 'emoji'}` }, face);
        tile.addEventListener('click', async () => {
          if (tile.disabled || grid.classList.contains('done')) return;
          if (c.correct) {
            grid.classList.add('done');
            tile.classList.add('good');
            Session.record(item.id, first);
            if (first) comboUp(combo); else comboBreak(combo);
            await praise();
            await sleep(250);
            idx++;
            step();
          } else {
            first = false;
            tile.disabled = true;
            tile.classList.add('bad');
            Sfx.wrong();
            // Wrong picture: hear the sentence again, karaoke and all.
            await karaoke(item.audio, item.prompt, chips);
          }
        });
        grid.append(tile);
      });
      stage.append(hint('Quelle image va avec la phrase ?'), grid);
    };
  };
  step();
}

/* karaoke plays a sentence and lights its words up as the voice reaches
   them, pacing by the real audio clock (each word gets a share of the clip
   proportional to its length). Without a clip, a steady beat per word. */
async function karaoke(url, text, chips) {
  chips.forEach((c) => c.classList.remove('lit', 'read'));
  const weights = chips.map((c) => c.textContent.length + 2);
  const total = weights.reduce((x, y) => x + y, 0);
  let started = Date.now();
  const light = (k) => chips.forEach((c, i) => {
    c.classList.toggle('lit', i === k);
    c.classList.toggle('read', i < k);
  });
  const timer = setInterval(() => {
    const a = Voice.el;
    let frac;
    if (a && a.duration && isFinite(a.duration) && !a.paused) frac = a.currentTime / a.duration;
    else frac = (Date.now() - started) / (chips.length * 450);
    let acc = 0;
    let k = 0;
    for (; k < weights.length; k++) {
      acc += weights[k] / total;
      if (frac < acc) break;
    }
    light(Math.min(k, chips.length - 1));
  }, 60);
  started = Date.now();
  await Voice.play(url, text);
  clearInterval(timer);
  chips.forEach((c) => { c.classList.remove('lit'); c.classList.add('read'); });
}

function renderCombat(a, body, done) {
  const f = a.fight;
  let hp = f.hp;
  let round = 0;
  let perfect = 0; // rounds built without a single wrong syllable
  let over = false; // defeat cuts the fight short

  // Battle hearts: every wrong tap costs one, at zero the battle is LOST and
  // the episode restarts — reading mistakes have stakes now, like the games.
  // -1 from the settings means unlimited (the old behaviour).
  const maxLives = (Session.data && Session.data.lives) || -1;
  let lives = maxLives;
  const heartsEl = el('div', { class: 'hearts' });
  const paintHearts = () => {
    if (maxLives === -1) return;
    heartsEl.textContent = '❤️'.repeat(lives) + '🖤'.repeat(maxLives - lives);
  };
  paintHearts();

  const foeImg = el('img', { src: `sprites/art/${f.pokemon}.png`, alt: '' });
  const allyImg = companion('ally');
  // The partner's attack follows its first type: a name on screen, a
  // projectile across the arena.
  const partner = App.state && App.state.partner;
  const move = MOVES[((partner && partner.types) || [])[0]] || MOVES.Normal;
  let fastSaid = false;
  let t0 = performance.now();
  const hpFill = el('i');
  const hpBar = el('div', { class: 'hpbar' }, hpFill);

  // Green → yellow → red, like the games. The child reads the bar long before
  // he reads the word, so it is the clearest possible progress signal.
  const paintHP = () => {
    const pct = Math.max(0, hp / f.hp) * 100;
    hpFill.style.width = `${pct}%`;
    hpBar.className = `hpbar${pct <= 34 ? ' low' : pct <= 67 ? ' mid' : ''}`;
  };

  const arena = el('div', { class: `arena ${f.legendary ? 'legend' : ''}` },
    el('div', { class: 'foe' },
      el('div', { class: 'nameplate' },
        el('div', { class: 'name' }, f.name),
        el('div', { class: 'hprow' }, el('b', {}, 'PV'), hpBar),
      ),
      el('div', { class: 'platform' }, foeImg),
    ),
    el('div', { class: 'mine' },
      el('div', { class: 'platform' }, allyImg),
      App.state && App.state.partner
        ? el('div', { class: 'nameplate ally-plate' },
          el('div', { class: 'name' }, App.state.partner.name, el('span', { class: 'lvl' }, ` N.${App.state.partner.level}`)))
        : null,
      heartsEl,
    ),
  );
  paintHP();

  // Tower floors are scored: 2 points for a word built without a wrong tap,
  // 1 otherwise; the chip shows the running total against the floor's target.
  const tower = Session.data && Session.data.tower;
  const scoreChip = tower ? el('span', { class: 'chip tower-chip' }, `0 / ${tower.need} points`) : null;
  const paintScore = () => {
    if (!tower) return;
    const pts = round + perfect;
    scoreChip.textContent = `${pts} / ${tower.need} points`;
    scoreChip.classList.toggle('ok', pts >= tower.need);
  };

  const stage = el('div', { class: 'act', style: 'width:100%' });
  const cols = el('div', { class: 'combat-cols' },
    el('div', { class: 'combat-left' }, arena),
    el('div', { class: 'combat-right' }, hint(a.instruction), stage),
  );
  // Plain DOM append stringifies null into a literal "null" text node, so the
  // chip is only ever passed when it exists.
  body.append(
    el('h2', { class: f.legendary ? 'legend-title' : '' },
      tower ? `🗼 ÉTAGE ${tower.floor}` : f.legendary ? '⚡ COMBAT LÉGENDAIRE ⚡' : a.title),
    ...(scoreChip ? [scoreChip] : []),
    cols,
  );

  // The trainers' face-off: both sprites slam in around a VS flash before the
  // first word appears. Pure staging, but it is what makes it a *battle*.
  const intro = () => {
    const splash = el('div', { class: `vs-splash ${f.legendary ? 'legend' : ''}` },
      el('img', { class: 'vs-ally', src: `sprites/art/${companionId()}.png`, alt: '' }),
      el('span', { class: 'vs-mark' }, 'VS'),
      el('img', { class: 'vs-foe', src: `sprites/art/${f.pokemon}.png`, alt: '' }),
    );
    stage.append(splash);
    if (f.legendary) {
      Sfx.legendary();
      burst('⚡');
    } else {
      Sfx.attack();
    }
    setTimeout(() => { splash.remove(); step(); }, 1400);
  };

  const step = () => {
    if (hp <= 0 || round >= f.rounds.length) return victory();
    const item = f.rounds[round];
    let pos = 0;
    let missed = false;
    t0 = performance.now();
    stage.innerHTML = '';

    const slots = el('div', { class: 'slot-row' });
    item.syllables.forEach(() => slots.append(el('div', { class: 'slot' }, '')));

    const tray = el('div', { class: 'tray' });
    item.tray.forEach((syl) => {
      const b = el('button', { class: 'syl-btn' }, syl);
      b.addEventListener('click', async () => {
        if (b.classList.contains('used')) return;
        if (syl === item.syllables[pos]) {
          Sfx.tap();
          const slot = slots.children[pos];
          slot.textContent = syl;
          slot.classList.add('filled');
          b.classList.add('used');
          pos++;
          if (pos === item.syllables.length) await attack(item, missed);
        } else {
          missed = true;
          b.classList.add('bad');
          setTimeout(() => b.classList.remove('bad'), 450);
          // The foe counterattacks — and with the lives setting on, it hurts.
          Sfx.thud();
          foeImg.classList.remove('lunge');
          void foeImg.offsetWidth;
          foeImg.classList.add('lunge');
          allyImg.classList.add('flinch');
          setTimeout(() => allyImg.classList.remove('flinch'), 500);
          if (maxLives !== -1) {
            lives--;
            paintHearts();
            burst('💔');
            if (lives <= 0) {
              over = true;
              Session.record(item.id, false);
              return defeat();
            }
          }
          await Voice.play(item.audio, item.prompt);
        }
      });
      tray.append(b);
    });

    // The attack word is shown whole: segmenting it into syllables is the
    // exercise, so no colour hints and no tap-to-hear here.
    stage.append(el('div', { class: 'word' }, item.prompt), slots, tray);
  };

  const attack = async (item, missed) => {
    if (over) return;
    Session.record(item.id, !missed);
    const fast = !missed && performance.now() - t0 < 1400 * item.syllables.length + 800;
    stage.innerHTML = '';
    const who = partner ? `${partner.name} utilise ${move.name} !` : `Attaque ${move.name} !`;
    if (missed) {
      Sfx.attack();
      stage.append(el('div', { class: 'shout' }, who));
    } else {
      // A word built without a single wrong tap lands as a critical hit.
      perfect++;
      Sfx.crit();
      stage.append(el('div', { class: 'shout crit' }, who), el('div', { class: 'shout-sub' }, fast ? '⚡ SUPER RAPIDE ! COUP CRITIQUE !' : 'COUP CRITIQUE !'));
    }
    // The partner lunges, the attack flies across the arena.
    allyImg.classList.remove('lunge-fwd');
    void allyImg.offsetWidth;
    allyImg.classList.add('lunge-fwd');
    const shot = el('span', { class: `projectile ${missed ? '' : 'big'}` }, move.fx);
    arena.append(shot);
    setTimeout(() => shot.remove(), 700);
    const spoken = Voice.play(item.audio, item.prompt);
    await sleep(520);
    foeImg.classList.add('hit');
    burst(move.fx);
    Arcade.popup(foeImg, missed ? '-1' : 'CRITIQUE ×2', missed ? '' : 'hot');
    if (fast && !fastSaid) { fastSaid = true; await spoken; await say('Super rapide !'); }
    await spoken;
    cheerCompanion();
    hp--;
    paintHP();
    await sleep(450);
    foeImg.classList.remove('hit');
    // The arena finale: on its last hit point a legendary boss GIGAMAXES —
    // the sprite grows huge for the last word. Pure staging, maximum drama.
    if (f.legendary && hp === 1 && round + 1 < f.rounds.length && !foeImg.classList.contains('giga')) {
      foeImg.classList.add('giga');
      stage.innerHTML = '';
      stage.append(el('div', { class: 'shout crit' }, 'GIGAMAX !'));
      Sfx.legendary();
      burst('🌟');
      await sleep(1100);
    }
    round++;
    paintScore();
    step();
  };

  // Out of hearts: the battle is lost. Mistakes still feed the review boxes,
  // then the whole episode restarts — losing exists, and it costs the run.
  const defeat = async () => {
    WakeLock.off();
    music.stop();
    allyImg.classList.add('faint');
    foeImg.classList.remove('lunge');
    stage.innerHTML = '';
    stage.append(el('div', { class: 'shout defeat' }, 'DÉFAITE...'));
    Sfx.faint();
    try {
      await api(`api/session/${Session.data.episode_id}/result`, {
        items: Session.answers, stars: 0, completed: false,
      });
    } catch (_) {}
    await say('Oh non, tu as perdu ! Entraîne-toi et reviens plus fort !');
    const isTower = !!Session.data.tower;
    App.screen(
      App.topbar(),
      el('div', { class: 'page' },
        el('div', { class: 'act' },
          el('h2', {}, '💔 Défaite...'),
          isTower
            ? hint('La Tour t\'a battu cette fois. Réessaie l\'étage !')
            : hint('Le combat est perdu : l\'épisode recommence du début. Tu vas y arriver !'),
          el('button', { class: 'btn btn-huge', onclick: () => Session.start(Session.data.episode_id) },
            isTower ? 'RÉESSAYER L\'ÉTAGE 🔄' : 'RECOMMENCER L\'ÉPISODE 🔄'),
          el('button', { class: 'btn btn-ghost', onclick: () => App.home() }, 'CARTE 🗺️'),
        ),
      ),
    );
  };

  const victory = async () => {
    // finish() needs the fight's tally to score a tower floor.
    Session.fightStats = { perfect, rounds: round };
    music.stop();
    // K.O.: the foe spins away, the arena flashes, a giant K.O.
    arena.classList.add('ko-flash');
    foeImg.classList.add('ko');
    stage.innerHTML = '';
    stage.append(el('div', { class: 'shout ko-text' }, 'K.O. !'));
    Sfx.ko();
    confetti(50);
    await say('K.O. ! Bravo !');
    stage.innerHTML = '';
    stage.append(el('div', { class: 'shout' }, 'GAGNÉ !'));
    burst('🏅');
    await say('Tu as gagné le combat !');
    if (perfect === round && round > 0) {
      // Every single word first try: worth its own fanfare.
      confetti();
      burst('🎯');
      stage.innerHTML = '';
      stage.append(el('div', { class: 'shout crit' }, 'COMBAT PARFAIT !'));
      await say('Combat parfait !');
    }
    await sleep(400);
    done();
  };

  // The battle beat: soft, original, and faster as the foe weakens.
  const music = Sfx.battle(() => document.body.contains(arena), () => hp / f.hp);
  intro();
}

// One attack per type: its French name and what flies across the arena.
const MOVES = {
  Feu: { name: 'Flammèche', fx: '🔥' }, Eau: { name: 'Pistolet à O', fx: '💧' },
  Plante: { name: 'Tranch\'Herbe', fx: '🍃' }, 'Électrik': { name: 'Éclair', fx: '⚡' },
  Glace: { name: 'Vent Glace', fx: '❄️' }, Psy: { name: 'Choc Mental', fx: '🌀' },
  Vol: { name: 'Tornade', fx: '🌪️' }, Poison: { name: 'Dard-Venin', fx: '💜' },
  Sol: { name: 'Coud\'Boue', fx: '🟤' }, Roche: { name: 'Jet-Pierres', fx: '🌑' },
  Combat: { name: 'Poing-Karaté', fx: '👊' }, Insecte: { name: 'Dard-Nuée', fx: '🐝' },
  Spectre: { name: 'Léchouille', fx: '👻' }, Dragon: { name: 'Draco-Rage', fx: '🐉' },
  'Fée': { name: 'Vent Féérique', fx: '✨' }, Normal: { name: 'Charge', fx: '⭐' },
};

/* The badge ceremony: the medal rises out of a glow, the room applauds. Earned
   only after an arena victory, so it should feel like the games' badge screen. */
function renderBadge(a, body, done) {
  const b = a.badge;
  const stage = el('div', { class: 'reveal' });
  body.append(el('h2', {}, '🎖 ' + a.title), stage);

  stage.append(
    el('div', { class: 'badge-hero' }, badgeEl(b.id, true, 'badge-big')),
    el('div', { class: 'score' }, 'Il rejoint ta collection !'),
    el('button', { class: 'btn btn-huge btn-go', onclick: done }, 'TERMINER ✓'),
  );
  Sfx.badge();
  confetti(70);
  burst('🎖');
  Voice.play(b.audio, `Tu as gagné le ${b.name} !`).then(() => say('Il rejoint ta collection !'));
}

function renderReward(a, body, done) {
  const r = a.reward;
  // Remember the shiny roll so finish() reports it and the Pokédex keeps it.
  if (r.shiny) Session.shiny = true;
  const stage = el('div', { class: 'reveal' });
  body.append(el('h2', {}, r.shiny ? '✨ Un Pokémon chromatique !' : a.title), stage);

  stage.append(el('div', { class: 'catch-ball' }, pokeball()));
  burst('✨');
  Sfx.catch_();
  confetti(r.shiny ? 110 : 70);

  setTimeout(async () => {
    stage.innerHTML = '';
    stage.append(
      el('img', { class: `art ${r.shiny ? 'shiny' : ''}`, src: `sprites/art/${r.pokemon}.png`, alt: '' }),
      r.shiny ? el('span', { class: 'shinymark' }, '✨ chromatique ✨') : null,
      syllableWord(r.syllables, r.name, r.syllable_audio),
      el('div', { class: 'types' }, (r.types || []).map((t) => el('span', { 'data-type': t }, t))),
      el('div', { class: 'dex' }, r.dex),
      el('button', { class: 'btn btn-huge btn-go', onclick: done }, 'TERMINER ✓'),
    );
    burst(r.shiny ? '✨' : '🎉');
    if (r.shiny) await say('Ouah ! Un Pokémon chromatique !');
    await Voice.play(r.name_audio, r.name);
    await Voice.play(r.dex_audio, r.dex);
  }, 1500);
}

/* ---------------------------------------------------------------- arcade ---
   The mini-games corner: timed games with stored records to beat. The Pokémon
   games (memory, la chasse, mystère) draw from the whole roster — the course's
   Pokémon caught or not, plus data/arcade.json — so they stay varied from day
   one; the reading games use only learned material and report their answers so
   the spaced repetition learns from play. Nothing here blocks or advances the
   course. */

const Arcade = {
  timer: null,     // the game's interval/timeout handle
  pool: null,      // learned words + syllables + Pokémon roster from /api/arcade
  items: [],       // spaced-repetition answers collected by comprehension games

  GAMES: [
    { id: 'atelier', icon: '⚒️', name: 'L\'atelier des mots', desc: 'Reconstruis un maximum de mots, comme au combat ! 90 secondes.', lower: false, unit: '' },
    { id: 'lecture', icon: '📖', name: 'Lis et attrape', desc: 'Lis le mot, touche la bonne image. 60 secondes !', lower: false, unit: '' },
    { id: 'oreille', icon: '👂', name: 'L\'oreille fine', desc: 'Écoute la syllabe, touche-la. 60 secondes !', lower: false, unit: '' },
    { id: 'mystere', icon: '❓', name: 'Pokémon mystère', desc: 'Qui est cette ombre ? Lis les noms pour le découvrir. 60 secondes !', lower: false, unit: '' },
    { id: 'paires', icon: '🔤', name: 'Les paires mot-image', desc: 'Associe chaque mot écrit à son image, le plus vite possible.', lower: true, unit: 's' },
    { id: 'memory', icon: '🃏', name: 'Memory Pokémon', desc: 'Retrouve les paires le plus vite possible.', lower: true, unit: 's' },
    { id: 'chasse', icon: '🌿', name: 'La chasse', desc: 'Attrape les Pokémon avant qu\'ils se cachent. 45 secondes !', lower: false, unit: '' },
  ],

  stop() {
    if (this.timer) { clearInterval(this.timer); clearTimeout(this.timer); this.timer = null; }
    this.addTime = null;
  },

  /* The juice every game shares: a combo multiplier (x2 from 3 in a row, x3
     from 6), the points popping out of what was tapped, +2 s on the clock
     while the combo burns, the partner jumping, and a cheer every fifth. */
  streak: 0,
  comboEl: null,
  addTime: null,
  CHEERS: ['Trop fort !', 'Incroyable !', 'Tu es une fusée !', 'Continue comme ça !'],

  gain(node, base = 1) {
    this.streak++;
    const mult = this.streak >= 6 ? 3 : this.streak >= 3 ? 2 : 1;
    const pts = base * mult;
    this.popup(node, `+${pts}`, mult > 1 ? 'hot' : '');
    if (mult > 1 && this.addTime) {
      this.addTime(2);
      setTimeout(() => this.popup(node, '+2s', 'time'), 180);
    }
    this.paintCombo();
    cheerCompanion();
    if (this.streak % 5 === 0) {
      confetti(30);
      burst('🔥');
      say(this.CHEERS[(this.streak / 5 - 1) % this.CHEERS.length]);
    }
    return pts;
  },

  miss(node) {
    this.streak = 0;
    this.paintCombo();
    if (node) this.popup(node, '✗', 'miss');
    const page = $('.page');
    if (page) { page.classList.remove('jolt'); void page.offsetWidth; page.classList.add('jolt'); }
  },

  paintCombo() {
    const c = this.comboEl;
    if (!c) return;
    const mult = this.streak >= 6 ? 3 : this.streak >= 3 ? 2 : 1;
    c.hidden = this.streak < 2;
    c.textContent = mult > 1 ? `🔥 ${this.streak} · x${mult}` : `🔥 ${this.streak}`;
    c.className = `chip combo arcade-combo m${mult}`;
    c.classList.remove('pop'); void c.offsetWidth; c.classList.add('pop');
  },

  // A floating "+2" rising out of the tapped element.
  popup(node, text, cls) {
    if (!node || !node.getBoundingClientRect) return;
    const r = node.getBoundingClientRect();
    const pop = el('div', { class: `score-pop ${cls || ''}` }, text);
    pop.style.left = `${r.left + r.width / 2}px`;
    pop.style.top = `${r.top + r.height / 3}px`;
    document.body.append(pop);
    setTimeout(() => pop.remove(), 900);
  },

  /* Every Pokémon the games may show; the caught ones if the roster is missing. */
  roster() {
    const all = (this.pool && this.pool.pokemon) || [];
    if (all.length) return all;
    return ((App.state && App.state.pokedex) || []).filter((p) => p.caught);
  },

  record(id) {
    return (App.state && App.state.records && App.state.records[id]) || null;
  },

  /* The hub: pick a game, see the record to beat. */
  async hub() {
    Voice.stop();
    this.stop();
    if (!this.pool) {
      try { this.pool = await api('api/arcade'); } catch (_) { this.pool = { words: [], syllables: [], pokemon: [] }; }
    }
    const cards = this.GAMES.map((g) => {
      const best = this.record(g.id);
      return el('button', { class: 'game-card', onclick: () => this[g.id]() },
        el('span', { class: 'game-icon' }, g.icon),
        el('b', {}, g.name),
        el('small', {}, g.desc),
        el('span', { class: 'game-record' }, best !== null ? `🏆 Record : ${best}${g.unit}` : 'Pas encore de record'),
      );
    });
    App.screen(
      App.topbar(el('button', { class: 'btn-icon', onclick: () => App.home() }, '✕')),
      el('div', { class: 'page' },
        el('div', { class: 'act' },
          el('h2', {}, '🎮 Mini-jeux'),
          hint('Choisis un jeu, et bats tes records !'),
          el('div', { class: 'game-grid' }, cards),
        ),
      ),
    );
  },

  screen(title, hintText, ...rest) {
    this.stop();
    this.streak = 0;
    this.comboEl = el('span', { class: 'chip combo arcade-combo', hidden: true });
    App.screen(
      App.topbar(this.comboEl, el('button', { class: 'btn-icon', onclick: () => { this.stop(); this.hub(); } }, '✕')),
      el('div', { class: 'page' },
        el('div', { class: 'act' },
          el('h2', {}, title),
          hintText ? hint(hintText) : null,
          ...rest,
        ),
      ),
    );
    $('#app').append(companion());
  },

  /* A visible countdown bar; onDone fires when time is up. */
  countdown(seconds, onDone) {
    const fill = el('i');
    const bar = el('div', { class: 'timebar' }, fill);
    let left = seconds;
    fill.style.width = '100%';
    this.addTime = (s) => { left = Math.min(seconds, left + s); };
    this.timer = setInterval(() => {
      left -= 0.1;
      const pct = Math.max(0, left / seconds) * 100;
      fill.style.width = `${pct}%`;
      bar.classList.toggle('low', pct <= 20);
      if (left <= 0) {
        this.stop();
        onDone();
      }
    }, 100);
    return bar;
  },

  /* Post the score, then celebrate — with extra fireworks on a new record. */
  async gameOver(game, score, lower, title) {
    this.stop();
    Voice.stop();
    const prev = this.record(game);
    // Gold for a record, silver within a quarter of it, bronze otherwise.
    const ratio = prev === null || prev === 0 ? 1 : lower ? prev / Math.max(1, score) : score / prev;
    const medal = prev === null || (lower ? score < prev : score > prev) ? '🥇' : ratio >= 0.75 ? '🥈' : '🥉';
    let beaten = false;
    let res = {};
    try {
      res = await api('api/records', { game, score, lower, items: this.items });
      App.state = res.state;
      beaten = res.beaten;
    } catch (_) {}
    this.items = [];
    const g = this.GAMES.find((x) => x.id === game);
    App.screen(
      App.topbar(),
      el('div', { class: 'page' },
        el('div', { class: 'act' },
          el('h2', {}, title),
          el('div', { class: 'medal' }, medal),
          el('div', { class: 'bigscore' }, `${score}${g.unit}`),
          beaten ? el('div', { class: 'score newrecord' }, '🏆 NOUVEAU RECORD !') : el('div', { class: 'score' }, `Record : ${this.record(game)}${g.unit}`),
          ballsWon(res.balls_won),
          Partner.gainRow(res.partner_event),
          el('button', { class: 'btn btn-huge', onclick: () => this[game]() }, 'REJOUER 🔄'),
          el('button', { class: 'btn btn-ghost', onclick: () => this.hub() }, '🎮 Mini-jeux'),
          el('button', { class: 'btn btn-go', onclick: () => App.home() }, 'CARTE 🗺️'),
        ),
      ),
    );
    if (beaten) { confetti(90); burst('🏆'); Sfx.fanfare(); await say('Nouveau record !'); }
    else { burst('🎉'); Sfx.correct(); await say('Bravo !'); }
    if (res.balls_won) await say('Tu gagnes des Poké Balls !');
    await celebrateAll(res);
  },

  /* Memory: six pairs of Pokémon, the record is the time. */
  memory() {
    const ids = this.roster().map((p) => p.id);
    ids.sort(() => Math.random() - 0.5);
    const picks = ids.slice(0, 6);
    const pad = [25, 1, 4, 7, 133, 143];
    while (picks.length < 6) picks.push(pad[picks.length]);
    const cards = [...picks, ...picks].sort(() => Math.random() - 0.5);

    let open = null;
    let lock = false;
    let matched = 0;
    let seconds = 0;

    const clock = el('span', { class: 'chip' }, '⏱ 0s');
    const grid = el('div', { class: 'memory-grid big' });
    cards.forEach((id) => {
      const card = el('button', { class: 'memory-card' },
        el('span', { class: 'face back' }, pokeball('pokeball-sm')),
        el('span', { class: 'face front' }, el('img', { src: `sprites/art/${id}.png`, alt: '' })),
      );
      card.dataset.id = id;
      card.addEventListener('click', () => {
        if (lock || card.classList.contains('up') || card.classList.contains('won')) return;
        Sfx.flip();
        card.classList.add('up');
        if (!open) { open = card; return; }
        if (open.dataset.id === card.dataset.id) {
          open.classList.add('won');
          card.classList.add('won');
          open = null;
          matched++;
          Sfx.correct();
          burst('✨');
          this.gain(card);
          if (matched === picks.length) this.gameOver('memory', seconds, true, '🃏 Memory Pokémon');
        } else {
          lock = true;
          const other = open;
          open = null;
          Sfx.wrong(); this.miss();
          setTimeout(() => {
            other.classList.remove('up');
            card.classList.remove('up');
            lock = false;
          }, 800);
        }
      });
      grid.append(card);
    });

    this.screen('🃏 Memory Pokémon', 'Retrouve les paires !', clock, grid);
    this.timer = setInterval(() => { seconds++; clock.textContent = `⏱ ${seconds}s`; }, 1000);
  },

  /* La chasse : 45 seconds, the pop-ups speed up as the clock runs. */
  chasse() {
    const ids = this.roster().map((p) => p.id);
    let score = 0;
    let active = null;
    let popTimer = null;

    const scoreChip = el('span', { class: 'chip' }, 'Attrapés : 0');
    const holes = [];
    const grid = el('div', { class: 'chase-grid' });
    for (let i = 0; i < 9; i++) {
      const img = el('img', { alt: '' });
      const hole = el('button', { class: 'chase-hole' }, img);
      hole.addEventListener('click', () => {
        if (hole !== active) return;
        active = null;
        hole.classList.remove('show');
        const kind = hole.dataset.kind;
        if (kind === 'bomb') {
          // Voltorbe: it explodes, the combo goes with it.
          score = Math.max(0, score - 2);
          Sfx.thud();
          burst('💥');
          this.popup(hole, '-2', 'miss');
          this.miss();
        } else {
          score += this.gain(hole, kind === 'gold' ? 5 : 1);
          Sfx.pop();
          burst(kind === 'gold' ? '🌟' : '✨');
          if (kind === 'gold') { Sfx.catch_(); confetti(40); }
        }
        scoreChip.textContent = `Attrapés : ${score}`;
      });
      holes.push(hole);
      grid.append(hole);
    }

    let elapsed = 0;
    const pop = () => {
      if (active) { active.classList.remove('show'); active = null; }
      const hole = holes[Math.floor(Math.random() * holes.length)];
      // 1 in 8: a golden shiny worth 5; 1 in 7: Voltorbe, which explodes.
      const roll = Math.random();
      const kind = roll < 0.125 ? 'gold' : roll < 0.27 ? 'bomb' : 'normal';
      const id = kind === 'bomb' ? 100 : ids[Math.floor(Math.random() * ids.length)] || 25;
      const img = $('img', hole);
      img.src = `sprites/art/${id}.png`;
      img.classList.toggle('shiny', kind === 'gold');
      hole.dataset.kind = kind;
      hole.classList.toggle('gold', kind === 'gold');
      hole.classList.toggle('bomb', kind === 'bomb');
      hole.classList.add('show');
      active = hole;
      // From a friendly 1.1 s down to a frantic 0.55 s by the end.
      const delay = Math.max(550, 1100 - elapsed * 14);
      popTimer = setTimeout(pop, delay);
    };

    const bar = this.countdown(45, () => {
      clearTimeout(popTimer);
      this.gameOver('chasse', score, false, '🌿 La chasse');
    });
    // countdown() owns this.timer; track elapsed for the speed-up alongside it.
    const tick = setInterval(() => { elapsed++; }, 1000);
    const stopAll = this.stop.bind(this);
    this.stop = () => { clearTimeout(popTimer); clearInterval(tick); stopAll(); this.stop = stopAll; };

    this.screen('🌿 La chasse', 'Touche les Pokémon, mais pas Voltorbe : il explose !', scoreChip, bar, grid);
    pop();
  },

  /* Lis et attrape : read the word, tap the right picture — 60 s of reading. */
  lecture() {
    const words = (this.pool.words || []).filter((w) => w.sprite || w.emoji || w.icon);
    if (words.length < 4) return this.hub();
    this.items = [];
    let score = 0;

    const scoreChip = el('span', { class: 'chip' }, 'Points : 0');
    const stage = el('div', { class: 'act', style: 'width:100%' });

    const ask = () => {
      const shuffled = [...words].sort(() => Math.random() - 0.5);
      const target = shuffled[0];
      const options = [target];
      const seen = new Set([faceKey(target)]);
      for (const w of shuffled.slice(1)) {
        if (options.length === 4) break;
        if (!seen.has(faceKey(w))) { seen.add(faceKey(w)); options.push(w); }
      }
      options.sort(() => Math.random() - 0.5);

      stage.innerHTML = '';
      const grid = el('div', { class: 'grid four' });
      let answered = false;
      options.forEach((w) => {
        const face = w.sprite ? el('img', { src: w.sprite, alt: '' })
          : w.icon ? iconEl(w.icon)
          : el('span', {}, w.emoji);
        const tile = el('button', { class: `tile ${w.sprite ? 'sprite' : w.icon ? 'ico' : 'emoji'}` }, face);
        tile.addEventListener('click', () => {
          if (tile.disabled) return;
          if (w === target) {
            if (!answered) { this.items.push({ id: `mot:${target.text}`, correct: true }); score += this.gain(tile); }
            answered = true;
            scoreChip.textContent = `Points : ${score}`;
            tile.classList.add('good');
            Sfx.correct();
            burst('⭐');
            Voice.play(target.audio, target.text);
            setTimeout(ask, 550);
          } else {
            if (!answered) this.items.push({ id: `mot:${target.text}`, correct: false });
            answered = true;
            tile.classList.add('bad');
            tile.disabled = true;
            Sfx.wrong(); this.miss();
          }
        });
        grid.append(tile);
      });
      stage.append(el('div', { class: 'word arcade-word' }, target.text), grid);
    };

    const bar = this.countdown(60, () => this.gameOver('lecture', score, false, '📖 Lis et attrape'));
    this.screen('📖 Lis et attrape', 'Lis le mot, touche la bonne image !', scoreChip, bar, stage);
    ask();
  },

  /* L'atelier des mots : the battle's build-the-word mechanic as a race —
     as many words as possible in 90 s, each one rebuilt from its syllables
     plus one trap. The exercise the fight made fun, distilled. */
  atelier() {
    const words = (this.pool.words || []).filter((w) => (w.syllables || []).length >= 2);
    const syllables = (this.pool.syllables || []).map((s) => s.text);
    if (words.length < 3) return this.hub();
    this.items = [];
    let score = 0;

    const scoreChip = el('span', { class: 'chip' }, 'Mots : 0');
    const stage = el('div', { class: 'act', style: 'width:100%' });

    const ask = () => {
      const target = words[Math.floor(Math.random() * words.length)];
      let pos = 0;
      let missed = false;
      stage.innerHTML = '';

      const slots = el('div', { class: 'slot-row' });
      target.syllables.forEach(() => slots.append(el('div', { class: 'slot' }, '')));

      const inWord = new Set(target.syllables.map((x) => x.toLowerCase()));
      const traps = syllables.filter((x) => !inWord.has(x.toLowerCase()));
      const tray = [...target.syllables];
      if (traps.length) tray.push(traps[Math.floor(Math.random() * traps.length)]);
      tray.sort(() => Math.random() - 0.5);

      const trayEl = el('div', { class: 'tray' });
      tray.forEach((syl) => {
        const b = el('button', { class: 'syl-btn' }, syl);
        b.addEventListener('click', () => {
          if (b.classList.contains('used')) return;
          if (syl === target.syllables[pos]) {
            Sfx.tap();
            const slot = slots.children[pos];
            slot.textContent = syl;
            slot.classList.add('filled');
            b.classList.add('used');
            pos++;
            if (pos === target.syllables.length) {
              this.items.push({ id: `mot:${target.text}`, correct: !missed });
              score += this.gain(slots, missed ? 1 : 2);
              scoreChip.textContent = `Mots : ${score}`;
              Sfx.crit();
              burst('⚒️');
              Voice.play(target.audio, target.text);
              setTimeout(ask, 500);
            }
          } else {
            missed = true;
            b.classList.add('bad');
            setTimeout(() => b.classList.remove('bad'), 450);
            Sfx.wrong(); this.miss();
          }
        });
        trayEl.append(b);
      });

      stage.append(el('div', { class: 'word arcade-word' }, target.text), slots, trayEl);
    };

    const bar = this.countdown(90, () => this.gameOver('atelier', score, false, '⚒️ L\'atelier des mots'));
    this.screen('⚒️ L\'atelier des mots', 'Remets les syllabes dans l\'ordre, encore et encore !', scoreChip, bar, stage);
    ask();
  },

  /* Pokémon mystère : a silhouette from the whole roster, four written names —
     reading Pokémon names is the whole game's fantasy, distilled. */
  mystere() {
    const roster = this.roster();
    if (roster.length < 4) return this.hub();
    this.items = [];
    let score = 0;

    const scoreChip = el('span', { class: 'chip' }, 'Points : 0');
    const stage = el('div', { class: 'act', style: 'width:100%' });

    const ask = () => {
      const shuffled = [...roster].sort(() => Math.random() - 0.5);
      const target = shuffled[0];
      const options = shuffled.slice(0, 4).sort(() => Math.random() - 0.5);

      stage.innerHTML = '';
      const shadow = el('img', { class: 'silhouette', src: target.sprite, alt: '' });
      const grid = el('div', { class: 'grid four name-grid' });
      let answered = false;
      options.forEach((p) => {
        const tile = el('button', { class: 'tile name-tile' }, p.name);
        tile.addEventListener('click', () => {
          if (tile.disabled) return;
          if (p === target) {
            if (!answered) score += this.gain(tile);
            answered = true;
            scoreChip.textContent = `Points : ${score}`;
            tile.classList.add('good');
            shadow.classList.add('revealed');
            Sfx.catch_();
            burst('✨');
            Voice.play(target.name_audio, target.name);
            setTimeout(ask, 900);
          } else {
            answered = true;
            tile.classList.add('bad');
            tile.disabled = true;
            Sfx.wrong(); this.miss();
          }
        });
        grid.append(tile);
      });
      stage.append(shadow, grid);
    };

    const bar = this.countdown(60, () => this.gameOver('mystere', score, false, '❓ Pokémon mystère'));
    this.screen('❓ Pokémon mystère', 'Qui est cette ombre ? Lis les noms !', scoreChip, bar, stage);
    ask();
  },

  /* Les paires mot-image : memory where one card of each pair is the WRITTEN
     word and the other its picture — reading as the matching key. */
  paires() {
    const words = (this.pool.words || []).filter((w) => w.sprite || w.emoji || w.icon);
    if (words.length < 4) return this.hub();
    this.items = [];
    const seen = new Set();
    const picks = [];
    for (const w of [...words].sort(() => Math.random() - 0.5)) {
      if (picks.length === 4) break;
      if (!seen.has(faceKey(w))) { seen.add(faceKey(w)); picks.push(w); }
    }
    // One word card + one picture card per pick, all shuffled together.
    const cards = [];
    picks.forEach((w) => {
      cards.push({ key: w.text, kind: 'text', w });
      cards.push({ key: w.text, kind: 'pic', w });
    });
    cards.sort(() => Math.random() - 0.5);

    let open = null;
    let lock = false;
    let matched = 0;
    let seconds = 0;

    const clock = el('span', { class: 'chip' }, '⏱ 0s');
    const grid = el('div', { class: 'memory-grid big' });
    cards.forEach((c) => {
      const front = c.kind === 'text'
        ? el('span', { class: 'pair-word' }, c.w.text)
        : c.w.sprite ? el('img', { src: c.w.sprite, alt: '' })
        : c.w.icon ? iconEl(c.w.icon)
        : el('span', { class: 'pair-emoji' }, c.w.emoji);
      const card = el('button', { class: 'memory-card' },
        el('span', { class: 'face back' }, pokeball('pokeball-sm')),
        el('span', { class: 'face front' }, front),
      );
      card.dataset.key = c.key;
      card.dataset.kind = c.kind;
      card.addEventListener('click', () => {
        if (lock || card.classList.contains('up') || card.classList.contains('won')) return;
        Sfx.flip();
        card.classList.add('up');
        if (!open) { open = card; return; }
        // A pair is a word card AND its picture card — two identical kinds don't match.
        if (open.dataset.key === card.dataset.key && open.dataset.kind !== card.dataset.kind) {
          open.classList.add('won');
          card.classList.add('won');
          open = null;
          matched++;
          this.items.push({ id: `mot:${c.key}`, correct: true });
          Sfx.correct();
          burst('✨');
          this.gain(card);
          const w = picks.find((x) => x.text === c.key);
          if (w) Voice.play(w.audio, w.text);
          if (matched === picks.length) this.gameOver('paires', seconds, true, '🔤 Les paires mot-image');
        } else {
          lock = true;
          const other = open;
          open = null;
          Sfx.wrong(); this.miss();
          setTimeout(() => {
            other.classList.remove('up');
            card.classList.remove('up');
            lock = false;
          }, 800);
        }
      });
      grid.append(card);
    });

    this.screen('🔤 Les paires mot-image', 'Associe chaque mot à son image !', clock, grid);
    this.timer = setInterval(() => { seconds++; clock.textContent = `⏱ ${seconds}s`; }, 1000);
  },

  /* L'oreille fine : hear the syllable, tap it — 60 s of phoneme work. */
  oreille() {
    const syllables = this.pool.syllables || [];
    if (syllables.length < 4) return this.hub();
    this.items = [];
    let score = 0;

    const scoreChip = el('span', { class: 'chip' }, 'Points : 0');
    const stage = el('div', { class: 'act', style: 'width:100%' });

    const ask = () => {
      const shuffled = [...syllables].sort(() => Math.random() - 0.5);
      const target = shuffled[0];
      const options = shuffled.slice(0, 4).sort(() => Math.random() - 0.5);

      stage.innerHTML = '';
      const listen = el('button', { class: 'btn', onclick: () => Voice.play(target.audio, target.text) }, '🔊 ÉCOUTE');
      const grid = el('div', { class: 'grid four' });
      let answered = false;
      options.forEach((s) => {
        const tile = el('button', { class: 'tile' }, s.text);
        tile.addEventListener('click', () => {
          if (tile.disabled) return;
          if (s === target) {
            if (!answered) { this.items.push({ id: `syl:${target.text}`, correct: true }); score += this.gain(tile); }
            answered = true;
            scoreChip.textContent = `Points : ${score}`;
            tile.classList.add('good');
            Sfx.correct();
            burst('⭐');
            setTimeout(ask, 500);
          } else {
            if (!answered) this.items.push({ id: `syl:${target.text}`, correct: false });
            answered = true;
            tile.classList.add('bad');
            tile.disabled = true;
            Sfx.wrong(); this.miss();
            Voice.play(target.audio, target.text);
          }
        });
        grid.append(tile);
      });
      stage.append(listen, grid);
      Voice.play(target.audio, target.text);
    };

    const bar = this.countdown(60, () => this.gameOver('oreille', score, false, '👂 L\'oreille fine'));
    this.screen('👂 L\'oreille fine', 'Écoute la syllabe, puis touche-la !', scoreChip, bar, stage);
    ask();
  },
};

// faceKey mirrors the server's picture identity, so two words sharing an image
// never end up as two different "right answers" on screen.
function faceKey(w) {
  return w.sprite ? `s${w.sprite}` : w.icon ? `i${w.icon}` : `e${w.emoji}`;
}

/* ------------------------------------------------------------------ boot --- */

$('#gate-btn').addEventListener('click', async () => {
  // First: fullscreen must be asked for inside the tap itself.
  if (Fullscreen.supported() && !Fullscreen.installed() && Fullscreen.wanted()) Fullscreen.enter();
  Voice.unlock();
  Sfx.unlock();
  Sfx.tap();
  $('#gate').hidden = true;
  $('#app').hidden = false;
  try {
    await App.boot();
  } catch (err) {
    $('#app').innerHTML = `<div class="screen"><h1>Oups</h1><p class="gate-sub">${err.message}</p></div>`;
  }
});

// iPadOS fires gesture events for pinch-zoom even with the viewport locked;
// preventing them keeps a two-finger mash from zooming the game.
document.addEventListener('gesturestart', (e) => e.preventDefault());

// A service worker (and therefore a real offline PWA) needs a secure context.
// Over plain http on the LAN this is a no-op; add mkcert TLS and it kicks in.
if ('serviceWorker' in navigator && window.isSecureContext) {
  navigator.serviceWorker.register('sw.js').catch(() => {});
}
