'use strict';

/* Espace parent — the Tableau de bord and Statistiques pages, and the small
   kit they are drawn with (README « Statistiques »): stat tiles, cards, and
   hand-drawn SVG charts. One series per chart, in one colour (--series-1);
   every bar, point and cell has a hover/tap tooltip, and every chart a
   « Voir les chiffres » table. Data: state.sum.days (60 days, oldest first),
   state.sum.slowest, items, episodes, fluency, exams. */

const KIND_NAMES = { son: 'Sons', syl: 'Syllabes', mot: 'Mots', phrase: 'Phrases' };

const fmtS = (ms) => (ms ? `${(ms / 1000).toFixed(1).replace('.', ',')} s` : '—');
const fmtMin = (m) => (m >= 60 ? `${Math.floor(m / 60)} h ${String(m % 60).padStart(2, '0')}` : `${m} min`);
const fmtPct = (n, d) => (d ? `${Math.round((n / d) * 100)} %` : '—');
const dayLabel = (iso, long) => new Date(`${iso}T12:00`).toLocaleDateString('fr-FR',
  long ? { weekday: 'long', day: 'numeric', month: 'long' } : { day: 'numeric', month: 'short' });

// totals adds days up: minutes, sessions, answers, right, and the average
// answer time weighed by each day's timed answers.
function totals(days) {
  const t = { play: 0, sessions: 0, answers: 0, right: 0, timed: 0, ms: 0, active: 0 };
  days.forEach((d) => {
    t.play += d.play_min; t.sessions += d.sessions; t.answers += d.answers; t.right += d.right;
    t.timed += d.timed || 0; t.ms += (d.avg_ms || 0) * (d.timed || 0);
    if (d.sessions || d.answers) t.active++;
  });
  t.avg = t.timed ? Math.round(t.ms / t.timed) : 0;
  return t;
}

const lastDays = (n) => (state.sum.days || []).slice(-n);

/* ------------------------------------------------------------ the kit --- */

// kpi: a stat tile — the number, what it is, and (optionally) how it moved.
// trend: { text, good } — good decides ▲/▼ colour; the arrow and words carry
// the meaning too, never the colour alone.
function kpi(label, value, sub, trend) {
  return el('div', { class: 'kpi' },
    el('span', { class: 'kpi-label' }, label),
    el('b', { class: 'kpi-value' }, value),
    sub ? el('span', { class: 'kpi-sub' }, sub) : null,
    trend ? el('span', { class: `trend ${trend.good === null ? '' : trend.good ? 'up' : 'down'}` }, trend.text) : null);
}

// change compares two values: "▲ 12 %" style text, and whether that is good.
function change(now, before, { lowerIsBetter = false, points = false } = {}) {
  if (!before || !now) return null;
  const delta = points ? Math.round(now - before) : Math.round(((now - before) / before) * 100);
  if (delta === 0) return { text: '= comme la semaine d\'avant', good: null };
  const up = delta > 0;
  return {
    text: `${up ? '▲' : '▼'} ${Math.abs(delta)}${points ? ' pts' : ' %'} vs la semaine d'avant`,
    good: lowerIsBetter ? !up : up,
  };
}

function segmented(options, value, onChange) {
  return el('div', { class: 'segmented', role: 'group' },
    options.map(([v, label]) => el('button', { class: v === value ? 'on' : '', onclick: () => onChange(v) }, label)));
}

// card: a titled panel. table: rows for « Voir les chiffres » ([head], ...rows).
function card(title, sub, body, table, wide) {
  const box = el('section', { class: `card ${wide ? 'wide' : ''}` },
    el('div', { class: 'card-head' },
      el('div', {}, el('h3', {}, title), sub ? el('small', {}, sub) : null)));
  const view = el('div', { class: 'card-body' }, body);
  box.append(view);
  if (table && table.length > 1) {
    const tbl = el('table', { class: 'viz-table', hidden: true },
      el('tr', {}, table[0].map((h) => el('th', {}, h))),
      table.slice(1).map((r) => el('tr', {}, r.map((c) => el('td', {}, String(c))))));
    const toggle = el('button', { class: 'linkbtn small' }, 'Voir les chiffres');
    toggle.addEventListener('click', () => {
      tbl.hidden = !tbl.hidden;
      view.hidden = !tbl.hidden;
      toggle.textContent = tbl.hidden ? 'Voir les chiffres' : 'Voir le graphique';
    });
    box.querySelector('.card-head').append(toggle);
    box.append(tbl);
  }
  return box;
}

const empty = (text) => el('p', { class: 'empty' }, text || 'Pas encore de données : elles arrivent dès qu\'il joue.');

/* ------------------------------------------------------------ tooltip --- */

const tip = {
  show(html, ev) {
    const t = $('#viz-tip');
    t.innerHTML = html;
    t.hidden = false;
    const x = Math.min(window.innerWidth - t.offsetWidth - 8, ev.clientX + 12);
    const y = Math.max(8, ev.clientY - t.offsetHeight - 12);
    t.style.left = `${x}px`;
    t.style.top = `${y}px`;
  },
  hide() { $('#viz-tip').hidden = true; },
};

// hover wires a hit target: pointer hover on a mouse, a tap on a tablet.
function hover(node, html, onIn, onOut) {
  node.addEventListener('pointerenter', (ev) => { tip.show(html, ev); if (onIn) onIn(); });
  node.addEventListener('pointermove', (ev) => tip.show(html, ev));
  node.addEventListener('pointerleave', () => { tip.hide(); if (onOut) onOut(); });
  node.addEventListener('click', (ev) => { tip.show(html, ev); if (onIn) onIn(); });
}

/* ------------------------------------------------------------- charts --- */

const SVG = 'http://www.w3.org/2000/svg';
function svg(tag, attrs, ...kids) {
  const n = document.createElementNS(SVG, tag);
  Object.entries(attrs || {}).forEach(([k, v]) => n.setAttribute(k, v));
  kids.flat().forEach((k) => k && n.append(k));
  return n;
}

// niceMax rounds an axis top up to 1, 2 or 5 × a power of ten.
function niceMax(v) {
  if (v <= 0) return 1;
  const p = 10 ** Math.floor(Math.log10(v));
  return [1, 2, 5, 10].map((m) => m * p).find((m) => m >= v);
}

const W = 640;
const H = 190;
const PAD = { l: 40, r: 10, t: 12, b: 24 };

function frame(n, max, fmtY) {
  const g = svg('g', {});
  for (let i = 0; i <= 2; i++) {
    const v = (max / 2) * i;
    const y = PAD.t + (H - PAD.t - PAD.b) * (1 - v / max);
    g.append(svg('line', { x1: PAD.l, x2: W - PAD.r, y1: y, y2: y, class: i ? 'grid' : 'axis' }));
    const label = svg('text', { x: PAD.l - 6, y: y + 4, class: 'tick', 'text-anchor': 'end' });
    label.textContent = fmtY(v);
    g.append(label);
  }
  return g;
}

function xLabels(rows, band) {
  const g = svg('g', {});
  const every = rows.length > 31 ? 10 : rows.length > 14 ? 7 : rows.length > 7 ? 2 : 1;
  rows.forEach((r, i) => {
    if ((rows.length - 1 - i) % every) return;
    const t = svg('text', { x: PAD.l + band * (i + 0.5), y: H - 6, class: 'tick', 'text-anchor': 'middle' });
    t.textContent = r.label;
    g.append(t);
  });
  return g;
}

// barChart: one bar per row (r.v), rounded 4px at the data end, a 2px gap.
function barChart(rows, { fmtY = (v) => String(v), tipOf }) {
  const max = niceMax(Math.max(...rows.map((r) => r.v)));
  const band = (W - PAD.l - PAD.r) / rows.length;
  const bw = Math.max(2, Math.min(28, band - 2));
  const base = H - PAD.b;
  const root = svg('svg', { viewBox: `0 0 ${W} ${H}`, class: 'viz', role: 'img' });
  root.append(frame(rows.length, max, fmtY));
  rows.forEach((r, i) => {
    const x = PAD.l + band * i + (band - bw) / 2;
    const h = (r.v / max) * (base - PAD.t);
    if (h > 0) {
      const rad = Math.min(4, bw / 2, h);
      const y = base - h;
      root.append(svg('path', {
        class: 'bar',
        d: `M${x},${base} V${y + rad} Q${x},${y} ${x + rad},${y} H${x + bw - rad} Q${x + bw},${y} ${x + bw},${y + rad} V${base} Z`,
      }));
    }
    const hit = svg('rect', { x: PAD.l + band * i, y: PAD.t, width: band, height: base - PAD.t, class: 'hit' });
    hover(hit, tipOf(r));
    root.append(hit);
  });
  root.append(xLabels(rows, band));
  return el('div', { class: 'viz-wrap' }, root);
}

// lineChart: a 2px line through the days that have a value (a day without
// data breaks it), a crosshair and an 8px marker under the pointer.
function lineChart(rows, { fmtY = (v) => String(v), tipOf, max: fixedMax }) {
  const vals = rows.filter((r) => r.v !== null);
  if (!vals.length) return empty();
  const max = fixedMax || niceMax(Math.max(...vals.map((r) => r.v)));
  const band = (W - PAD.l - PAD.r) / rows.length;
  const base = H - PAD.b;
  const X = (i) => PAD.l + band * (i + 0.5);
  const Y = (v) => base - (v / max) * (base - PAD.t);
  const root = svg('svg', { viewBox: `0 0 ${W} ${H}`, class: 'viz', role: 'img' });
  root.append(frame(rows.length, max, fmtY));
  let d = '';
  let open = false;
  rows.forEach((r, i) => {
    if (r.v === null) { open = false; return; }
    d += `${open ? 'L' : 'M'}${X(i)},${Y(r.v)} `;
    open = true;
  });
  root.append(svg('path', { d, class: 'line' }));
  // A lone day between gaps has no segment: a dot shows it.
  rows.forEach((r, i) => {
    const lone = r.v !== null && (i === 0 || rows[i - 1].v === null) && (i === rows.length - 1 || rows[i + 1].v === null);
    if (lone) root.append(svg('circle', { cx: X(i), cy: Y(r.v), r: 3.5, class: 'dot' }));
  });
  const cross = svg('line', { x1: 0, x2: 0, y1: PAD.t, y2: base, class: 'cross', visibility: 'hidden' });
  const mark = svg('circle', { cx: 0, cy: 0, r: 5, class: 'marker', visibility: 'hidden' });
  root.append(cross, mark);
  rows.forEach((r, i) => {
    const hit = svg('rect', { x: PAD.l + band * i, y: PAD.t, width: band, height: base - PAD.t, class: 'hit' });
    hover(hit, tipOf(r), () => {
      cross.setAttribute('x1', X(i)); cross.setAttribute('x2', X(i)); cross.setAttribute('visibility', 'visible');
      if (r.v === null) { mark.setAttribute('visibility', 'hidden'); return; }
      mark.setAttribute('cx', X(i)); mark.setAttribute('cy', Y(r.v)); mark.setAttribute('visibility', 'visible');
    }, () => { cross.setAttribute('visibility', 'hidden'); mark.setAttribute('visibility', 'hidden'); });
    root.append(hit);
  });
  root.append(xLabels(rows, band));
  return el('div', { class: 'viz-wrap' }, root);
}

// hbars: labelled horizontal bars (categories), the value written beside.
function hbars(rows, { fmt, tipOf, max: fixedMax }) {
  if (!rows.length) return empty();
  const max = fixedMax || Math.max(...rows.map((r) => r.v)) || 1;
  return el('div', { class: 'hbars' }, rows.map((r) => {
    const line = el('div', { class: 'hbar' },
      el('span', { class: 'hbar-label' }, r.label),
      el('span', { class: 'hbar-track' }, el('i', { style: `width:${Math.max(1, (r.v / max) * 100)}%` })),
      el('span', { class: 'hbar-value' }, fmt(r.v)));
    if (tipOf) hover(line, tipOf(r));
    return line;
  }));
}

// heatmap: the last 60 days as a calendar, a week per column, the time played
// as one blue ramp (lightest = little; an empty day is the plain cell).
function heatmap(days) {
  const levels = [0, 10, 20, 40];
  const level = (m) => (m <= 0 ? 0 : m <= levels[1] ? 1 : m <= levels[2] ? 2 : m <= levels[3] ? 3 : 4);
  const first = new Date(`${days[0].day}T12:00`);
  const offset = (first.getDay() + 6) % 7; // Monday first
  const grid = el('div', { class: 'heat' });
  for (let i = 0; i < offset; i++) grid.append(el('span', { class: 'heat-pad' }));
  days.forEach((d) => {
    const cell = el('span', { class: `heat-cell l${level(d.play_min)}` });
    hover(cell, `<b>${dayLabel(d.day, true)}</b><br>${d.play_min ? `${fmtMin(d.play_min)} · ${d.sessions} séance${d.sessions > 1 ? 's' : ''}` : 'pas joué'}`);
    grid.append(cell);
  });
  return el('div', {},
    el('div', { class: 'heat-days' }, ['L', 'M', 'M', 'J', 'V', 'S', 'D'].map((x) => el('span', {}, x))),
    grid,
    el('div', { class: 'heat-legend' }, 'moins',
      [0, 1, 2, 3, 4].map((l) => el('span', { class: `heat-cell l${l}` })), 'plus'));
}

/* --------------------------------------------------- what to work on --- */

function weakest(n) {
  return (state.sum.items || []).filter((it) => it.seen > 0 && it.box <= 1)
    .sort((a, b) => a.box - b.box || (a.correct / a.seen) - (b.correct / b.seen)).slice(0, n);
}

const itemName = (id) => id.replace(/^(son|syl|mot|phrase):/, '');
const itemKind = (id) => KIND_NAMES[id.split(':')[0]] || '';

/* ---------------------------------------------------------- dashboard --- */

function dashboardTab(list) {
  const s = state.sum;
  const week = totals(lastDays(7));
  const prev = totals((s.days || []).slice(-14, -7));
  const today = lastDays(1)[0] || { play_min: 0, sessions: 0, answers: 0, right: 0 };
  const done = s.episodes.filter((e) => e.completions > 0).length;
  const caught = s.pokedex.filter((p) => p.caught).length;
  const next = s.episodes.find((e) => e.unlocked && e.completions === 0);

  list.append(el('div', { class: 'kpis' },
    kpi('Aujourd\'hui', fmtMin(today.play_min), today.sessions ? `${today.sessions} séance${today.sessions > 1 ? 's' : ''} · ${fmtPct(today.right, today.answers)} de réussite` : 'pas encore joué'),
    kpi('Cette semaine', fmtMin(week.play), `${week.active} jour${week.active > 1 ? 's' : ''} joué${week.active > 1 ? 's' : ''} sur 7`, change(week.play, prev.play)),
    kpi('Réussite (7 j)', fmtPct(week.right, week.answers), `${week.answers} réponses`,
      week.answers && prev.answers ? change((week.right / week.answers) * 100, (prev.right / prev.answers) * 100, { points: true }) : null),
    kpi('Temps de réponse (7 j)', fmtS(week.avg), 'par bonne réponse', change(week.avg, prev.avg, { lowerIsBetter: true })),
    kpi('Série', `🔥 ${s.streak} jour${s.streak > 1 ? 's' : ''}`, `dernier jour : ${s.last_day || '—'}`),
    kpi('Parcours', `${done} / ${s.episodes.length}`, next ? `prochaine : ${next.title}` : 'tout est fait !'),
    kpi('Pokédex', `${caught} / ${s.pokedex.length}`, `⭐ ${s.stars} étoiles`),
  ));

  const d14 = lastDays(14);
  const grid = el('div', { class: 'cards' });
  grid.append(
    card('Temps de jeu', '14 derniers jours',
      d14.some((d) => d.play_min) ? barChart(d14.map((d) => ({ label: dayLabel(d.day), v: d.play_min, d })), {
        fmtY: (v) => `${Math.round(v)}`,
        tipOf: (r) => `<b>${dayLabel(r.d.day, true)}</b><br>${fmtMin(r.v)} · ${r.d.sessions} séance${r.d.sessions > 1 ? 's' : ''}`,
      }) : empty(),
      [['Jour', 'Minutes', 'Séances'], ...d14.map((d) => [dayLabel(d.day), d.play_min, d.sessions])]),
    card('Réussite', '14 derniers jours, en %',
      lineChart(d14.map((d) => ({ label: dayLabel(d.day), v: d.answers ? Math.round((d.right / d.answers) * 100) : null, d })), {
        max: 100, fmtY: (v) => `${v} %`,
        tipOf: (r) => `<b>${dayLabel(r.d.day, true)}</b><br>${r.v === null ? 'pas joué' : `${r.v} % — ${r.d.right} sur ${r.d.answers}`}`,
      }),
      [['Jour', 'Justes', 'Réponses'], ...d14.map((d) => [dayLabel(d.day), d.right, d.answers])]),
  );

  // What to work on: the most fragile items and the slowest ones.
  const weak = weakest(6);
  const slow = (s.slowest || []).slice(0, 6);
  grid.append(card('Ce qui coince', 'à faire relire, à voix haute, à côté de lui',
    weak.length || slow.length ? el('div', { class: 'two-cols' },
      el('div', {}, el('h4', {}, '🧠 Les plus fragiles'), weak.length ? el('ul', { class: 'plain' },
        weak.map((it) => el('li', {}, el('b', {}, itemName(it.id)), ` ${itemKind(it.id).toLowerCase()} · ${it.correct}/${it.seen}`))) : empty('Rien de fragile.')),
      el('div', {}, el('h4', {}, '🐢 Les plus lents'), slow.length ? el('ul', { class: 'plain' },
        slow.map((it) => el('li', {}, el('b', {}, it.text), ` ${fmtS(it.avg_ms)}`))) : empty('Pas encore assez de réponses chronométrées.')))
      : empty()));

  // Where he is: each region's progress.
  const regions = [];
  s.episodes.forEach((e) => {
    let r = regions[regions.length - 1];
    if (!r || r.name !== e.region) regions.push(r = { name: e.region, done: 0, n: 0 });
    r.n++;
    if (e.completions > 0) r.done++;
  });
  grid.append(card('Où il en est', 'les régions de l\'aventure',
    hbars(regions.map((r) => ({ label: r.name, v: r.done, r })), {
      max: Math.max(...regions.map((r) => r.n)), fmt: (v) => v,
      tipOf: (row) => `<b>${row.r.name}</b><br>${row.r.done} route${row.r.done > 1 ? 's' : ''} sur ${row.r.n}`,
    })));

  grid.append(card('Activité', '60 derniers jours', (s.days || []).length ? heatmap(s.days) : empty()));

  const st = s.settings || {};
  if (st.daily_limit) {
    const lessons = (s.today && s.today.lessons) || 0;
    const limit = card('Limite du jour', 'réglée dans Règles du jeu',
      el('p', {}, `${lessons} leçon(s) faite(s) aujourd'hui sur ${st.daily_lessons || 2}, ${st.daily_missions || 3} objectifs en tout.`));
    grid.append(limit);
  }
  list.append(grid);
}

/* --------------------------------------------------------- statistics --- */

function statsTab(list) {
  const s = state.sum;
  state.period = state.period || 14;
  const days = lastDays(state.period);
  const t = totals(days);

  list.append(el('div', { class: 'filters' },
    el('span', { class: 'filters-label' }, 'Période'),
    segmented([[7, '7 jours'], [14, '14 jours'], [30, '30 jours'], [60, '60 jours']], state.period,
      (v) => { state.period = v; render(); })));

  list.append(el('div', { class: 'kpis' },
    kpi('Temps de jeu', fmtMin(t.play), `${t.active} jour${t.active > 1 ? 's' : ''} joué${t.active > 1 ? 's' : ''} sur ${days.length}`),
    kpi('Par jour joué', t.active ? fmtMin(Math.round(t.play / t.active)) : '—', `${t.sessions} séances en tout`),
    kpi('Réponses', String(t.answers), `${fmtPct(t.right, t.answers)} justes`),
    kpi('Temps de réponse', fmtS(t.avg), `moyenne sur ${t.timed} bonnes réponses chronométrées`),
  ));

  const rows = (f) => days.map((d) => ({ label: dayLabel(d.day), d, v: f(d) }));
  const grid = el('div', { class: 'cards' });
  grid.append(
    card('Temps de jeu', 'minutes par jour',
      t.play ? barChart(rows((d) => d.play_min), {
        fmtY: (v) => `${Math.round(v)}`,
        tipOf: (r) => `<b>${dayLabel(r.d.day, true)}</b><br>${fmtMin(r.v)} · ${r.d.sessions} séance${r.d.sessions > 1 ? 's' : ''}`,
      }) : empty(),
      [['Jour', 'Minutes', 'Séances'], ...days.map((d) => [dayLabel(d.day), d.play_min, d.sessions])]),
    card('Réponses', 'questions répondues par jour',
      t.answers ? barChart(rows((d) => d.answers), {
        fmtY: (v) => `${Math.round(v)}`,
        tipOf: (r) => `<b>${dayLabel(r.d.day, true)}</b><br>${r.v} réponses, ${r.d.right} justes`,
      }) : empty(),
      [['Jour', 'Réponses', 'Justes'], ...days.map((d) => [dayLabel(d.day), d.answers, d.right])]),
    card('Réussite', 'en % des réponses du jour',
      lineChart(rows((d) => (d.answers ? Math.round((d.right / d.answers) * 100) : null)), {
        max: 100, fmtY: (v) => `${v} %`,
        tipOf: (r) => `<b>${dayLabel(r.d.day, true)}</b><br>${r.v === null ? 'pas joué' : `${r.v} % — ${r.d.right} sur ${r.d.answers}`}`,
      }),
      [['Jour', '% justes'], ...days.map((d) => [dayLabel(d.day), d.answers ? Math.round((d.right / d.answers) * 100) : '—'])]),
    card('Temps de réponse', 'secondes par bonne réponse — plus bas = plus à l\'aise',
      lineChart(rows((d) => (d.avg_ms ? d.avg_ms / 1000 : null)), {
        fmtY: (v) => `${v.toFixed(1).replace('.', ',')} s`,
        tipOf: (r) => `<b>${dayLabel(r.d.day, true)}</b><br>${r.v === null ? 'pas de réponse chronométrée' : `${fmtS(r.d.avg_ms)} en moyenne (${r.d.timed} réponses)`}`,
      }),
      [['Jour', 'Secondes'], ...days.map((d) => [dayLabel(d.day), d.avg_ms ? (d.avg_ms / 1000).toFixed(1) : '—'])]),
  );

  // By kind of question over the period: time and success.
  const kinds = {};
  days.forEach((d) => Object.entries(d.kinds || {}).forEach(([k, v]) => {
    const a = kinds[k] || (kinds[k] = { answers: 0, right: 0, timed: 0, ms: 0 });
    a.answers += v.answers; a.right += v.right; a.timed += v.timed || 0; a.ms += (v.avg_ms || 0) * (v.timed || 0);
  }));
  const kindRows = Object.keys(KIND_NAMES).filter((k) => kinds[k]).map((k) => ({ k, label: KIND_NAMES[k], ...kinds[k] }));
  grid.append(card('Par type de question', `sur ${state.period} jours`,
    kindRows.length ? el('div', { class: 'two-cols' },
      el('div', {}, el('h4', {}, '⏱ Temps de réponse'), hbars(kindRows.filter((r) => r.timed).map((r) => ({ ...r, v: r.ms / r.timed })), {
        fmt: (v) => fmtS(v), tipOf: (r) => `<b>${r.label}</b><br>${fmtS(r.v)} sur ${r.timed} bonnes réponses`,
      })),
      el('div', {}, el('h4', {}, '✅ Réussite'), hbars(kindRows.map((r) => ({ ...r, v: (r.right / r.answers) * 100 })), {
        max: 100, fmt: (v) => `${Math.round(v)} %`, tipOf: (r) => `<b>${r.label}</b><br>${r.right} justes sur ${r.answers}`,
      }))) : empty(),
    [['Type', 'Réponses', 'Justes', 'Temps moyen'], ...kindRows.map((r) => [r.label, r.answers, r.right, r.timed ? fmtS(r.ms / r.timed) : '—'])]));

  const slow = s.slowest || [];
  grid.append(card('Les plus lents à lire', 'temps habituel, sur au moins deux réponses',
    hbars(slow.map((it) => ({ label: `${it.text}`, v: it.avg_ms, it })), {
      fmt: (v) => fmtS(v),
      tipOf: (r) => `<b>${r.it.text}</b> (${(KIND_NAMES[r.it.kind] || '').toLowerCase()})<br>${fmtS(r.v)} · ${r.it.correct}/${r.it.seen} justes`,
    }),
    [['Élément', 'Temps', 'Justes / vus'], ...slow.map((it) => [it.text, fmtS(it.avg_ms), `${it.correct}/${it.seen}`])]));

  // The parent's one-minute check, if any.
  const runs = s.fluency || [];
  if (runs.length) {
    grid.append(card('Lecture à voix haute', 'mots bien lus par minute, test après test',
      lineChart(runs.map((r) => ({ label: new Date(r.at).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' }), v: r.wcpm, r })), {
        fmtY: (v) => `${Math.round(v)}`,
        tipOf: (row) => `<b>${fmtDate(row.r.at)}</b><br>${row.v} mots/min — ${row.r.right} bien lus, ${row.r.wrong} ratés`,
      }),
      [['Date', 'Mots/min'], ...runs.map((r) => [fmtDate(r.at), r.wcpm])]));
  }

  const exams = (s.exams || []).filter((x) => x.record && x.record.attempts);
  if (exams.length) {
    grid.append(card('Examens', 'meilleur score, et le seuil à atteindre',
      hbars(exams.map((x) => ({ label: x.title, v: x.record.best, x })), {
        max: 100, fmt: (v) => `${v} %`,
        tipOf: (r) => `<b>${r.x.title}</b><br>meilleur ${r.v} % (il faut ${r.x.pass} %) · ${r.x.record.attempts} essai(s)${r.x.record.passed ? ' · réussi ✅' : ''}`,
      })));
  }

  grid.append(card('Activité', '60 derniers jours', (s.days || []).length ? heatmap(s.days) : empty(), null, true));
  list.append(grid);
}
