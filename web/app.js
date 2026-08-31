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
  cancelled: 0,

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
      // Safety net: a stalled element must not freeze the game.
      setTimeout(() => finish(false), 12000);
    });
  },

  speak(text) {
    if (!text || !('speechSynthesis' in window)) return Promise.resolve();
    const synth = window.speechSynthesis;
    return new Promise((resolve) => {
      const go = () => {
        const u = new SpeechSynthesisUtterance(text);
        u.lang = 'fr-FR';
        u.rate = 0.85;
        const list = this.voices.length ? this.voices : synth.getVoices() || [];
        // Android reports fr_FR, desktop fr-FR.
        const fr = list.find((v) => v.lang && v.lang.replace('_', '-').startsWith('fr'));
        if (fr) u.voice = fr;
        u.onend = () => resolve();
        u.onerror = () => resolve();
        synth.speak(u);
      };
      // Android drops an utterance queued in the same task as a cancel(), which
      // is every line of the game: play() stops the previous one first.
      const wait = Date.now() - this.cancelled < 250 ? 150 : 0;
      if (wait) setTimeout(go, wait);
      else go();
      setTimeout(resolve, 9000);
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
        el('div', { class: 'act' },
          el('h2', {}, s.all_done ? '🏆 Tu es le Champion !' : next ? next.title : 'Bravo !'),
          el('button', { class: 'btn btn-huge', onclick: () => Session.start(s.next_episode) },
            next && next.done > 0 ? 'REJOUER ▶' : 'JOUER ▶'),
          el('button', { class: 'btn btn-ghost', onclick: () => this.pokedex() },
            `📘 Pokédex  ${caught}/${s.pokedex.length}`),
          s.tower ? el('button', { class: 'btn btn-ghost', onclick: () => Session.start('tour') },
            '🗼 Tour de Combat') : null,
        ),
        this.badgeCase(),
        el('div', { class: 'routes', style: 'margin-top:26px' },
          s.episodes.map((e) => this.routeCard(e)),
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

  routeCard(e) {
    const stars = '★'.repeat(e.stars) + '☆'.repeat(3 - e.stars);
    const conseil = e.legendary && !e.badge;
    return el('button', {
      class: `route ${e.unlocked ? '' : 'locked'} ${e.legendary ? 'legend' : ''}`,
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

  render() {
    Voice.stop();
    const a = this.data.activities[this.i];
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
      default: return renderQuiz(a, body, done);
    }
  },

  async finish() {
    const total = this.right + this.wrong;
    const ratio = total ? this.right / total : 1;
    const stars = ratio >= 0.9 ? 3 : ratio >= 0.7 ? 2 : 1;
    const tower = this.data.episode_id === 'tour';

    // The tower is practice: its answers feed the review boxes but it never
    // completes anything, awards no stars and catches nothing.
    const res = await api(`api/session/${this.data.episode_id}/result`, {
      items: this.answers,
      stars: tower ? 0 : stars,
      completed: !tower,
      shiny: this.shiny,
    });
    App.state = res.state;

    if (tower) {
      App.screen(
        App.topbar(),
        el('div', { class: 'page' },
          el('div', { class: 'act' },
            el('h2', {}, '🗼 Tour de Combat'),
            el('div', { class: 'score' }, `${this.right} mot${this.right > 1 ? 's' : ''} sur ${total} du premier coup`),
            el('button', { class: 'btn btn-huge', onclick: () => Session.start('tour') }, 'REVANCHE 🔄'),
            el('button', { class: 'btn btn-go', onclick: () => App.home() }, 'CARTE 🗺️'),
          ),
        ),
      );
      burst('🗼');
      Sfx.fanfare();
      await say('Bravo !');
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
          el('button', { class: 'btn', onclick: () => Bonus.start() }, '🎮 JEU BONUS'),
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
          await praise();
          if (isText) await Voice.play(item.audio, item.prompt);
          await sleep(250);
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

  const stage = el('div', { class: 'act', style: 'width:100%' });
  body.append(
    el('h2', { class: f.legendary ? 'legend-title' : '' }, f.legendary ? '⚡ COMBAT LÉGENDAIRE ⚡' : a.title),
    arena,
    el('p', { class: 'hint' }, a.instruction),
    stage,
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
    await sleep(600);
    foeImg.classList.remove('hit');
    round++;
    step();
  };

  const victory = async () => {
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

/* ---------------------------------------------------------------- bonus ---
   Two tiny no-reading games offered after a finished session: pure celebration,
   built from his own catches so filling the Pokédex makes the games richer.
   Nothing here is recorded as progress — this is dessert, not dinner. */

const Bonus = {
  timer: null,

  caughtIds() {
    return ((App.state && App.state.pokedex) || []).filter((p) => p.caught).map((p) => p.id);
  },

  start() {
    Voice.stop();
    (Math.random() < 0.5 ? this.memory : this.chase).call(this);
  },

  stop() {
    if (this.timer) { clearInterval(this.timer); this.timer = null; }
  },

  screen(title, hintText, stage) {
    this.stop();
    App.screen(
      App.topbar(el('button', { class: 'btn-icon', onclick: () => { this.stop(); App.home(); } }, '✕')),
      el('div', { class: 'page' },
        el('div', { class: 'act' },
          el('h2', {}, title),
          el('p', { class: 'hint' }, hintText),
          stage,
        ),
      ),
    );
  },

  async win() {
    this.stop();
    confetti(70);
    burst('🎉');
    Sfx.fanfare();
    await say('Super !');
    App.home();
  },

  /* Memory: three pairs of his own Pokémon, face down. */
  memory() {
    const ids = this.caughtIds();
    ids.sort(() => Math.random() - 0.5);
    const picks = ids.slice(0, 3);
    while (picks.length < 3) picks.push(25); // fresh save: pad with Pikachu
    const cards = [...picks, ...picks].sort(() => Math.random() - 0.5);

    const stage = el('div', { class: 'memory-grid' });
    let open = null;      // the one face-up unmatched card
    let lock = false;     // ignore taps while a wrong pair is shown
    let matched = 0;

    cards.forEach((id) => {
      const card = el('button', { class: 'memory-card' },
        el('span', { class: 'face back' }, pokeball('pokeball-sm')),
        el('span', { class: 'face front' }, el('img', { src: `sprites/art/${id}.png`, alt: '' })),
      );
      card.dataset.id = id;
      card.addEventListener('click', async () => {
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
          if (matched === picks.length) await this.win();
        } else {
          lock = true;
          const other = open;
          open = null;
          Sfx.wrong();
          setTimeout(() => {
            other.classList.remove('up');
            card.classList.remove('up');
            lock = false;
          }, 900);
        }
      });
      stage.append(card);
    });

    this.screen('Memory Pokémon', 'Retrouve les paires !', stage);
  },

  /* The chase: Pokémon pop out of the tall grass, tap them before they hide. */
  chase() {
    const ids = this.caughtIds();
    const GOAL = 8;
    let score = 0;
    let active = null; // the hole currently showing a Pokémon

    const tally = el('div', { class: 'chase-tally' },
      Array.from({ length: GOAL }, () => el('i')));
    const paintTally = () => {
      [...tally.children].forEach((n, i) => n.classList.toggle('got', i < score));
    };

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
        paintTally();
        Sfx.pop();
        burst('✨');
        cheerCompanion();
        if (score >= GOAL) this.win();
      });
      holes.push(hole);
      grid.append(hole);
    }
    paintTally();

    const stage = el('div', { class: 'act', style: 'width:100%' }, tally, grid);
    this.screen('Attrape-les !', 'Touche le Pokémon avant qu\'il se cache !', stage);

    this.timer = setInterval(() => {
      if (active) { active.classList.remove('show'); active = null; }
      const hole = holes[Math.floor(Math.random() * holes.length)];
      const id = ids[Math.floor(Math.random() * ids.length)] || 25;
      $('img', hole).src = `sprites/art/${id}.png`;
      hole.classList.add('show');
      active = hole;
      Sfx.tap();
    }, 1100);
  },
};

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
