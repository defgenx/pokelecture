'use strict';

/* The Pokémon-game layer on top of the course: the partner (a starter chosen
   on first launch, levelled by reading and evolving on screen), the daily
   gift, the Poké Ball bag and the tall grass where wild Pokémon are caught by
   reading their name. Loaded after app.js; App, Session and Arcade call in. */

/* ------------------------------------------------------------- partner --- */

const Partner = {
  get() { return App.state && App.state.partner; },

  /* The starter choice, the iconic first screen: four Poké Balls on a table,
     each opening on its Pokémon. Pick one and it follows you everywhere. */
  choose() {
    const s = App.state;
    let picked = null;
    const info = el('div', { class: 'starter-info' });
    const confirm = el('button', { class: 'btn btn-huge btn-go', disabled: true }, 'JE TE CHOISIS ! ▶');
    const table = el('div', { class: 'starter-table' },
      (s.starters || []).map((p) => {
        const card = el('button', { class: 'starter' },
          el('img', { src: `sprites/art/${p.species}.png`, alt: '' }),
          pokeball('starter-ball'),
        );
        card.addEventListener('click', () => {
          table.querySelectorAll('.starter').forEach((n) => n.classList.remove('picked'));
          card.classList.add('picked');
          picked = p;
          Sfx.pop();
          info.innerHTML = '';
          info.append(
            syllableWord(null, p.name, [p.name_audio]),
            el('div', { class: 'types' }, (p.types || []).map((t) => el('span', { 'data-type': t }, t))),
          );
          confirm.disabled = false;
          Voice.play(p.name_audio, p.name);
        });
        return card;
      }),
    );
    confirm.addEventListener('click', async () => {
      if (!picked) return;
      confirm.disabled = true;
      try {
        const res = await api('api/partner', { species: picked.species });
        App.state = res.state;
      } catch (_) { return App.home(); }
      Sfx.catch_();
      confetti(80);
      await Voice.play(picked.chosen.audio, picked.chosen.text);
      Gift.maybe();
    });

    App.screen(
      App.topbar(),
      el('div', { class: 'page' },
        el('div', { class: 'act' },
          el('h2', {}, 'Choisis ton premier Pokémon !'),
          // The prompt below is read out in full (s.starter_prompt).
          hint('Il grandit à chaque mot que tu lis.', false),
          table, info, confirm,
        ),
      ),
    );
    Voice.play(s.starter_prompt.audio, s.starter_prompt.text);
  },

  /* The home-screen card: who, what level, how far to the next one, and the
     promise of the next evolution — the reason to read one more route. */
  card() {
    const p = this.get();
    if (!p) return null;
    const pct = Math.round((p.xp / p.need) * 100);
    return el('button', { class: `partner-card ${App.state.hungry ? 'hungry' : ''}`, onclick: () => this.interact() },
      el('img', { class: 'partner-img', src: `sprites/art/${p.species}.png`, alt: '' }),
      el('div', { class: 'partner-meta' },
        el('div', { class: 'partner-name' }, el('b', {}, p.name), el('span', { class: 'lvl' }, `N.${p.level}`)),
        el('div', { class: 'xpbar' }, el('i', { style: `width:${pct}%` })),
        App.state.hungry ? el('small', { class: 'hungry-note' }, '🍓 Il a faim ! Touche-le.')
          : p.next_species
          ? el('small', {}, `Évolue au niveau ${p.next_level} ✨`)
          : el('small', {}, 'Forme finale ! 💪'),
      ),
    );
  },

  detail() {
    const p = this.get();
    if (!p) return;
    Voice.play(p.name_audio, p.name);
    cheerCompanion();
    document.querySelectorAll('.partner-img').forEach((n) => {
      n.classList.remove('cheer');
      void n.offsetWidth;
      n.classList.add('cheer');
    });
  },

  /* What an award did to the partner, staged: the XP bar fills, a level-up
     banner, and — the big one — the evolution scene. Resolves when done. */
  async celebrate(ev) {
    if (!ev) return;
    if (ev.to) return this.evolve(ev);
    if (ev.to_level > ev.from_level && ev.level_up) {
      const p = this.get();
      const toast = el('div', { class: 'levelup' },
        el('img', { src: `sprites/art/${p ? p.species : 25}.png`, alt: '' }),
        el('div', {}, el('b', {}, `N.${ev.to_level} !`), el('small', {}, ev.level_up.text)),
      );
      $('#fx').append(toast);
      Sfx.levelUp();
      await Voice.play(ev.level_up.audio, ev.level_up.text);
      await sleep(600);
      toast.classList.add('gone');
      setTimeout(() => toast.remove(), 500);
    }
  },

  /* "Quoi ? Salamèche évolue !" — the old and new silhouettes swap faster
     and faster in white, a flash, then the new form in colour. */
  evolve(ev) {
    return new Promise((resolve) => {
      const from = el('img', { class: 'evo-from', src: `sprites/art/${ev.from}.png`, alt: '' });
      const to = el('img', { class: 'evo-to', src: `sprites/art/${ev.to}.png`, alt: '' });
      const caption = el('div', { class: 'evo-caption' }, ev.evolving.text);
      const stage = el('div', { class: 'evo-stage' }, from, to);
      const scene = el('div', { class: 'evo-scene' }, stage, caption);
      $('#fx').append(scene);
      Sfx.evolve();
      Voice.play(ev.evolving.audio, ev.evolving.text);

      setTimeout(() => stage.classList.add('morph'), 1200);
      setTimeout(async () => {
        scene.classList.add('flash');
        stage.classList.remove('morph');
        stage.classList.add('done');
        caption.textContent = ev.evolved.text;
        Sfx.fanfare();
        confetti(120);
        await Voice.play(ev.evolved.audio, ev.evolved.text);
        if (ev.new_catch) {
          caption.append(el('div', { class: 'evo-new' }, '📘 Nouveau dans ta collection !'));
          await say('Tu as attrapé un nouveau Pokémon !');
        }
        const ok = el('button', { class: 'btn btn-huge btn-go' }, 'GÉNIAL ! ▶');
        ok.addEventListener('click', () => { scene.remove(); resolve(); });
        scene.append(ok);
      }, 5200);
    });
  },

  /* A compact "+XP" line for result screens, filling as it appears. */
  gainRow(ev) {
    const p = this.get();
    if (!ev || !p) return null;
    const pct = Math.round((p.xp / p.need) * 100);
    const fill = el('i', { style: 'width:0%' });
    setTimeout(() => { fill.style.width = `${pct}%`; }, 250);
    return el('div', { class: 'xp-gain' },
      el('img', { src: `sprites/art/${p.species}.png`, alt: '' }),
      el('div', { class: 'partner-meta' },
        el('div', { class: 'partner-name' }, el('b', {}, p.name), el('span', { class: 'lvl' }, `N.${p.level}`)),
        el('div', { class: 'xpbar' }, fill),
        el('small', {}, `+${ev.xp} XP`),
      ),
    );
  },
};

/* ---------------------------------------------------------------- gift --- */

const Gift = {
  /* Once a day: a wrapped present on the home screen, three Poké Balls inside.
     Coming back tomorrow is the habit that teaches reading. */
  async maybe() {
    if (!App.state || !App.state.gift) return App.home();
    const box = el('button', { class: 'gift-box' }, '🎁');
    const stage = el('div', { class: 'reveal' }, box, hint('Touche le cadeau !', false));
    App.screen(App.topbar(), el('div', { class: 'page' },
      el('div', { class: 'act' }, el('h2', {}, '🎁 Cadeau du jour !'), stage)));
    say('Cadeau du jour !').then(() => say('Touche le cadeau !'));
    box.addEventListener('click', async () => {
      box.disabled = true;
      box.classList.add('open');
      Sfx.catch_();
      let res;
      try { res = await api('api/gift', {}); App.state = res.state; } catch (_) { return App.home(); }
      await sleep(500);
      stage.innerHTML = '';
      const balls = el('div', { class: 'ball-row' });
      for (let i = 0; i < res.balls; i++) balls.append(pokeball('ball-pop'));
      stage.append(balls,
        el('div', { class: 'score' }, `${res.balls} Poké Balls !`),
        el('button', { class: 'btn btn-huge btn-go', onclick: () => App.home() }, 'SUPER ! ▶'));
      confetti(60);
      await say('Tu gagnes trois Poké Balls !');
    }, { once: true });
  },
};

/* ----------------------------------------------------------- tall grass --- */

const Wild = {
  pool: null,

  available() { return ((this.pool && this.pool.pokemon) || []).filter((p) => p.available); },

  async load() {
    try { this.pool = await api('api/wild'); } catch (_) { this.pool = null; }
    return this.pool;
  },

  /* The grass field: tufts swaying in the wind; touch one and something
     rustles out. The bag shows the Poké Balls left. */
  async enter() {
    Voice.stop();
    if (!(await this.load())) return App.home();
    const pool = this.pool;
    const avail = this.available();
    const caught = avail.filter((p) => p.caught).length;
    const field = el('div', { class: 'grass-field' });

    const body = [];
    if (!avail.length) {
      body.push(hint('Il n\'y a personne ici. Apprends de nouveaux sons pour trouver des Pokémon !'));
    } else if (pool.balls <= 0) {
      body.push(el('div', { class: 'empty-bag' }, pokeball('pokeball-sm'), ' ✕ 0'),
        hint('Tu n\'as plus de Poké Balls ! Lis une route pour en gagner.'),
        el('button', { class: 'btn btn-huge', onclick: () => Session.start(App.state.next_episode) }, 'JOUER ▶'));
    } else {
      for (let i = 0; i < 9; i++) {
        const tuft = el('button', { class: 'tuft', style: `animation-delay:${(i % 3) * 0.3 + Math.random() * 0.4}s` }, '🌿');
        tuft.addEventListener('click', () => {
          if (field.classList.contains('busy')) return;
          field.classList.add('busy');
          tuft.classList.add('rustle');
          Sfx.rustle();
          setTimeout(() => this.encounter(), 900);
        });
        field.append(tuft);
      }
      body.push(field, hint('Touche les herbes. Qui s\'y cache ?'));
    }

    App.screen(
      App.topbar(
        el('span', { class: 'chip' }, pokeball('pokeball-xs'), ` ${pool.balls}`),
        el('button', { class: 'btn-icon', onclick: () => App.home() }, '✕'),
      ),
      el('div', { class: 'page' },
        el('div', { class: 'act grass-act' },
          el('h2', {}, '🌿 Les hautes herbes'),
          el('div', { class: 'wild-count' }, `Attrapés : ${caught} / ${avail.length}`),
          ...body,
          el('button', { class: 'btn btn-ghost', onclick: () => App.pokedex('wild') }, '📘 Pokédex sauvage'),
        ),
      ),
    );
  },

  /* Uncaught Pokémon come out first, so the collection always grows; once
     all are caught, any of them — with the odd shiny. */
  pick() {
    const avail = this.available();
    const fresh = avail.filter((p) => !p.caught);
    const from = fresh.length && Math.random() < 0.8 ? fresh : avail;
    const p = from[Math.floor(Math.random() * from.length)];
    const shiny = Math.random() < 1 / (this.pool.shiny_odds || 16);
    return { p, shiny };
  },

  /* The encounter: read the name, syllable by syllable, to weaken it — then
     throw the ball. A name built without a slip is caught first wobble. */
  encounter() {
    const { p, shiny } = this.pick();
    let pos = 0;
    let slips = 0;
    const known = this.pool.traps || [];
    const trapAudio = this.pool.trap_audio || [];

    const sprite = el('img', { class: `wild-sprite ${shiny ? 'shiny' : ''}`, src: p.sprite, alt: '' });
    const ballSlot = el('div', { class: 'throw-zone' });
    const slots = el('div', { class: 'slot-row' });
    p.syllables.forEach(() => slots.append(el('div', { class: 'slot' }, '')));

    const inName = new Set(p.syllables.map((x) => x.toLowerCase()));
    const traps = known.filter((x) => !inName.has(x.toLowerCase())).sort(() => Math.random() - 0.5);
    const tray = [...p.syllables, ...traps.slice(0, p.syllables.length >= 3 ? 2 : 1)].sort(() => Math.random() - 0.5);
    const trayEl = el('div', { class: 'tray' });
    const word = el('div', { class: 'word arcade-word wild-name' }, p.name);
    const flee = el('button', { class: 'btn btn-ghost', onclick: () => this.enter() }, '🏃 Fuir');

    tray.forEach((syl) => {
      const b = el('button', { class: 'syl-btn' }, syl);
      b.addEventListener('click', () => {
        if (b.classList.contains('used') || pos >= p.syllables.length) return;
        if (syl === p.syllables[pos]) {
          const slot = slots.children[pos];
          slot.textContent = syl;
          slot.classList.add('filled');
          b.classList.add('used');
          Voice.play(p.syllable_audio[pos], syl);
          pos++;
          sprite.classList.remove('hit');
          void sprite.offsetWidth;
          sprite.classList.add('hit');
          Sfx.tap();
          if (pos === p.syllables.length) setTimeout(ready, 450);
        } else {
          slips++;
          b.classList.add('bad');
          setTimeout(() => b.classList.remove('bad'), 450);
          Sfx.wrong();
          const i = known.indexOf(syl);
          Voice.play(i >= 0 ? trapAudio[i] : null, syl);
        }
      });
      trayEl.append(b);
    });

    const ready = async () => {
      trayEl.remove();
      flee.remove();
      burst('⭐');
      Sfx.correct();
      await Voice.play(p.name_audio, p.name);
      this.offerThrow(p, shiny, slips, sprite, ballSlot, 1);
    };

    // Same two columns as the battle: in landscape the Pokémon stays on the
    // left and the reading on the right, so nothing scrolls below the fold.
    App.screen(
      App.topbar(
        el('span', { class: 'chip bag-chip' }, pokeball('pokeball-xs'), ` ${this.pool.balls}`),
      ),
      el('div', { class: 'page' },
        el('div', { class: 'act' },
          el('h2', {}, shiny ? '✨ Un Pokémon chromatique sauvage !' : 'Un Pokémon sauvage apparaît !'),
          el('div', { class: 'combat-cols' },
            el('div', { class: 'combat-left' }, el('div', { class: 'wild-arena' }, el('div', { class: 'platform' }, sprite))),
            el('div', { class: 'combat-right act' }, word, slots, trayEl, ballSlot, flee),
          ),
        ),
      ),
    );
    Sfx.encounter();
    say(shiny ? 'Ouah ! Un Pokémon chromatique !' : 'Un Pokémon sauvage apparaît !')
      .then(() => say('Lis son nom pour l\'attraper !'));
  },

  // The big ball to throw — a real button, sized for a finger.
  offerThrow(p, shiny, slips, sprite, zone, attempt) {
    zone.innerHTML = '';
    const supers = (App.state.bag || {}).superball || 0;
    const row = el('div', { class: 'throw-row' });
    const make = (isSuper) => {
      const ball = el('button', { class: `throw-ball ${isSuper ? 'super' : ''}`, 'aria-label': isSuper ? 'Super Ball' : 'Poké Ball' },
        pokeball(`throw-ball-icon ${isSuper ? 'superball' : ''}`),
        el('b', {}, isSuper ? `SUPER BALL ×${supers}` : attempt > 1 ? 'RELANCE !' : 'LANCE !'));
      ball.addEventListener('click', () => this.throwBall(p, shiny, slips, sprite, zone, attempt, isSuper), { once: true });
      return ball;
    };
    if ((App.state.balls || 0) > 0) row.append(make(false));
    if (supers > 0) row.append(make(true));
    zone.append(row, supers > 0 ? hint('Choisis ta Poké Ball ! La Super Ball attrape mieux.')
      : attempt > 1 ? hint('Relance vite une Poké Ball !') : hint('Lance la Poké Ball !'));
  },

  /* One throw. The server rolls the catch (reading the name well makes it
     likely); the animation then plays that outcome: a catch clicks shut, a
     miss bursts open and the Pokémon jumps back out. Three misses and it
     runs away. */
  async throwBall(p, shiny, slips, sprite, zone, attempt, isSuper) {
    zone.innerHTML = '';
    let res = null;
    try {
      res = await api('api/wild/catch', { id: p.id, shiny, slips, attempt, super: !!isSuper });
      App.state = res.state;
    } catch (_) {
      return this.enter();
    }
    if (this.pool) this.pool.balls = App.state.balls;
    const bag = $('.bag-chip');
    if (bag) bag.lastChild.textContent = ` ${App.state.balls}`;

    Sfx.throw_();
    const ball = pokeball(`flying-ball ${isSuper ? 'superball' : ''}`);
    sprite.parentNode.append(ball);
    await sleep(600);
    sprite.classList.remove('escaped');
    sprite.classList.add('absorbed');
    Sfx.absorb();
    await sleep(500);
    ball.classList.add('landed');
    const wobbles = res.caught ? (slips === 0 ? 1 : 3) : 1 + Math.floor(Math.random() * 3);
    for (let i = 0; i < wobbles; i++) {
      ball.classList.remove('wobble');
      void ball.offsetWidth;
      ball.classList.add('wobble');
      Sfx.wobble();
      await sleep(750);
    }

    if (!res.caught) {
      ball.classList.add('burst-open');
      Sfx.escape();
      sprite.classList.remove('absorbed');
      sprite.classList.add('escaped');
      setTimeout(() => ball.remove(), 500);
      if (attempt >= 3) {
        sprite.classList.add('fled');
        await say('Oh non, il s\'est enfui !');
        zone.append(el('div', { class: 'home-row' },
          el('button', { class: 'btn btn-huge btn-go', onclick: () => this.enter() }, 'ENCORE 🌿'),
          el('button', { class: 'btn btn-ghost', onclick: () => App.home() }, 'CARTE 🗺️')));
        return;
      }
      await say('Oh non ! Il est sorti de la Poké Ball !');
      if ((App.state.balls || 0) <= 0 && !((App.state.bag || {}).superball > 0)) {
        zone.append(hint('Tu n\'as plus de Poké Balls ! Lis une route pour en gagner.'),
          el('button', { class: 'btn btn-ghost', onclick: () => App.home() }, 'CARTE 🗺️'));
        return;
      }
      return this.offerThrow(p, shiny, slips, sprite, zone, attempt + 1);
    }

    Sfx.click();
    ball.classList.add('caught');
    burst('⭐');
    const isNew = res.new;
    Sfx.catch_();
    confetti(isNew ? 80 : 40);
    put(zone,
      el('div', { class: 'score' }, `Et hop ! ${p.name} est attrapé !`),
      isNew ? el('div', { class: 'score newrecord' }, shiny ? '✨ Nouveau chromatique !' : '🆕 Il rejoint ton Pokédex sauvage !') : null,
    );
    await say('Et hop ! Attrapé !');
    await Voice.play(p.name_audio, p.name);
    if (isNew) await say(shiny ? 'Ouah ! Un Pokémon chromatique !' : 'Il rejoint ton Pokédex sauvage !');
    await celebrateAll(res);
    put(zone,
      res.partner_event ? Partner.gainRow(res.partner_event) : null,
      el('div', { class: 'home-row' },
        el('button', { class: 'btn btn-huge btn-go', onclick: () => this.enter() }, 'ENCORE 🌿'),
        el('button', { class: 'btn btn-ghost', onclick: () => App.home() }, 'CARTE 🗺️'),
      ),
    );
  },
};

/* ---------------------------------------------------- wild pokédex tab --- */

/* The second Pokédex page: every Pokémon the tall grass can ever hold —
   caught in colour, readable-but-not-yet-caught as a shadow, the rest locked
   behind sounds still to learn. */
async function wildDex(grid) {
  const pool = Wild.pool || (await Wild.load());
  if (!pool) return;
  grid.innerHTML = '';
  pool.pokemon.forEach((p) => {
    const cls = p.caught ? '' : 'locked';
    grid.append(el('button', {
      class: `dexcell ${cls}`,
      onclick: () => {
        if (p.caught) return Voice.play(p.name_audio, p.name);
        say(p.available ? 'Il se cache dans les hautes herbes !' : 'Apprends de nouveaux sons pour le trouver !');
      },
    },
      el('img', { class: p.shiny ? 'shiny' : '', src: p.sprite, alt: '', loading: 'lazy' }),
      p.shiny ? el('span', { class: 'shinymark' }, '✨') : null,
      !p.available && !p.caught ? el('span', { class: 'lockmark' }, '🔒') : null,
      el('b', {}, p.caught ? p.name : '???'),
    ));
  });
}

/* -------------------------------------------------- missions & partner --- */

const Quests = {
  /* The day's three missions, read out one by one; tapping a row reads it
     again. Opened from the 📜 button on the map. */
  panel() {
    const qs = (App.state && App.state.quests) || [];
    const box = el('div', { class: 'quest-panel' },
      el('div', { class: 'quest-head' }, el('b', {}, '📜 Missions du jour'),
        el('button', { class: 'btn-icon', onclick: () => box.remove() }, '✕')),
      qs.map((q) => el('button', { class: `quest-row ${q.done ? 'done' : ''}`, onclick: () => Voice.play(q.audio, q.text) },
        el('span', { class: 'quest-icon' }, q.done ? '✅' : { superball: '🔵', berry: '🍓', balls: '🔴' }[q.reward] || '🎁'),
        el('span', { class: 'quest-text' }, q.text),
        el('span', { class: 'quest-bar' }, el('i', { style: `width:${Math.round((q.progress / q.goal) * 100)}%` })),
        el('small', {}, `${q.progress} / ${q.goal}`))),
    );
    $('#app').append(box);
    (async () => {
      await say('Tes missions du jour !');
      for (const q of qs) {
        if (!document.body.contains(box)) return;
        if (!q.done) await Voice.play(q.audio, q.text);
      }
    })();
  },

  // A finished mission: a banner, the reward, Jeanne says so.
  async celebrate(done) {
    for (const q of done || []) {
      const reward = { superball: '+1 Super Ball', berry: '+1 baie 🍓', balls: '+2 Poké Balls' }[q.reward] || '';
      const toast = el('div', { class: 'levelup quest-toast' },
        el('span', { class: 'quest-big' }, '📜'),
        el('div', {}, el('b', {}, 'Mission accomplie !'), el('small', {}, `${q.text} ${reward}`)));
      $('#fx').append(toast);
      Sfx.levelUp();
      confetti(50);
      await say('Mission accomplie !');
      await sleep(900);
      toast.classList.add('gone');
      setTimeout(() => toast.remove(), 500);
    }
  },
};

Object.assign(Partner, {
  /* Tap the partner: it jumps and says its name; once a day it is hungry
     and a berry from the bag makes it happy (and a little stronger). */
  async interact() {
    const p = this.get();
    if (!p) return;
    this.detail();
    if (!App.state.hungry) return;
    if (!((App.state.bag || {}).berry > 0)) {
      return say('Ton partenaire a faim ! Trouve une baie sur la carte ou dans les missions.');
    }
    let res;
    try { res = await api('api/feed', {}); App.state = res.state; } catch (_) { return; }
    for (let i = 0; i < 6; i++) setTimeout(() => burst('💖'), i * 160);
    Sfx.catch_();
    await say('Miam ! Ton partenaire adore les baies !');
    await celebrateAll(res);
    if ($('.ow-map')) App.home();
  },
});

/* ------------------------------------------- team, trainer, milestones --- */

/* Every celebration an answer can carry, in order: the partner's level or
   evolution, finished missions, a trainer level, a Pokédex milestone. */
async function celebrateAll(res) {
  if (!res) return;
  await Partner.celebrate(res.partner_event);
  await Quests.celebrate(res.quests_done);
  await Trainer.celebrate(res.trainer_event);
  if (res.milestone) {
    const toast = el('div', { class: 'levelup quest-toast' },
      el('span', { class: 'quest-big' }, '📘'),
      el('div', {}, el('b', {}, res.milestone.text), el('small', {}, '+1 Super Ball, +2 Poké Balls')));
    $('#fx').append(toast);
    Sfx.badge();
    confetti(90);
    await Voice.play(res.milestone.audio, res.milestone.text);
    await sleep(700);
    toast.classList.add('gone');
    setTimeout(() => toast.remove(), 500);
  }
}

const Trainer = {
  chip() {
    const t = App.state && App.state.trainer;
    if (!t) return null;
    return el('span', { class: 'chip trainer-chip', title: t.title },
      '🎖 ', el('b', {}, `N.${t.level}`),
      el('span', { class: 'trainer-bar' }, el('i', { style: `width:${Math.round((t.xp / t.need) * 100)}%` })));
  },

  async celebrate(ev) {
    if (!ev) return;
    const toast = el('div', { class: 'levelup trainer-toast' },
      el('span', { class: 'quest-big' }, '🎖'),
      el('div', {}, el('b', {}, `Dresseur niveau ${ev.level} !`),
        el('small', {}, `${ev.title} · +${ev.balls} Poké Balls${ev.bonus ? ', +1 Super Ball, +1 baie' : ''}`)));
    $('#fx').append(toast);
    Sfx.fanfare();
    confetti(70);
    await Voice.play(ev.line.audio, ev.line.text);
    if (ev.new_rank) await Voice.play(ev.new_rank.audio, ev.new_rank.text);
    await sleep(600);
    toast.classList.add('gone');
    setTimeout(() => toast.remove(), 500);
  },
};

const Team = {
  /* "Mon équipe": every Pokémon he owns, one per evolution line, with its
     level. Tap one and it becomes the partner — it walks the map, fights
     beside him and gains the levels from now on. */
  async open() {
    Voice.stop();
    let data;
    try { data = await api('api/team'); } catch (_) { return App.home(); }
    const grid = el('div', { class: 'team-grid' },
      data.members.map((m) => el('button', {
        class: `team-cell ${m.active ? 'active' : ''}`,
        onclick: async () => {
          Voice.play(m.name_audio, m.name);
          if (m.active) return;
          try { const r = await api('api/team', { species: m.species }); App.state = r.state; } catch (_) { return; }
          Sfx.catch_();
          burst('💫');
          await Voice.play(m.name_audio, m.name);
          await Voice.play(data.chosen.audio, data.chosen.text);
          this.open();
        },
      },
        el('img', { src: `sprites/art/${m.species}.png`, alt: '', loading: 'lazy' }),
        el('b', {}, m.name),
        el('span', { class: 'lvl' }, `N.${m.level}`),
        m.next_name ? el('small', {}, `✨ ${m.next_name} au N.${m.next_level}`) : null,
        m.active ? el('span', { class: 'team-active' }, 'Partenaire') : null,
      )),
    );
    App.screen(
      App.topbar(el('button', { class: 'btn-icon', onclick: () => App.home() }, '✕')),
      el('div', { class: 'page' },
        el('div', { class: 'act' },
          el('h2', {}, '👥 Mon équipe'),
          el('button', { class: 'hint hint-voice', onclick: () => Voice.play(data.prompt.audio, data.prompt.text) }, '🔊 ', data.prompt.text),
          grid,
        ),
      ),
    );
    Voice.lead(data.prompt.audio, data.prompt.text);
  },
};
