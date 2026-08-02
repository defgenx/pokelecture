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

const Voice = {
  cache: new Map(),
  current: null,

  unlock() {
    // Called from the first real tap. Touching speechSynthesis here also warms
    // up the voice list, which Android populates lazily.
    if ('speechSynthesis' in window) window.speechSynthesis.getVoices();
  },

  element(url) {
    let a = this.cache.get(url);
    if (!a) {
      a = new Audio(url);
      a.preload = 'auto';
      this.cache.set(url, a);
    }
    return a;
  },

  stop() {
    if (this.current) {
      try { this.current.pause(); } catch (_) {}
      this.current = null;
    }
    if ('speechSynthesis' in window) window.speechSynthesis.cancel();
  },

  play(url, text) {
    this.stop();
    if (!url) return this.speak(text);

    return new Promise((resolve) => {
      const a = this.element(url);
      let settled = false;
      const finish = (fallback) => {
        if (settled) return;
        settled = true;
        a.removeEventListener('ended', onEnd);
        a.removeEventListener('error', onErr);
        if (fallback) this.speak(text).then(resolve);
        else resolve();
      };
      const onEnd = () => finish(false);
      const onErr = () => finish(true);

      a.addEventListener('ended', onEnd);
      a.addEventListener('error', onErr);
      try { a.currentTime = 0; } catch (_) {}
      this.current = a;
      a.play().catch(() => finish(true));
      // Safety net: a stalled element must not freeze the game.
      setTimeout(() => finish(false), 12000);
    });
  },

  speak(text) {
    if (!text || !('speechSynthesis' in window)) return Promise.resolve();
    return new Promise((resolve) => {
      const u = new SpeechSynthesisUtterance(text);
      u.lang = 'fr-FR';
      u.rate = 0.85;
      const fr = window.speechSynthesis.getVoices().find((v) => v.lang && v.lang.startsWith('fr'));
      if (fr) u.voice = fr;
      u.onend = () => resolve();
      u.onerror = () => resolve();
      window.speechSynthesis.speak(u);
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

const PRAISE = ['Bravo !', 'Super !'];
let praiseIdx = 0;

// say() prefers the recorded phrase from the session payload and degrades to the
// browser voice when the audio track has not been generated yet.
function say(phrase) {
  const url = Session.data && Session.data.ui ? Session.data.ui[phrase] : null;
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
    const s = this.state;
    const next = s.episodes.find((e) => e.id === s.next_episode);
    const caught = s.pokedex.filter((p) => p.caught).length;

    this.screen(
      this.topbar(
        el('span', { class: 'chip' }, `⭐ ${s.stars}`),
        s.streak > 1 ? el('span', { class: 'chip' }, `🔥 ${s.streak}`) : null,
      ),
      el('div', { class: 'page' },
        el('div', { class: 'act' },
          el('h2', {}, next ? next.title : 'Bravo !'),
          el('button', { class: 'btn btn-huge', onclick: () => Session.start(s.next_episode) },
            next && next.done > 0 ? 'REJOUER ▶' : 'JOUER ▶'),
          el('button', { class: 'btn btn-ghost', onclick: () => this.pokedex() },
            `📘 Pokédex  ${caught}/${s.pokedex.length}`),
        ),
        el('div', { class: 'routes', style: 'margin-top:26px' },
          s.episodes.map((e) => this.routeCard(e)),
        ),
      ),
    );
  },

  routeCard(e) {
    const stars = '★'.repeat(e.stars) + '☆'.repeat(3 - e.stars);
    return el('button', {
      class: `route ${e.unlocked ? '' : 'locked'} ${e.legendary ? 'legend' : ''}`,
      onclick: () => (e.unlocked ? Session.start(e.id) : Voice.speak('Termine la route précédente !')),
    },
      el('span', { class: 'no' }, e.legendary ? '★' : e.route),
      el('img', { src: `sprites/art/${e.reward_pokemon}.png`, alt: '', loading: 'lazy' }),
      el('div', { class: 't' },
        el('b', {}, e.unlocked ? e.title : e.legendary ? 'Arène légendaire — 🔒' : `Route ${e.route} — 🔒`),
        el('div', { class: 'sounds' }, (e.sounds || []).map((g) => el('i', {}, g))),
      ),
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
            onclick: () => (p.caught ? this.pokemonDetail(p) : Voice.speak('Celui-là, tu ne l\'as pas encore attrapé !')),
          },
            el('img', { src: p.sprite, alt: '', loading: 'lazy' }),
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
          el('img', { class: 'art', src: p.art, alt: '' }),
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

  async start(id) {
    Voice.stop();
    App.screen(el('div', { class: 'loading' }, pokeball('spin-fast')));
    this.data = await api(`api/session/${id}`);
    this.i = 0;
    this.answers = [];
    this.right = 0;
    this.wrong = 0;
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

    // The buddy tags along on the drill screens; the battle and the catch have
    // their own staging and would only be cluttered by it.
    if (!['combat', 'recompense', 'story'].includes(a.kind)) {
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
      default: return renderQuiz(a, body, done);
    }
  },

  async finish() {
    const total = this.right + this.wrong;
    const ratio = total ? this.right / total : 1;
    const stars = ratio >= 0.9 ? 3 : ratio >= 0.7 ? 2 : 1;

    const res = await api(`api/session/${this.data.episode_id}/result`, {
      items: this.answers,
      stars,
      completed: true,
    });
    App.state = res.state;

    App.screen(
      App.topbar(),
      el('div', { class: 'page' },
        el('div', { class: 'act' },
          el('div', { class: 'bigstars' }, '★'.repeat(stars) + '☆'.repeat(3 - stars)),
          el('div', { class: 'score' }, `${this.right} bonne${this.right > 1 ? 's' : ''} réponse${this.right > 1 ? 's' : ''} sur ${total}`),
          res.new_catch ? el('div', { class: 'score' }, '🎉 Nouveau Pokémon dans le Pokédex !') : null,
          el('button', { class: 'btn btn-huge btn-go', onclick: () => App.home() }, 'CARTE 🗺️'),
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
    const isText = !!(item.emoji || item.sprite) || a.kind === 'phrase';
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

    const grid = el('div', { class: `grid ${item.choices.length === 2 ? 'two' : ''}` });
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
  if (f.legendary) {
    Sfx.legendary();
    burst('⚡');
  } else {
    Sfx.tap();
  }
  let hp = f.hp;
  let round = 0;

  const foeImg = el('img', { src: `sprites/art/${f.pokemon}.png`, alt: '' });
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
      el('div', { class: 'platform' }, companion('ally')),
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
    Sfx.attack();
    stage.append(el('div', { class: 'shout' }, 'ATTAQUE !'));
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
    await sleep(400);
    done();
  };

  step();
}

function renderReward(a, body, done) {
  const r = a.reward;
  const stage = el('div', { class: 'reveal' });
  body.append(el('h2', {}, a.title), stage);

  stage.append(el('div', { class: 'catch-ball' }, pokeball()));
  burst('✨');
  Sfx.catch_();
  confetti(70);

  setTimeout(async () => {
    stage.innerHTML = '';
    stage.append(
      el('img', { class: 'art', src: `sprites/art/${r.pokemon}.png`, alt: '' }),
      syllableWord(r.syllables, r.name),
      el('div', { class: 'types' }, (r.types || []).map((t) => el('span', { 'data-type': t }, t))),
      el('div', { class: 'dex' }, r.dex),
      el('button', { class: 'btn btn-huge btn-go', onclick: done }, 'TERMINER ✓'),
    );
    burst('🎉');
    await Voice.play(r.name_audio, r.name);
    await Voice.play(r.dex_audio, r.dex);
  }, 1500);
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

// A service worker (and therefore a real offline PWA) needs a secure context.
// Over plain http on the LAN this is a no-op; add mkcert TLS and it kicks in.
if ('serviceWorker' in navigator && window.isSecureContext) {
  navigator.serviceWorker.register('sw.js').catch(() => {});
}
