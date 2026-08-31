'use strict';

/* Studio voix — record your own voice over any line the game speaks.
   Hold the record button while you talk, release to upload. The server stores it
   under the same /audio/<key> URL the game already requests, so a recording is
   a drop-in replacement for the synthetic voice. */

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

const GROUPS = [
  ['sons', 'Sons'],
  ['syllabes', 'Syllabes'],
  ['mots', 'Mots'],
  ['phrases', 'Phrases'],
];

const state = { texts: [], group: 'sons', query: '', onlyTts: false };

/* ------------------------------------------------------------------ mic --- */

const Mic = {
  stream: null,
  mime: '',

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
    return this.stream;
  },

  // start() resolves once recording is live and returns stop(), which resolves
  // with the recorded Blob. Keeping it promise-shaped avoids juggling state
  // across pointerdown/pointerup handlers.
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

/* ------------------------------------------------------------------ api --- */

async function loadTexts() {
  const res = await fetch('api/texts');
  if (!res.ok) throw new Error(await res.text());
  state.texts = await res.json();
}

async function upload(key, blob) {
  const res = await fetch(`api/record/${key}`, {
    method: 'PUT',
    headers: { 'Content-Type': blob.type || 'application/octet-stream' },
    body: blob,
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

async function revert(key) {
  const res = await fetch(`api/record/${key}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

/* ---------------------------------------------------------------- render --- */

function tally() {
  const done = state.texts.filter((t) => t.recorded).length;
  const sounds = state.texts.filter((t) => t.group === 'sons');
  const soundsDone = sounds.filter((t) => t.recorded).length;
  $('#tally').textContent =
    `${done}/${state.texts.length} en voix réelle · sons : ${soundsDone}/${sounds.length}`;
}

function tabs() {
  const nav = $('#tabs');
  nav.innerHTML = '';
  GROUPS.forEach(([id, label]) => {
    const n = state.texts.filter((t) => t.group === id).length;
    nav.append(el('button', {
      class: state.group === id ? 'on' : '',
      onclick: () => { state.group = id; render(); },
    }, `${label} (${n})`));
  });
}

function visible() {
  const q = state.query.trim().toLowerCase();
  return state.texts.filter((t) => {
    if (t.group !== state.group) return false;
    if (state.onlyTts && t.recorded) return false;
    return !q || t.text.toLowerCase().includes(q);
  });
}

function row(t) {
  const badge = el('span', { class: 'badge' }, t.recorded ? '● ta voix' : 'synthèse');
  const revertBtn = el('button', { class: 'revert', disabled: !t.recorded, title: 'Revenir à la synthèse' }, '↺');
  const recBtn = el('button', { class: 'rec' }, '● ENREGISTRER');
  const node = el('div', { class: `row ${t.recorded ? 'done' : ''}` },
    el('div', { class: 'txt' }, el('b', {}, t.text), el('small', {}, `${t.style} · ${t.key}`)),
    el('button', { class: 'play' }, '▶'),
    recBtn,
    revertBtn,
    badge,
  );

  const refresh = () => {
    node.classList.toggle('done', t.recorded);
    badge.textContent = t.recorded ? '● ta voix' : 'synthèse';
    revertBtn.disabled = !t.recorded;
  };

  // Cache-bust so a fresh recording is heard instead of the previous take.
  $('.play', node).addEventListener('click', () => {
    new Audio(`${t.url}?v=${Date.now()}`).play().catch(() => {});
  });

  revertBtn.addEventListener('click', async () => {
    await revert(t.key);
    t.recorded = false;
    refresh();
    tally();
  });

  let stop = null;
  const begin = async (ev) => {
    ev.preventDefault();
    if (stop) return;
    if (!Mic.supported()) return;
    recBtn.classList.add('armed');
    recBtn.textContent = '● …PARLE';
    node.classList.add('busy');
    try {
      stop = await Mic.start();
    } catch (err) {
      stop = null;
      recBtn.classList.remove('armed');
      recBtn.textContent = '● ENREGISTRER';
      node.classList.remove('busy');
      warn(`Micro refusé : ${err.message}`);
    }
  };
  const end = async () => {
    if (!stop) return;
    const finish = stop;
    stop = null;
    recBtn.classList.remove('armed');
    recBtn.textContent = '⏳';
    try {
      const blob = await finish();
      if (blob.size > 0) {
        await upload(t.key, blob);
        t.recorded = true;
      }
    } catch (err) {
      warn(`Envoi impossible : ${err.message}`);
    } finally {
      recBtn.textContent = '● ENREGISTRER';
      node.classList.remove('busy');
      refresh();
      tally();
    }
  };

  recBtn.addEventListener('pointerdown', begin);
  recBtn.addEventListener('pointerup', end);
  recBtn.addEventListener('pointerleave', end);
  recBtn.addEventListener('pointercancel', end);

  return node;
}

function warn(msg) {
  const n = $('#warn');
  n.hidden = false;
  n.textContent = msg;
}

function render() {
  tabs();
  tally();
  const list = $('#list');
  list.innerHTML = '';
  const rows = visible();
  if (!rows.length) {
    list.append(el('p', { style: 'color:#6a7180' }, 'Rien à afficher avec ce filtre.'));
    return;
  }
  rows.forEach((t) => list.append(row(t)));
}

/* ------------------------------------------------------------------ boot --- */

$('#q').addEventListener('input', (e) => { state.query = e.target.value; render(); });
$('#only-tts').addEventListener('change', (e) => { state.onlyTts = e.target.checked; render(); });

(async () => {
  if (!window.isSecureContext) {
    warn("Le micro est bloqué : cette page doit être ouverte sur http://localhost:8080/parent.html (pas via l'adresse IP).");
  } else if (!Mic.supported()) {
    warn("Ce navigateur n'expose pas MediaRecorder. Utilise Chrome sur l'ordinateur.");
  }
  try {
    await loadTexts();
    render();
  } catch (err) {
    warn(`Chargement impossible : ${err.message}`);
  }
})();
