'use strict';

/* Studio voix — a page of the espace parent (README « Studio voix »): record
   your own voice over any line the game speaks. Hold the button while you
   talk, release to send: the server stores it under the same /audio/<key>
   URL the game already asks for, so a recording replaces the synthetic voice
   with nothing else to change. ↺ goes back to the synthesis. Recordings are
   for the whole game, not one player. */

const STUDIO_GROUPS = [
  ['sons', '🔊 Sons'], ['syllabes', '🔤 Syllabes'], ['mots', '📖 Mots'], ['phrases', '💬 Phrases'],
];
const STUDIO_PAGE = 100;

/* ------------------------------------------------------------------ mic --- */

const Mic = {
  stream: null,
  mime: '',
  analyser: null,

  supported() {
    return !!(navigator.mediaDevices && window.MediaRecorder && window.isSecureContext);
  },

  pickMime() {
    // AAC first: an .m4a recording plays everywhere, iPad included, while
    // iPads refuse WebM/Opus and would fall back to the synthetic voice.
    const wanted = ['audio/mp4', 'audio/webm;codecs=opus', 'audio/webm'];
    return wanted.find((m) => MediaRecorder.isTypeSupported(m)) || '';
  },

  async open() {
    if (this.stream) return this.stream;
    this.stream = await navigator.mediaDevices.getUserMedia({
      audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true },
    });
    this.mime = this.pickMime();
    try {
      const ctx = new (window.AudioContext || window.webkitAudioContext)();
      this.analyser = ctx.createAnalyser();
      this.analyser.fftSize = 512;
      ctx.createMediaStreamSource(this.stream).connect(this.analyser);
    } catch (_) { this.analyser = null; }
    return this.stream;
  },

  // level is the microphone's loudness now, 0–1 (the meter while recording).
  level() {
    if (!this.analyser) return 0;
    const buf = new Uint8Array(this.analyser.fftSize);
    this.analyser.getByteTimeDomainData(buf);
    let peak = 0;
    for (const v of buf) peak = Math.max(peak, Math.abs(v - 128));
    return Math.min(1, peak / 90);
  },

  // start() resolves once recording is live and returns stop(), which
  // resolves with the recorded Blob.
  async start() {
    const stream = await this.open();
    const rec = new MediaRecorder(stream, this.mime ? { mimeType: this.mime } : undefined);
    const chunks = [];
    rec.ondataavailable = (e) => { if (e.data && e.data.size) chunks.push(e.data); };
    const stopped = new Promise((resolve) => { rec.onstop = () => resolve(new Blob(chunks, { type: rec.mimeType })); });
    rec.start();
    return async () => {
      if (rec.state !== 'inactive') rec.stop();
      return stopped;
    };
  },
};

// meter animates a level bar while fn's recording runs; returns the stopper.
function runMeter(bar) {
  let on = true;
  const tick = () => {
    if (!on) return;
    bar.style.setProperty('--level', `${Math.round(Mic.level() * 100)}%`);
    requestAnimationFrame(tick);
  };
  tick();
  return () => { on = false; bar.style.setProperty('--level', '0%'); };
}

/* ------------------------------------------------------------------ api --- */

async function loadTexts() {
  const res = await fetch('api/texts');
  if (!res.ok) throw new Error(await res.text());
  state.texts = await res.json();
}

async function uploadTake(key, blob) {
  const res = await fetch(`api/record/${key}`, {
    method: 'PUT',
    headers: { 'Content-Type': blob.type || 'application/octet-stream' },
    body: blob,
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

async function revertTake(key) {
  const res = await fetch(`api/record/${key}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

// Cache-busted, so a fresh take is heard instead of the previous one.
function playLine(t) {
  const a = new Audio(`${t.url}?v=${Date.now()}`);
  return new Promise((resolve) => {
    a.onended = resolve;
    a.onerror = resolve;
    a.play().catch(resolve);
  });
}

/* ------------------------------------------------------ hold to record --- */

// holdToRecord wires a button: press = record, release = send. onDone(true)
// once the take is stored. Space does the same while the guided mode is open.
function holdToRecord(btn, meter, t, onDone) {
  let stop = null;
  let stopMeter = null;
  const idle = btn.innerHTML;
  const begin = async (ev) => {
    if (ev) ev.preventDefault();
    if (stop || !Mic.supported()) return;
    btn.classList.add('armed');
    btn.textContent = '● Je t\'écoute…';
    try {
      stop = await Mic.start();
      if (meter) stopMeter = runMeter(meter);
    } catch (err) {
      stop = null;
      btn.classList.remove('armed');
      btn.innerHTML = idle;
      warn(`Micro refusé : ${err.message}`);
    }
  };
  const end = async () => {
    if (!stop) return;
    const finish = stop;
    stop = null;
    if (stopMeter) stopMeter();
    btn.classList.remove('armed');
    btn.textContent = '⏳ Envoi…';
    let ok = false;
    try {
      const blob = await finish();
      if (blob.size > 0) {
        await uploadTake(t.key, blob);
        t.recorded = true;
        ok = true;
      }
    } catch (err) {
      warn(`Envoi impossible : ${err.message}`);
    } finally {
      btn.innerHTML = idle;
    }
    if (onDone) onDone(ok);
  };
  btn.addEventListener('pointerdown', begin);
  btn.addEventListener('pointerup', end);
  btn.addEventListener('pointerleave', end);
  btn.addEventListener('pointercancel', end);
  return { begin, end };
}

/* --------------------------------------------------------------- page --- */

function studioTab(list) {
  if (!state.texts) {
    list.append(el('p', { class: 'empty' }, 'Chargement des phrases du jeu…'));
    loadTexts().then(() => { if (state.tab === 'studio') render(); })
      .catch((err) => warn(`Chargement impossible : ${err.message}`));
    return;
  }
  const texts = state.texts;
  state.studio = state.studio || { group: 'sons', show: 'all', q: '', limit: STUDIO_PAGE };
  const sv = state.studio;

  if (!window.isSecureContext) {
    list.append(el('div', { class: 'card notice' },
      el('b', {}, '🎙 Micro bloqué'),
      el('p', {}, 'Le micro n\'est permis que sur une page sécurisée : ouvre l\'espace parent en https, ou sur localhost sur cet ordinateur.')));
  } else if (!Mic.supported()) {
    list.append(el('div', { class: 'card notice' },
      el('b', {}, '🎙 Pas d\'enregistrement ici'),
      el('p', {}, 'Ce navigateur n\'enregistre pas le son. Utilise Chrome, Edge, Firefox ou Safari récents.')));
  }

  // Progress, per group.
  const done = texts.filter((t) => t.recorded).length;
  list.append(el('div', { class: 'kpis' },
    kpi('En ta voix', `${done} / ${texts.length}`, `${Math.round((done / texts.length) * 100)} % des phrases du jeu`),
    ...STUDIO_GROUPS.map(([id, label]) => {
      const g = texts.filter((t) => t.group === id);
      const r = g.filter((t) => t.recorded).length;
      return kpi(label, `${r} / ${g.length}`, id === 'sons' ? 'les plus utiles à enregistrer' : `${Math.round((r / Math.max(1, g.length)) * 100)} %`);
    })));

  // The guided mode and the mic check.
  const meter = el('span', { class: 'meter' });
  const todo = texts.filter((t) => t.group === sv.group && !t.recorded);
  const actions = el('div', { class: 'card studio-actions' },
    el('div', {},
      el('h3', {}, '▶ Enregistrer à la suite'),
      el('small', {}, 'Une phrase à la fois, en grand : écoute la synthèse, maintiens pour parler, ta prise est rejouée, puis la suivante. La barre d\'espace marche aussi.')),
    el('div', { class: 'studio-buttons' },
      el('button', { class: 'btn primary', onclick: () => guided(todo) },
        `Commencer (${STUDIO_GROUPS.find((g) => g[0] === sv.group)[1]})`),
      el('button', {
        class: 'btn ghost',
        onclick: async () => {
          try {
            await Mic.open();
            const stopMeter = runMeter(meter);
            setTimeout(stopMeter, 4000);
          } catch (err) { warn(`Micro refusé : ${err.message}`); }
        },
      }, '🎤 Tester le micro'),
      meter));
  list.append(actions);

  // Filters.
  const search = el('input', { class: 'search', type: 'search', placeholder: 'Chercher une phrase…', value: sv.q });
  search.addEventListener('input', () => { sv.q = search.value; sv.limit = STUDIO_PAGE; paint(); });
  list.append(el('div', { class: 'filters' },
    segmented(STUDIO_GROUPS.map(([id, label]) => [id, `${label} ${texts.filter((t) => t.group === id).length}`]), sv.group,
      (v) => { sv.group = v; sv.limit = STUDIO_PAGE; render(); }),
    segmented([['all', 'Toutes'], ['todo', 'À enregistrer'], ['mine', 'Ma voix']], sv.show,
      (v) => { sv.show = v; sv.limit = STUDIO_PAGE; render(); }),
    search));

  const box = el('div', { class: 'studio-list' });
  list.append(box);
  const paint = () => {
    const q = sv.q.trim().toLowerCase();
    const rows = texts.filter((t) => t.group === sv.group
      && (sv.show === 'all' || (sv.show === 'mine') === t.recorded)
      && (!q || t.text.toLowerCase().includes(q)));
    box.innerHTML = '';
    if (!rows.length) { box.append(el('p', { class: 'empty' }, 'Rien à afficher avec ce filtre.')); return; }
    rows.slice(0, sv.limit).forEach((t) => box.append(studioRow(t)));
    if (rows.length > sv.limit) {
      box.append(el('button', { class: 'btn ghost more', onclick: () => { sv.limit += STUDIO_PAGE; paint(); } },
        `Afficher plus (${rows.length - sv.limit} de plus)`));
    }
  };
  paint();
}

function studioRow(t) {
  const badge = el('span', { class: `badge ${t.recorded ? 'mine' : ''}` });
  const meter = el('span', { class: 'meter small' });
  const rec = el('button', { class: 'rec', title: 'Maintiens pendant que tu parles' }, '● Enregistrer');
  const back = el('button', { class: 'linkbtn small', title: 'Revenir à la voix de synthèse' }, '↺');
  const node = el('div', { class: 'studio-row' },
    el('div', { class: 'txt' }, el('b', {}, t.text), el('small', {}, `${t.style} · ${t.key}`)),
    el('button', { class: 'linkbtn small', title: 'Écouter ce que le jeu dit', onclick: () => playLine(t) }, '▶'),
    rec, meter, back, badge);
  const paint = () => {
    node.classList.toggle('done', !!t.recorded);
    badge.textContent = t.recorded ? '● ta voix' : 'synthèse';
    badge.classList.toggle('mine', !!t.recorded);
    back.disabled = !t.recorded;
  };
  back.addEventListener('click', async () => {
    try { await revertTake(t.key); t.recorded = false; paint(); } catch (err) { warn(`Impossible : ${err.message}`); }
  });
  holdToRecord(rec, meter, t, (ok) => { paint(); if (ok) playLine(t); });
  paint();
  return node;
}

/* ---------------------------------------------------------- guided mode --- */

// guided walks the lines to record one by one, as an overlay: the line big,
// the synthesis first, hold (or Space) to record, the take played back, next.
function guided(queue) {
  if (!queue.length) { warn('Tout ce groupe est déjà enregistré dans ta voix.'); return; }
  let i = 0;
  const overlay = el('div', { class: 'studio-overlay', role: 'dialog', 'aria-label': 'Enregistrer à la suite' });
  const close = () => {
    document.removeEventListener('keydown', onKey);
    document.removeEventListener('keyup', onKeyUp);
    overlay.remove();
    render();
  };
  let ctl = null;
  const show = async () => {
    if (i >= queue.length) {
      overlay.innerHTML = '';
      overlay.append(el('div', { class: 'studio-stage' },
        el('div', { class: 'studio-big' }, '🎉'),
        el('p', {}, 'Ce groupe est enregistré. Merci !'),
        el('button', { class: 'btn primary', onclick: close }, 'Terminer')));
      return;
    }
    const t = queue[i];
    const meter = el('span', { class: 'meter big' });
    const rec = el('button', { class: 'rec big' }, '● Maintiens pour parler');
    overlay.innerHTML = '';
    overlay.append(el('div', { class: 'studio-stage' },
      el('div', { class: 'studio-step' }, `${i + 1} / ${queue.length}`),
      el('div', { class: 'studio-big' }, t.text),
      el('small', {}, `${t.style} · ${t.key}`),
      el('div', { class: 'studio-buttons' },
        el('button', { class: 'btn ghost', onclick: () => playLine(t) }, '▶ Écouter la synthèse'),
        el('button', { class: 'btn ghost', onclick: () => { i++; show(); } }, 'Passer ➜')),
      rec, meter,
      el('button', { class: 'linkbtn', onclick: close }, 'Arrêter')));
    ctl = holdToRecord(rec, meter, t, async (ok) => {
      if (!ok) return;
      rec.textContent = '✓ Enregistré — écoute ta prise';
      await playLine(t);
      i++;
      show();
    });
    await playLine(t); // the synthesis first: what to say, and how
  };
  // Space = hold to record, Escape = stop.
  const onKey = (e) => {
    if (e.key === 'Escape') return close();
    if (e.code === 'Space' && !e.repeat && ctl) { e.preventDefault(); ctl.begin(); }
  };
  const onKeyUp = (e) => { if (e.code === 'Space' && ctl) { e.preventDefault(); ctl.end(); } };
  document.addEventListener('keydown', onKey);
  document.addEventListener('keyup', onKeyUp);
  document.body.append(overlay);
  show();
}
