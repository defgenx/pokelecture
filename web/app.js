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
  if (!res.ok) throw new Error(`${path}: ${res.status} ${await res.text()}`);
  return res.json();
}

/* ------------------------------------------------------------------ voice ---
   Pre-generated .m4a files are the good path (one consistent French voice,
   recorded at a speed chosen per style). If a file is missing we fall back to
   the browser's own French voice so a content change never breaks the game. */

// A 40 ms silent WAV. Played inside the COMMENCER tap it grants the one audio
// element permission to play for the rest of the session — Android only trusts
// an element that was started from a real gesture.
const SILENCE = 'data:audio/wav;base64,UklGRjAAAABXQVZFZm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQwAAAAAAAAAAAAAAAAAAAA=';

const Voice = {
  el: null,
  voices: [],
  missing: new Set(), // URLs the server does not have; never wait on them twice
  primed: new Set(),  // URLs already pushed into the HTTP cache
  cancelled: 0,

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
    this.loadVoices();
  },

  // Android fills the voice list asynchronously: getVoices() is empty on the
  // first call, so an utterance built from it goes out with no French voice
  // attached and the device reads it in its own language — or stays silent.
  loadVoices() {
    if (!('speechSynthesis' in window)) return;
    const grab = () => { this.voices = window.speechSynthesis.getVoices() || []; };
    grab();
    window.speechSynthesis.onvoiceschanged = grab;
  },

  stop() {
    if (this.el) {
      try { this.el.pause(); } catch (_) {}
    }
    if ('speechSynthesis' in window) {
      const s = window.speechSynthesis;
      if (s.speaking || s.pending) {
        s.cancel();
        this.cancelled = Date.now();
      }
    }
  },

  play(url, text) {
    this.stop();
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
      const onErr = () => finish(true);

      a.addEventListener('ended', onEnd);
      a.addEventListener('error', onErr);
      a.src = url;
      const p = a.play();
      if (p && p.catch) p.catch(() => finish(true));
      // Safety net: a stalled element must not freeze the game. Scaled to the
      // text (every line here is a few seconds at most) — the old flat 12 s
      // was itself the freeze it was guarding against.
      setTimeout(() => finish(false), Math.min(2500 + (text || '').length * 90, 8000));
    });
  },

  speak(text) {
    if (!text || !('speechSynthesis' in window)) return Promise.resolve();
    const synth = window.speechSynthesis;
    return new Promise((resolve) => {
      let settled = false;
      const done = () => { if (!settled) { settled = true; resolve(); } };
      const go = () => {
        const list = this.voices.length ? this.voices : synth.getVoices() || [];
        // Android reports fr_FR, desktop fr-FR.
        const fr = list.find((v) => v.lang && v.lang.replace('_', '-').startsWith('fr'));
        // No voices at all (an Android tablet without a French TTS): an
        // utterance would never fire onend and the flat watchdog used to hold
        // the game hostage for 9 seconds per line. Give up fast instead.
        if (!list.length) return setTimeout(done, 300);
        const u = new SpeechSynthesisUtterance(text);
        u.lang = 'fr-FR';
        u.rate = 0.85;
        if (fr) u.voice = fr;
        u.onend = done;
        u.onerror = done;
        synth.speak(u);
        // Watchdog scaled to the line length, not one-size-fits-all.
        setTimeout(done, Math.min(1500 + text.length * 90, 7000));
      };
      // Android drops an utterance queued in the same task as a cancel(), which
      // is every line of the game: play() stops the previous one first.
      const wait = Date.now() - this.cancelled < 250 ? 150 : 0;
      if (wait) setTimeout(go, wait);
      else go();
    });
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

// say() prefers the recorded phrase — from the session payload, or from the
// state payload outside a session (home, Pokédex) — and degrades to the browser
// voice when the audio track has not been generated yet.
function say(phrase) {
  const url = (Session.data && Session.data.ui && Session.data.ui[phrase])
    || (App.state && App.state.ui && App.state.ui[phrase])
    || null;
  return Voice.play(url, phrase);
}

function praise() {
  burst(['⭐', '🎉', '👏', '💫'][praiseIdx % 4]);
  Sfx.correct();
  cheerCompanion();
  return say(PRAISE[praiseIdx++ % PRAISE.length]);
}

/* --------------------------------------------------------------- buddy ---
   A sprite of his best catch follows him from screen to screen and jumps when
   he gets one right. Pikachu is the starter buddy so the corner is never empty. */

function companionId() {
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
document.addEventListener('visibilitychange', () => {
  if (!document.hidden && Session.data) WakeLock.on();
});

/* ------------------------------------------------------------------ shell --- */

const App = {
  state: null,

  async boot() {
    this.state = await api('api/state');
    this.home();
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
    );
  },

  /* ------------------------------------------------------------ home ------ */

  home() {
    Voice.stop();
    WakeLock.off();
    const s = this.state;
    const next = s.episodes.find((e) => e.id === s.next_episode);
    const caught = s.pokedex.filter((p) => p.caught).length;
    const shinies = s.pokedex.filter((p) => p.shiny).length;

    this.screen(
      this.topbar(
        el('span', { class: 'chip' }, `⭐ ${s.stars}`),
        shinies > 0 ? el('span', { class: 'chip' }, `✨ ${shinies}`) : null,
        s.streak > 1 ? el('span', { class: 'chip' }, `🔥 ${s.streak}`) : null,
      ),
      el('div', { class: 'page' },
        el('div', { class: 'home-top' },
          el('div', { class: 'act' },
            el('h2', {}, s.all_done ? '🏆 Tu es le Champion !' : next ? next.title : 'Bravo !'),
            el('button', { class: 'btn btn-huge', onclick: () => Session.start(s.next_episode) },
              next && next.done > 0 ? 'REJOUER ▶' : 'JOUER ▶'),
            el('div', { class: 'home-row' },
              el('button', { class: 'btn btn-ghost', onclick: () => this.pokedex() },
                `📘 Pokédex ${caught}/${s.pokedex.length}`),
              s.tower ? el('button', { class: 'btn btn-ghost', onclick: () => Session.start('tour') },
                `🗼 Tour · Étage ${s.tower_floor + 1}`) : null,
              s.tower ? el('button', { class: 'btn btn-ghost', onclick: () => Arcade.hub() },
                '🎮 Mini-jeux') : null,
            ),
          ),
          this.badgeCase(),
        ),
        this.map(s),
      ),
    );
  },

  // The world map: routes grouped into their regions, with the next episode
  // glowing — a five-year-old should always see where the adventure continues.
  map(s) {
    const sections = [
      { title: '🌱 Région de Lecturia', match: (e) => e.route >= 1 && e.route <= 18 || (e.legendary && e.badge && ['glace', 'orage', 'psy', 'volcan', 'maree', 'arcenciel'].includes(e.badge)) },
      { title: '🏝 Îles Lointaines', match: (e) => (e.route >= 19 && e.route <= 24) || (e.badge && ['tonnerre', 'ciel'].includes(e.badge)) },
      { title: '🏆 Ligue Pokémon', match: () => true },
    ];
    const box = el('div', { class: 'routes', style: 'margin-top:26px' });
    const used = new Set();
    sections.forEach((sec) => {
      const eps = s.episodes.filter((e) => !used.has(e.id) && sec.match(e));
      if (!eps.length) return;
      eps.forEach((e) => used.add(e.id));
      box.append(el('div', { class: 'map-section' }, sec.title));
      eps.forEach((e) => box.append(this.routeCard(e, e.id === s.next_episode)));
    });
    return box;
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

  pokedex() {
    Voice.stop();
    this.screen(
      this.topbar(el('button', { class: 'btn-icon', onclick: () => this.home() }, '✕')),
      el('div', { class: 'page' },
        el('h2', { style: 'text-align:center;color:#e3350d' }, 'Pokédex'),
        el('div', { class: 'dexgrid' },
          this.state.pokedex.map((p) => el('button', {
            class: `dexcell ${p.caught ? '' : 'locked'}`,
            onclick: () => (p.caught ? this.pokemonDetail(p) : say('Celui-là, tu ne l\'as pas encore attrapé !')),
          },
            el('img', { class: p.shiny ? 'shiny' : '', src: p.sprite, alt: '', loading: 'lazy' }),
            p.shiny ? el('span', { class: 'shinymark' }, '✨') : null,
            el('b', {}, p.name),
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
          syllableWord(p.syllables, p.name),
          el('div', { class: 'types' }, (p.types || []).map((t) => el('span', { 'data-type': t }, t))),
          el('div', { class: 'dex' }, p.dex),
          el('button', { class: 'btn btn-ghost', onclick: () => Voice.play(p.dex_audio, p.dex) }, '🔊 Réécouter'),
        ),
      ),
    );
    Voice.play(p.name_audio, p.name).then(() => Voice.play(p.dex_audio, p.dex));
  },
};

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
              : el('p', { class: 'hint' }, 'Il faut lire au moins la moitié des mots du premier coup. Réessaie !'),
            catches.length ? el('div', { class: 'newcatch' },
              el('span', { class: 'score' }, '🎉 Un Pokémon rare de la Tour rejoint ton Pokédex !'),
              el('div', { class: 'catchrow' },
                catches.map((id) => el('img', { src: `sprites/art/${id}.png`, alt: '' }))),
            ) : null,
            el('button', { class: 'btn btn-huge', onclick: () => Session.start('tour') },
              passed ? `ÉTAGE ${towerInfo.floor + 1} ▶` : 'RÉESSAYER 🔄'),
            el('button', { class: 'btn btn-go', onclick: () => App.home() }, 'CARTE 🗺️'),
          ),
        ),
      );
      burst(passed ? '🏅' : '🗼');
      if (passed) { Sfx.fanfare(); confetti(); } else { Sfx.wrong(); }
      await say(passed ? 'Super !' : 'Essaie encore, tu y es presque.');
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
          el('button', { class: 'btn btn-huge btn-go', onclick: () => App.home() }, 'CARTE 🗺️'),
          el('button', { class: 'btn', onclick: () => Arcade.hub() }, '🎮 MINI-JEUX'),
        ),
      ),
    );
    burst('🏆');
    Sfx.fanfare();
    confetti();
    await say(stars === 3 ? 'Super !' : 'Bravo !');
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
    el('p', { class: 'hint' }, a.instruction),
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
  const counter = el('p', { class: 'hint' }, a.instruction);

  const grid = el('div', { class: 'grid' });
  a.graphemes.forEach((g) => {
    const tile = el('button', { class: 'tile' }, g.display);
    tile.addEventListener('click', () => {
      Voice.play(g.sound_audio, g.display);
      tile.classList.add('good');
      remaining.delete(g.id);
      if (remaining.size === 0) {
        go.disabled = false;
        counter.textContent = 'Bravo ! Tu peux continuer.';
      } else {
        counter.textContent = `Encore ${remaining.size} son${remaining.size > 1 ? 's' : ''} à toucher.`;
      }
    });
    grid.append(tile);
  });

  body.append(el('h2', {}, a.title), counter, grid, go);
}

function renderFusion(a, body, done) {
  let idx = 0;
  const stage = el('div', { class: 'fusion-row' });
  body.append(el('h2', {}, a.title), el('p', { class: 'hint' }, a.instruction), stage);

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
  const hint = el('p', { class: 'hint' }, a.instruction);
  const stage = el('div', { class: 'act', style: 'width:100%' });
  body.append(head, hint, stage);

  const step = () => {
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
          if (!answered) Session.record(item.id, true);
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
          if (!answered) { Session.record(item.id, false); answered = true; }
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

/* renderPhrase is the sentence workshop: the sentence's words arrive shuffled
   and the child rebuilds it in order — the battle mechanic applied to syntax.
   Every placed word is spoken, the whole sentence is read aloud at the end, and
   the hidden picture flips open as the meaning's confirmation. */
function renderPhrase(a, body, done) {
  let idx = 0;
  const head = el('h2', {}, a.title);
  const hint = el('p', { class: 'hint' }, a.instruction);
  const stage = el('div', { class: 'act', style: 'width:100%' });
  body.append(head, hint, stage);

  const step = () => {
    if (idx >= a.items.length) return done();
    const item = a.items[idx];
    const words = item.syllables; // the sentence's words, in order
    let pos = 0;
    let missed = false;
    stage.innerHTML = '';

    // The mystery picture: a Poké Ball until the sentence is rebuilt.
    const face = item.sprite ? el('img', { src: item.sprite, alt: '' })
      : item.icon ? iconEl(item.icon)
      : el('span', { class: 'em' }, item.emoji || '🎁');
    const mystery = el('div', { class: 'mystery' }, pokeball('pokeball-sm'));

    const slots = el('div', { class: 'slot-row phrase-slots' });
    words.forEach(() => slots.append(el('div', { class: 'slot word-slot' }, '')));

    const tray = el('div', { class: 'tray' });
    item.tray.forEach((word) => {
      const b = el('button', { class: 'syl-btn word-btn' }, word);
      b.addEventListener('click', async () => {
        if (b.classList.contains('used')) return;
        if (word === words[pos]) {
          Sfx.tap();
          const slot = slots.children[pos];
          slot.textContent = word;
          slot.classList.add('filled');
          b.classList.add('used');
          Voice.play(item.syllable_audio ? item.syllable_audio[pos] : null, word);
          pos++;
          if (pos === words.length) await complete();
        } else {
          missed = true;
          b.classList.add('bad');
          setTimeout(() => b.classList.remove('bad'), 450);
          Sfx.wrong();
        }
      });
      tray.append(b);
    });

    const complete = async () => {
      Session.record(item.id, !missed);
      stage.innerHTML = '';
      mystery.innerHTML = '';
      mystery.append(face);
      mystery.classList.add('open');
      stage.append(
        el('div', { class: 'sentence' }, item.prompt),
        mystery,
      );
      burst('✨');
      Sfx.merge();
      cheerCompanion();
      // The sentence he just built, read back whole: this is the moment the
      // marks on the screen become speech.
      await Voice.play(item.audio, item.prompt);
      await sleep(300);
      idx++;
      step();
    };

    stage.append(mystery, slots, tray);
  };
  step();
}

function renderCombat(a, body, done) {
  const f = a.fight;
  let hp = f.hp;
  let round = 0;
  let perfect = 0; // rounds built without a single wrong syllable

  const foeImg = el('img', { src: `sprites/art/${f.pokemon}.png`, alt: '' });
  const allyImg = companion('ally');
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
    el('div', { class: 'combat-right' }, el('p', { class: 'hint' }, a.instruction), stage),
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
          // The foe counterattacks: a lunge and a thud, drama without damage —
          // losing is not a thing that can happen to a five-year-old here.
          Sfx.thud();
          foeImg.classList.remove('lunge');
          void foeImg.offsetWidth;
          foeImg.classList.add('lunge');
          allyImg.classList.add('flinch');
          setTimeout(() => allyImg.classList.remove('flinch'), 500);
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
    Session.record(item.id, !missed);
    stage.innerHTML = '';
    if (missed) {
      Sfx.attack();
      stage.append(el('div', { class: 'shout' }, 'ATTAQUE !'));
    } else {
      // A word built without a single wrong tap lands as a critical hit.
      perfect++;
      Sfx.crit();
      stage.append(el('div', { class: 'shout crit' }, 'COUP CRITIQUE !'));
      burst('⚡');
    }
    await Voice.play(item.audio, item.prompt);
    foeImg.classList.add('hit');
    burst('💥');
    cheerCompanion();
    hp--;
    paintHP();
    await sleep(450);
    foeImg.classList.remove('hit');
    round++;
    paintScore();
    step();
  };

  const victory = async () => {
    // finish() needs the fight's tally to score a tower floor.
    Session.fightStats = { perfect, rounds: round };
    Sfx.faint();
    foeImg.classList.add('faint');
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

  intro();
}

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
  Voice.play(b.audio, `Tu as gagné le ${b.name} !`);
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
      syllableWord(r.syllables, r.name),
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
   The mini-games corner: four timed games with stored records to beat. Two are
   pure celebration (memory, la chasse), two are reading in disguise (lecture,
   l'oreille fine) — those also report their answers so the spaced repetition
   learns from play. Nothing here blocks or advances the course. */

const Arcade = {
  timer: null,     // the game's interval/timeout handle
  pool: null,      // learned words + syllables from /api/arcade
  items: [],       // spaced-repetition answers collected by comprehension games

  GAMES: [
    { id: 'memory', icon: '🃏', name: 'Memory Pokémon', desc: 'Retrouve les paires le plus vite possible.', lower: true, unit: 's' },
    { id: 'chasse', icon: '🌿', name: 'La chasse', desc: 'Attrape les Pokémon avant qu\'ils se cachent. 45 secondes !', lower: false, unit: '' },
    { id: 'lecture', icon: '📖', name: 'Lis et attrape', desc: 'Lis le mot, touche la bonne image. 60 secondes !', lower: false, unit: '' },
    { id: 'oreille', icon: '👂', name: 'L\'oreille fine', desc: 'Écoute la syllabe, touche-la. 60 secondes !', lower: false, unit: '' },
  ],

  stop() {
    if (this.timer) { clearInterval(this.timer); clearTimeout(this.timer); this.timer = null; }
  },

  caughtIds() {
    return ((App.state && App.state.pokedex) || []).filter((p) => p.caught).map((p) => p.id);
  },

  record(id) {
    return (App.state && App.state.records && App.state.records[id]) || null;
  },

  /* The hub: pick a game, see the record to beat. */
  async hub() {
    Voice.stop();
    this.stop();
    if (!this.pool) {
      try { this.pool = await api('api/arcade'); } catch (_) { this.pool = { words: [], syllables: [] }; }
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
          el('p', { class: 'hint' }, 'Joue et bats tes records !'),
          el('div', { class: 'game-grid' }, cards),
        ),
      ),
    );
  },

  screen(title, hintText, ...rest) {
    this.stop();
    App.screen(
      App.topbar(el('button', { class: 'btn-icon', onclick: () => { this.stop(); this.hub(); } }, '✕')),
      el('div', { class: 'page' },
        el('div', { class: 'act' },
          el('h2', {}, title),
          hintText ? el('p', { class: 'hint' }, hintText) : null,
          ...rest,
        ),
      ),
    );
  },

  /* A visible countdown bar; onDone fires when time is up. */
  countdown(seconds, onDone) {
    const fill = el('i');
    const bar = el('div', { class: 'timebar' }, fill);
    let left = seconds;
    fill.style.width = '100%';
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
    let beaten = false;
    try {
      const res = await api('api/records', { game, score, lower, items: this.items });
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
          el('div', { class: 'bigscore' }, `${score}${g.unit}`),
          beaten ? el('div', { class: 'score newrecord' }, '🏆 NOUVEAU RECORD !') : el('div', { class: 'score' }, `Record : ${this.record(game)}${g.unit}`),
          el('button', { class: 'btn btn-huge', onclick: () => this[game]() }, 'REJOUER 🔄'),
          el('button', { class: 'btn btn-ghost', onclick: () => this.hub() }, '🎮 Mini-jeux'),
          el('button', { class: 'btn btn-go', onclick: () => App.home() }, 'CARTE 🗺️'),
        ),
      ),
    );
    if (beaten) { confetti(90); burst('🏆'); Sfx.fanfare(); await say('Génial !'); }
    else { burst('🎉'); Sfx.correct(); await say('Bravo !'); }
  },

  /* Memory: six pairs of his Pokémon, the record is the time. */
  memory() {
    const ids = this.caughtIds();
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
          if (matched === picks.length) this.gameOver('memory', seconds, true, '🃏 Memory Pokémon');
        } else {
          lock = true;
          const other = open;
          open = null;
          Sfx.wrong();
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
    const ids = this.caughtIds();
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
        score++;
        scoreChip.textContent = `Attrapés : ${score}`;
        Sfx.pop();
        burst('✨');
      });
      holes.push(hole);
      grid.append(hole);
    }

    let elapsed = 0;
    const pop = () => {
      if (active) { active.classList.remove('show'); active = null; }
      const hole = holes[Math.floor(Math.random() * holes.length)];
      const id = ids[Math.floor(Math.random() * ids.length)] || 25;
      $('img', hole).src = `sprites/art/${id}.png`;
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

    this.screen('🌿 La chasse', 'Touche le Pokémon avant qu\'il se cache !', scoreChip, bar, grid);
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
            if (!answered) { this.items.push({ id: `mot:${target.text}`, correct: true }); score++; }
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
            Sfx.wrong();
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
            if (!answered) { this.items.push({ id: `syl:${target.text}`, correct: true }); score++; }
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
            Sfx.wrong();
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
