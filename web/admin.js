'use strict';

/* Administration — the parent's control room.
   Everything reads and writes var/progress.json through /api/admin/*: check the
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
  ['danger', 'Réglages'],
];

const state = { sum: null, tab: 'episodes' };

/* ------------------------------------------------------------------ api --- */

async function load() {
  const res = await fetch('api/admin/summary');
  if (!res.ok) throw new Error(await res.text());
  state.sum = await res.json();
}

async function post(path, body) {
  const res = await fetch(path, {
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
  state.sum.episodes.forEach((e) => {
    const stars = e.completions > 0 ? '★'.repeat(e.best_stars) + '☆'.repeat(3 - e.best_stars) : '';
    list.append(el('div', { class: 'row' },
      el('div', { class: 'txt' },
        el('b', {}, `${e.legendary ? '★ ' : ''}${e.title}${e.badge ? ` · 🎖 ${e.badge}` : ''}`),
        el('small', {}, e.unlocked
          ? `${e.completions} partie${e.completions > 1 ? 's' : ''} · dernière : ${fmtDate(e.last_played)}`
          : 'verrouillé — termine les épisodes précédents'),
      ),
      el('span', { class: 'stars' }, stars),
      el('button', {
        class: 'small',
        disabled: e.completions === 0,
        title: 'Efface les stats de cet épisode (les étoiles restent acquises)',
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
        el('b', {}, it.id.replace(/^(syl|mot|phrase):/, (m, g) => ({ syl: '🔤 ', mot: '📖 ', phrase: '📝 ' })[g] || '')),
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

  // Progression policy: what "done enough to move on" means is the parent's call.
  const set = s.settings || { min_stars: 0, free_play: false };
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
  const apply = () => act('api/admin/settings', {
    min_stars: Number(minStars.value),
    free_play: freePlay.checked,
  });
  minStars.addEventListener('change', apply);
  freePlay.addEventListener('change', apply);
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
      'laisse jouer n\'importe quel épisode. Appliqué immédiatement.'),
  ));

  const recs = s.records || {};
  const recNames = { memory: '🃏 Memory (temps)', chasse: '🌿 La chasse', lecture: '📖 Lis et attrape', oreille: '👂 L\'oreille fine' };
  list.append(el('div', { class: 'section' },
    el('h2', {}, 'Tour de Combat et mini-jeux'),
    el('p', { style: 'color:#6a7180;font-size:13px' },
      `🗼 Étage atteint : ${s.tower_floor || 0} · prochain défi : étage ${(s.tower_floor || 0) + 1}. ` +
      (Object.keys(recs).length
        ? 'Records : ' + Object.entries(recs).map(([g, v]) => `${recNames[g] || g} : ${v}`).join(' · ')
        : 'Aucun record de mini-jeu pour le moment.')),
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
  ));
}

function render() {
  tabs();
  tally();
  const list = $('#list');
  list.innerHTML = '';
  ({ episodes: episodesTab, items: itemsTab, pokedex: pokedexTab, danger: dangerTab })[state.tab](list);
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
