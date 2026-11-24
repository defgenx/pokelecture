'use strict';

/* Administration — the parent's control room.
   Everything reads and writes one player's save (var/players/<id>.json, the
   one picked in the player bar) through /api/admin/*: check the
   progression, spot what is fragile, rename the child, replay or skip an
   episode, fix the Pokédex, or wipe the save and start over. */

const $ = (s, r = document) => r.querySelector(s);
const el = (tag, attrs, ...kids) => {
  const n = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (k === 'class') n.className = v;
    else if (k.startsWith('on')) n.addEventListener(k.slice(2), v);
    else if (v !== null && v !== undefined && v !== false) n.setAttribute(k, v);
  }
  for (const kid of kids.flat()) if (kid != null && kid !== false) n.append(kid instanceof Node ? kid : String(kid));
  return n;
};

const TABS = [
  ['episodes', 'Épisodes'],
  ['items', 'Révisions'],
  ['pokedex', 'Pokédex'],
  ['exams', 'Examens'],
  ['fluency', 'Lecture'],
  ['danger', 'Réglages'],
];

// player: whose save the page shows and acts on (every admin call carries it).
const state = { sum: null, tab: 'episodes', players: [], player: null };

/* ------------------------------------------------------------------ api --- */

// withPlayer adds ?player= to an admin call: the page acts on one save.
function withPlayer(path) {
  if (!state.player) return path;
  return `${path}${path.includes('?') ? '&' : '?'}player=${encodeURIComponent(state.player)}`;
}

async function loadPlayers() {
  const res = await fetch('api/admin/players');
  if (!res.ok) throw new Error(await res.text());
  state.players = (await res.json()).players || [];
  if (!state.players.some((p) => p.id === state.player)) state.player = state.players.length ? state.players[0].id : null;
}

async function load() {
  await loadPlayers();
  const res = await fetch(withPlayer('api/admin/summary'));
  if (!res.ok) throw new Error(await res.text());
  state.sum = await res.json();
}

async function post(path, body) {
  const res = await fetch(path.startsWith('api/admin/players') ? path : withPlayer(path), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body || {}),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

// act runs an admin action then reloads, so the page always shows server truth.
async function act(path, body) {
  try {
    await post(path, body);
    await load();
    render();
  } catch (err) {
    warn(`Action impossible : ${err.message}`);
  }
}

/* ---------------------------------------------------------------- render --- */

function warn(msg) {
  const n = $('#warn');
  n.hidden = false;
  n.textContent = msg;
}

function fmtDate(iso) {
  if (!iso || iso.startsWith('0001')) return 'jamais';
  const d = new Date(iso);
  return d.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' }) +
    ' ' + d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' });
}

function tally() {
  const s = state.sum;
  const done = s.episodes.filter((e) => e.completions > 0).length;
  const caught = s.pokedex.filter((p) => p.caught).length;
  $('#tally').textContent =
    `${s.name} · ⭐ ${s.stars} · 🔥 série de ${s.streak} jour${s.streak > 1 ? 's' : ''} · ` +
    `${done}/${s.episodes.length} épisodes · ${caught}/${s.pokedex.length} Pokémon`;
}

function tabs() {
  const nav = $('#tabs');
  nav.innerHTML = '';
  TABS.forEach(([id, label]) => {
    nav.append(el('button', {
      class: state.tab === id ? 'on' : '',
      onclick: () => { state.tab = id; render(); },
    }, label));
  });
}

function statCards() {
  const s = state.sum;
  return el('div', { class: 'statrow' },
    el('div', { class: 'stat' }, el('b', {}, s.name), el('span', {}, 'dresseur')),
    el('div', { class: 'stat' }, el('b', {}, `⭐ ${s.stars}`), el('span', {}, 'étoiles')),
    el('div', { class: 'stat' }, el('b', {}, `🔥 ${s.streak}`), el('span', {}, `série (dernier jour : ${s.last_day || '—'})`)),
    el('div', { class: 'stat' }, el('b', {}, fmtDate(s.updated_at)), el('span', {}, 'dernière sauvegarde')),
  );
}

function episodesTab(list) {
  list.append(statCards());
  const overrides = (state.sum.settings && state.sum.settings.episode_stars) || {};
  list.append(el('p', { style: 'color:#6a7180;font-size:13px' },
    '« ★ pour la suite » : les étoiles que CET épisode doit avoir pour débloquer le suivant — ' +
    'par défaut la règle globale (onglet Réglages), ajustable épisode par épisode.'));
  state.sum.episodes.forEach((e) => {
    const stars = e.completions > 0 ? '★'.repeat(e.best_stars) + '☆'.repeat(3 - e.best_stars) : '';

    // The per-episode unlock threshold, overriding the global policy.
    const sel = el('select', { class: 'small', title: '★ que cet épisode doit avoir pour débloquer le suivant' },
      [['', `global (${'★'.repeat(state.sum.settings.min_stars) || 'terminer'})`],
       ['0', 'terminer suffit'], ['1', '★ requis'], ['2', '★★ requis'], ['3', '★★★ requis']]
        .map(([v, label]) => {
          const o = el('option', { value: v }, label);
          if (String(overrides[e.id] ?? '') === v) o.selected = true;
          return o;
        }),
    );
    sel.addEventListener('change', () =>
      act(`api/admin/episodes/${e.id}/stars`, { min_stars: sel.value === '' ? -1 : Number(sel.value) }));

    list.append(el('div', { class: 'row' },
      el('div', { class: 'txt' },
        el('b', {}, `${e.legendary ? '★ ' : ''}${e.title}${e.badge ? ` · 🎖 ${e.badge}` : ''}`),
        el('small', {}, e.unlocked
          ? `${e.completions} partie${e.completions > 1 ? 's' : ''} · dernière : ${fmtDate(e.last_played)}`
          : 'verrouillé — termine les épisodes précédents'),
      ),
      el('span', { class: 'stars' }, stars),
      sel,
      el('button', {
        class: 'small',
        disabled: e.completions === 0,
        title: 'Efface les stats de cet épisode ; ses étoiles quittent le total et se regagnent en le rejouant',
        onclick: () => act(`api/admin/episodes/${e.id}/reset`),
      }, '↺ Rejouer comme neuf'),
      el('button', {
        class: 'small',
        disabled: e.completions > 0,
        title: 'Débloque la suite et remplit le Pokédex comme une vraie victoire',
        onclick: () => act(`api/admin/episodes/${e.id}/complete`),
      }, '✓ Marquer terminé'),
      el('span', { class: 'meta' }, e.id),
    ));
  });
}

function itemsTab(list) {
  const items = state.sum.items || [];
  if (!items.length) {
    list.append(el('p', { style: 'color:#6a7180' }, 'Aucune révision enregistrée pour le moment.'));
    return;
  }
  list.append(el('p', { style: 'color:#6a7180;font-size:13px' },
    'Classés du plus fragile au plus solide (boîte 0 = vu mais raté, boîte 4 = acquis). ' +
    'Les éléments « à revoir » reviendront en échauffement de la prochaine séance.'));
  items.forEach((it) => {
    const pct = it.seen ? Math.round((it.correct / it.seen) * 100) : 0;
    list.append(el('div', { class: 'row' },
      el('div', { class: 'txt' },
        el('b', {}, it.id.replace(/^(son|syl|mot|phrase):/, (m, g) => ({ son: '🔊 ', syl: '🔤 ', mot: '📖 ', phrase: '📝 ' })[g] || '')),
        el('small', {}, `${it.correct}/${it.seen} réussi${it.correct > 1 ? 's' : ''} · vu le ${fmtDate(it.last_seen)}`),
      ),
      el('span', { class: `itembar ${it.box < 2 ? 'weak' : ''}` },
        el('i', { style: `width:${(it.box / 4) * 100}%` })),
      el('span', { class: 'meta' }, `boîte ${it.box}`),
      el('span', { class: 'meta' }, it.due ? 'à revoir' : ''),
      el('span', { class: 'meta' }, `${pct} %`),
    ));
  });
}

/* The surprise exams (data/events.json): each one's condition, today's
   draw, and the last attempt section by section — the report card that says
   which sounds hold. "Aujourd'hui" puts it on the map now. */
const SECTION_NAMES = { son: 'Sons', syl: 'Syllabes', mot: 'Mots', phrase: 'Phrases' };

function examsTab(list) {
  const exams = state.sum.exams || [];
  list.append(el('p', { style: 'color:#6a7180;font-size:13px' },
    'Un examen surprise apparaît sur la carte (🎓) certains jours, une fois son épisode terminé. ' +
    'Une seule réponse par question ; réussi, il donne son Pokémon et ne revient plus. ' +
    'Les réponses fausses repartent en révision.'));
  if (!exams.length) list.append(el('p', {}, 'Aucun examen dans data/events.json.'));
  exams.forEach((x) => {
    const r = x.record || {};
    const sections = Object.keys(SECTION_NAMES).filter((k) => (r.sections || {})[k])
      .map((k) => `${SECTION_NAMES[k]} ${r.sections[k][0]}/${r.sections[k][1]}`).join(' · ');
    const status = r.passed ? `✅ réussi (meilleur ${r.best} %)`
      : r.attempts ? `❌ ${r.attempts} essai${r.attempts > 1 ? 's' : ''}, dernier ${r.last} % (il faut ${x.pass} %)`
      : x.eligible ? 'pas encore tenté' : `🔒 après l'épisode ${x.after}`;
    const call = r.passed ? '' : el('button', {
      class: 'btn',
      onclick: () => act(`api/admin/events/${x.today ? 'none' : x.id}/today`),
    }, x.today ? 'Retirer de la carte' : 'Faire apparaître aujourd\'hui');
    list.append(el('div', { class: 'row' },
      el('img', { src: `sprites/art/${x.pokemon}.png`, alt: '', style: 'width:40px;height:40px;object-fit:contain' }),
      el('div', { class: 'txt' },
        el('b', {}, `${x.today ? '🎓 ' : ''}${x.title}`),
        el('small', { style: 'display:block' }, status),
        sections ? el('small', { style: 'display:block' }, `Dernier essai, le ${fmtDate(r.last_at)} : ${sections}`) : null,
        x.missed.length ? el('small', { style: 'display:block' }, `À revoir : ${x.missed.map((m) => m.label).join(', ')}`) : null,
      ),
      call,
    ));
  });
}

/* « Lecture » — the one-minute fluency check: the child reads the words
   aloud, the parent taps ✓ or ✗ for each. Words read correctly per minute,
   and every answer goes to the review boxes. */
const FLUENCY_SECONDS = 60;

function fluencyTab(list) {
  const runs = state.sum.fluency || [];
  const speed = state.sum.speed || {};
  list.append(el('p', { style: 'color:#6a7180;font-size:13px' },
    'Assieds-toi à côté de lui avec la tablette. Il lit chaque mot à voix haute ; ' +
    'touche ✓ s\'il est bien lu, ✗ sinon. Au bout d\'une minute : les mots bien lus par minute. ' +
    'Une fois par semaine suffit pour voir les progrès.'));
  const stage = el('div', { class: 'section fluency' });
  const start = el('button', { class: 'btn', onclick: () => runFluency(stage) }, '▶ Commencer (1 minute)');
  stage.append(start);
  list.append(stage);
  if (runs.length) {
    list.append(el('h2', {}, 'Historique'));
    runs.slice().reverse().forEach((r) => list.append(el('div', { class: 'row' },
      el('div', { class: 'txt' }, el('b', {}, `${r.wcpm} mots/min`),
        el('small', {}, `${fmtDate(r.at)} · ${r.right} bien lus, ${r.wrong} ratés en ${r.seconds} s`)),
    )));
  }
  const kinds = { syl: 'syllabe entendue', son: 'son entendu', mot: 'mot lu' };
  const known = Object.entries(speed).filter(([k, v]) => kinds[k] && v.n > 0);
  if (known.length) {
    list.append(el('h2', {}, 'Son temps de réponse habituel (jeu)'));
    known.forEach(([k, v]) => list.append(el('div', { class: 'row' },
      el('div', { class: 'txt' }, el('b', {}, `${(v.ms / 1000).toFixed(1)} s par ${kinds[k]}`),
        el('small', {}, `sur ${v.n} réponses — la 3e étoile se gagne un peu plus vite que ça`)))));
  }
}

async function runFluency(stage) {
  let words = [];
  try {
    const res = await fetch(withPlayer('api/admin/fluency/words'));
    if (!res.ok) throw new Error(await res.text());
    words = (await res.json()).words || [];
  } catch (err) { warn(`Impossible : ${err.message}`); return; }
  if (!words.length) return;
  const right = [];
  const wrong = [];
  let i = 0;
  const t0 = Date.now();
  const word = el('div', { class: 'fluency-word' }, words[0]);
  const clock = el('div', { class: 'fluency-clock' }, `${FLUENCY_SECONDS} s`);
  const next = (ok) => {
    (ok ? right : wrong).push(words[i]);
    i++;
    if (i >= words.length) return finish();
    word.textContent = words[i];
  };
  const finish = async () => {
    clearInterval(timer);
    const seconds = Math.max(10, Math.round((Date.now() - t0) / 1000));
    stage.innerHTML = '';
    try {
      await post('api/admin/fluency', { seconds, right, wrong });
      await load();
      render();
    } catch (err) { warn(`Enregistrement impossible : ${err.message}`); }
  };
  const timer = setInterval(() => {
    const left = FLUENCY_SECONDS - Math.floor((Date.now() - t0) / 1000);
    clock.textContent = `${Math.max(0, left)} s`;
    if (left <= 0) finish();
  }, 250);
  stage.innerHTML = '';
  stage.append(clock, word, el('div', { class: 'namebox' },
    el('button', { class: 'btn ok', onclick: () => next(true) }, '✓ Bien lu'),
    el('button', { class: 'btn ko', onclick: () => next(false) }, '✗ Raté'),
    el('button', { class: 'linkbtn', onclick: finish }, 'Arrêter'),
  ));
}

function pokedexTab(list) {
  state.sum.pokedex.forEach((p) => {
    list.append(el('div', { class: 'row' },
      el('div', { class: `dexline ${p.caught ? '' : 'off'}` },
        el('img', { src: `sprites/${p.id}.png`, alt: '', loading: 'lazy' }),
        el('b', {}, `#${p.id} ${p.name}`),
      ),
      el('span', { class: 'meta' }, p.caught ? (p.shiny ? 'attrapé ✨' : 'attrapé') : '—'),
      el('button', {
        class: `small ${p.caught ? 'danger' : ''}`,
        onclick: () => act(`api/admin/pokedex/${p.id}`, { caught: !p.caught }),
      }, p.caught ? 'Retirer' : 'Attraper'),
    ));
  });
}

function dangerTab(list) {
  const s = state.sum;

  const input = el('input', { value: s.name, placeholder: 'Prénom' });
  list.append(el('div', { class: 'section' },
    el('h2', {}, 'Prénom'),
    el('div', { class: 'namebox' },
      input,
      el('button', {
        class: 'linkbtn',
        onclick: () => {
          const name = input.value.trim();
          if (name) act('api/admin/name', { name });
        },
      }, 'Renommer'),
    ),
    el('p', { style: 'color:#6a7180;font-size:13px;margin-top:8px' },
      'Change le prénom sans toucher à la progression, au Pokédex ni aux étoiles.'),
  ));

  lockSection(list, s);

  // The reading font: what school uses is what he should see.
  const font = el('select', {},
    [['', 'Police du jeu'], ['andika', 'Andika — lettres d\'école (a et g simples)'], ['cursive', 'Cursive (écriture attachée de l\'école)']]
      .map(([v, label]) => {
        const o = el('option', { value: v }, label);
        if (v === ((s.settings && s.settings.font) || '')) o.selected = true;
        return o;
      }),
  );
  font.addEventListener('change', () => act('api/admin/font', { font: font.value }));
  list.append(el('div', { class: 'section' },
    el('h2', {}, 'Police de lecture'),
    el('div', { class: 'namebox' }, el('label', {}, 'Les mots à lire s\'écrivent en : '), font),
    el('p', { style: 'color:#6a7180;font-size:13px;margin-top:8px' },
      'Seuls les mots, syllabes et phrases à lire changent ; choisis ce que l\'école utilise.'),
  ));

  // Progression policy: what "done enough to move on" means is the parent's call.
  const set = s.settings || { min_stars: 0, free_play: false, battle_lives: 3 };
  const minStars = el('select', {},
    [['0', 'Terminer l\'épisode suffit'], ['1', 'Au moins ★'], ['2', 'Au moins ★★'], ['3', '★★★ obligatoires']]
      .map(([v, label]) => {
        const o = el('option', { value: v }, label);
        if (Number(v) === set.min_stars) o.selected = true;
        return o;
      }),
  );
  const freePlay = el('input', { type: 'checkbox' });
  freePlay.checked = !!set.free_play;
  const lives = el('select', {},
    [['-1', 'Illimitées (jamais de défaite)'], ['2', '2 vies'], ['3', '3 vies'], ['4', '4 vies'], ['5', '5 vies']]
      .map(([v, label]) => {
        const o = el('option', { value: v }, label);
        if (Number(v) === (set.battle_lives || 3)) o.selected = true;
        return o;
      }),
  );
  const apply = () => act('api/admin/settings', {
    min_stars: Number(minStars.value),
    free_play: freePlay.checked,
    battle_lives: Number(lives.value),
  });
  minStars.addEventListener('change', apply);
  freePlay.addEventListener('change', apply);
  lives.addEventListener('change', apply);
  list.append(el('div', { class: 'section' },
    el('h2', {}, 'Déblocage des épisodes'),
    el('div', { class: 'namebox' },
      el('label', {}, 'Pour débloquer la suite : '), minStars,
    ),
    el('div', { class: 'namebox', style: 'margin-top:8px' },
      el('label', { style: 'display:flex;align-items:center;gap:8px' },
        freePlay, 'Tout débloquer (mode libre — l\'ordre reste conseillé, plus imposé)'),
    ),
    el('p', { style: 'color:#6a7180;font-size:13px;margin-top:8px' },
      'Exiger des étoiles fait des rediffusions une partie du parcours ; le mode libre ' +
      'laisse jouer n\'importe quel épisode. Appliqué immédiatement. Chaque épisode peut ' +
      'avoir son propre seuil dans l\'onglet Épisodes (« ★ pour la suite »).'),
  ));

  list.append(el('div', { class: 'section' },
    el('h2', {}, 'Vies en combat'),
    el('div', { class: 'namebox' },
      el('label', {}, 'Erreurs permises avant la défaite : '), lives,
    ),
    el('p', { style: 'color:#6a7180;font-size:13px;margin-top:8px' },
      'Chaque syllabe fausse en combat coûte un cœur. À zéro : défaite, et ' +
      'l\'épisode recommence du début (l\'étage, pour la Tour). « Illimitées » ' +
      'revient à l\'ancien mode sans enjeu.'),
  ));

  const recs = s.records || {};
  const recNames = {
    memory: '🃏 Memory (temps)', chasse: '🌿 La chasse', lecture: '📖 Lis et attrape',
    oreille: '👂 L\'oreille fine', atelier: '⚒️ L\'atelier des mots',
    mystere: '❓ Pokémon mystère', paires: '🔤 Paires mot-image (temps)',
  };
  const recRows = Object.entries(recs).map(([g, v]) =>
    el('div', { class: 'namebox', style: 'margin-top:6px' },
      el('label', { style: 'min-width:220px' }, `${recNames[g] || g} : ${v}`),
      el('button', { class: 'linkbtn', onclick: () => act('api/admin/records/reset', { game: g }) }, '↺ Effacer'),
    ));
  list.append(el('div', { class: 'section' },
    el('h2', {}, 'Tour de Combat et mini-jeux'),
    el('p', { style: 'color:#6a7180;font-size:13px' },
      `🗼 Étage atteint : ${s.tower_floor || 0} · prochain défi : étage ${(s.tower_floor || 0) + 1}.`),
    recRows.length ? el('div', {}, recRows,
      el('div', { class: 'namebox', style: 'margin-top:10px' },
        el('button', { class: 'linkbtn', onclick: () => act('api/admin/records/reset', { game: '' }) },
          '🗑 Effacer tous les records')),
    ) : el('p', { style: 'color:#6a7180;font-size:13px' }, 'Aucun record de mini-jeu pour le moment.'),
  ));

  list.append(el('div', { class: 'section' },
    el('h2', {}, 'Zone dangereuse'),
    el('div', { class: 'dangerzone' },
      el('p', {}, 'Efface toute la progression : épisodes, étoiles, Pokédex, révisions. Seul le prénom est conservé. Irréversible.'),
      el('button', {
        onclick: () => {
          if (window.confirm('Tout effacer et repartir de zéro ?')) act('api/admin/reset');
        },
      }, '🗑 Réinitialiser la sauvegarde'),
    ),
    state.players.length > 1 ? el('div', { class: 'dangerzone' },
      el('p', {}, `Supprime le joueur ${state.sum.name} et toute sa progression. Sa sauvegarde part dans var/players/trash/ : on peut la récupérer.`),
      el('button', {
        onclick: async () => {
          if (!window.confirm(`Supprimer le joueur ${state.sum.name} ?`)) return;
          try {
            await post(`api/admin/players/${encodeURIComponent(state.player)}/delete`);
            state.player = null;
            await load();
            render();
          } catch (err) { warn(`Suppression impossible : ${err.message}`); }
        },
      }, `🗑 Supprimer le joueur ${state.sum.name}`),
    ) : null,
  ));
}

// The nine Pokémon a code is made of; the same list as lockPalette in
// internal/api/players.go and Players.PALETTE in app.js.
const LOCK_PALETTE = [[25, 'Pikachu'], [4, 'Salamèche'], [7, 'Carapuce'], [1, 'Bulbizarre'],
  [133, 'Évoli'], [39, 'Rondoudou'], [52, 'Miaouss'], [143, 'Ronflex'], [54, 'Psykokwak']];

/* 🔒 Privé: the game asks this player's three-Pokémon code once per device.
   🌐 Public: anyone on the tablet can play it. The code itself is stored
   hashed, so it cannot be shown again — write it down before saving. */
function lockSection(list, s) {
  const code = [];
  const slots = el('div', { class: 'lock-slots' });
  const save = el('button', { class: 'linkbtn', disabled: true, onclick: () => {
    const names = code.map((n) => LOCK_PALETTE.find(([id]) => id === n)[1]).join(' → ');
    if (window.confirm(`Code de ${s.name} : ${names}\nNotez-le : il ne pourra pas être réaffiché.`)) {
      act('api/admin/lock', { code: code.slice() });
    }
  } }, s.locked ? 'Changer le code' : 'Rendre privé avec ce code');
  const show = () => {
    slots.innerHTML = '';
    for (let i = 0; i < 4; i++) {
      slots.append(el('span', { class: 'lock-slot' },
        code[i] ? el('img', { src: `sprites/art/${code[i]}.png`, alt: '' }) : `${i + 1}`));
    }
    save.disabled = code.length !== 4;
  };
  const pad = el('div', { class: 'lock-pad' }, LOCK_PALETTE.map(([n, name]) => el('button', {
    class: 'lock-key', title: name,
    onclick: () => { if (code.length < 4) { code.push(n); show(); } },
  }, el('img', { src: `sprites/art/${n}.png`, alt: name }))));
  show();
  list.append(el('div', { class: 'section' },
    el('h2', {}, s.locked ? '🔒 Privé' : '🌐 Public'),
    el('p', { style: 'color:#6a7180;font-size:13px;margin-bottom:10px' }, s.locked
      ? `${s.name} est privé : le jeu demande son code (quatre Pokémon, dans l'ordre) une fois par appareil. Changer le code le redemande sur tous les appareils.`
      : `${s.name} est public : n'importe qui peut jouer sa partie. Choisissez quatre Pokémon, dans l'ordre, pour le rendre privé.`),
    s.lock_closed ? el('div', { class: 'dangerzone' },
      el('p', {}, `Fermé : ${s.lock_fails} codes faux d'affilée. Le jeu ne demande plus le code tant que vous ne rouvrez pas (les appareils déjà entrés continuent de jouer).`),
      el('button', { onclick: () => act('api/admin/lock', { reopen: true }) }, '🔓 Rouvrir (même code)'),
    ) : s.lock_fails ? el('p', { style: 'color:#b04a1d;font-size:13px;margin-bottom:10px' },
      `${s.lock_fails} code(s) faux depuis la dernière ouverture.`) : null,
    pad,
    el('div', { class: 'namebox' }, slots,
      el('button', { class: 'linkbtn', onclick: () => { code.length = 0; show(); } }, 'Effacer'),
      save,
      s.locked ? el('button', { class: 'linkbtn', onclick: () => {
        if (window.confirm(`Rendre ${s.name} public ? Plus aucun code ne sera demandé.`)) act('api/admin/lock', { code: [] });
      } }, '🌐 Rendre public') : null),
  ));
}

/* The players: one save each. Pick whose save the page shows, add one,
   remove one (its file goes to var/players/trash/, recoverable). */
function playerBar() {
  const bar = $('#players');
  bar.innerHTML = '';
  bar.append(el('b', {}, '👥 Joueur :'));
  state.players.forEach((p) => {
    bar.append(el('button', {
      class: `player-chip ${p.id === state.player ? 'on' : ''}`,
      onclick: async () => { state.player = p.id; await load(); render(); },
    }, p.avatar ? el('img', { src: `sprites/art/${p.avatar}.png`, alt: '' }) : el('span', { class: 'chip-unknown' }, '?'), `${p.locked ? '🔒 ' : ''}${p.name} · ⭐ ${p.stars || 0}`));
  });
  bar.append(el('button', {
    class: 'player-chip add',
    onclick: async () => {
      const name = window.prompt('Prénom du nouveau joueur ?');
      if (!name || !name.trim()) return;
      try {
        const res = await post('api/admin/players', { name: name.trim() });
        state.player = res.id;
        await load();
        render();
      } catch (err) { warn(`Création impossible : ${err.message}`); }
    },
  }, '➕ Nouveau joueur'));
}

function render() {
  playerBar();
  tabs();
  tally();
  const list = $('#list');
  list.innerHTML = '';
  ({ episodes: episodesTab, items: itemsTab, pokedex: pokedexTab, exams: examsTab, fluency: fluencyTab, danger: dangerTab })[state.tab](list);
}

/* ------------------------------------------------------------------ boot --- */

(async () => {
  try {
    await load();
    render();
  } catch (err) {
    warn(`Chargement impossible : ${err.message}`);
  }
})();
