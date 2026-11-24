'use strict';

/* Two more mini-games, added to the Arcade hub of app.js:

   🥁 Le tambour des syllabes — hear a word, count its syllables (oral
      phonological awareness, the skill under blending).
   🧩 Le son manquant — a word with one sound missing, pick the grapheme that
      fills the gap (encoding: from sound to letters).

   Ten rounds each, the record is the number found first try. */

// Graphemes that spell the same sound: never offered as two different
// answers to one gap, or the child would be right and told he is wrong.
const HOMOPHONES = [
  ['o', 'au', 'eau'], ['é', 'er'], ['è', 'ê', 'ai', 'ei'], ['an', 'am', 'en'],
  ['on', 'om'], ['in', 'ain'], ['s', 'ss', 'ce', 'ci'], ['c', 'k', 'qu'],
  ['f', 'ph', 'ff'], ['i', 'y'], ['m', 'mm'], ['p', 'pp'], ['t', 'tt'],
  ['l', 'll'], ['n', 'nn'], ['r', 'rr'], ['j', 'ge', 'gi'], ['ou', 'oo'], ['z', 's'],
];

// The server's list (curriculum.Homophones) wins; the local one is the
// fallback for a pool fetched before it carried one.
function sameSound(a, b) {
  const groups = (Arcade.pool && Arcade.pool.homophones) || HOMOPHONES;
  return a === b || groups.some((g) => g.includes(a) && g.includes(b));
}

const VOWELISH = new Set(['voyelle', 'digraphe', 'trigraphe']);

// A written syllable that is a consonant + mute e ("che", "re") is not an oral
// syllable: "Salamèche" is three beats, not four. Those words stay out of the
// drum game rather than teach a wrong count.
function muteFinal(syl) {
  const s = syl.toLowerCase();
  return s.length > 1 && s.endsWith('e') && !/[aeiouyéèêàâîôû]/.test(s.slice(-2, -1));
}

Object.assign(Arcade, {
  /* 📚 Mon livre: five sentences he has already read, one per page, with
     their picture. He reads aloud (any word can be tapped), then « J'AI LU ! »
     reads the page back word by word. Re-reading is what builds fluency.
     The score is the number of words read. */
  async livre() {
    const all = this.pool.sentences || [];
    if (!all.length) return this.hub();
    // The latest pages first: what he learned most recently needs it most.
    const pages = all.slice(-12).sort(() => Math.random() - 0.5).slice(0, 5);
    let page = 0;
    let words = 0;
    const stage = el('div', { class: 'act', style: 'width:100%' });
    const pips = el('div', { class: 'pips-holder' });
    this.screen('📚 Mon livre', 'Lis la page à voix haute, puis touche J\'AI LU !', pips, stage);

    const turn = () => {
      pips.innerHTML = '';
      pips.append(progressPips(page, pages.length));
      if (page >= pages.length) {
        confetti(60);
        say('Bravo ! Tu as lu tout le livre !');
        return this.gameOver('livre', words, false, '📚 Mon livre');
      }
      const p = pages[page];
      const chips = p.words.map((w, k) => el('button', { class: 'read-word', onclick: () => Voice.play(p.word_audio[k], w) }, w));
      const pic = p.sprite ? el('img', { class: 'book-pic', src: p.sprite, alt: '' })
        : p.icon ? iconEl(p.icon, 'book-pic') : el('span', { class: 'book-pic em' }, p.emoji || '📖');
      const read = el('button', { class: 'btn btn-huge btn-go' }, 'J\'AI LU ! 🔊');
      read.addEventListener('click', async () => {
        read.disabled = true;
        await karaoke(p.audio, p.text, chips);
        words += p.words.length;
        cheerCompanion();
        Sfx.correct();
        await sleep(400);
        page++;
        turn();
      }, { once: true });
      stage.innerHTML = '';
      stage.append(el('div', { class: 'book-page' }, pic, el('div', { class: 'read-sentence' }, chips)), read);
    };
    turn();
  },

  /* 🥁 Ten words: hear it, tap the number of beats, then the drum plays it
     back syllable by syllable. */
  async tambour() {
    const words = (this.pool.words || []).filter((w) =>
      (w.sprite || w.emoji || w.icon) && w.syllables && w.syllables.length >= 1 && w.syllables.length <= 4
      && !muteFinal(w.syllables[w.syllables.length - 1]));
    if (words.length < 4) return this.hub();
    this.items = [];
    let score = 0;
    let round = 0;
    const ROUNDS = 10;
    const deck = [...words].sort(() => Math.random() - 0.5);

    const scoreChip = el('span', { class: 'chip' }, '🥁 0');
    let pips = el('div');
    const stage = el('div', { class: 'act', style: 'width:100%' });
    this.screen('🥁 Le tambour des syllabes', 'Combien de syllabes ? Tape sur le bon tambour !', scoreChip, pips, stage);

    const ask = () => {
      if (round >= ROUNDS) return this.gameOver('tambour', score, false, '🥁 Le tambour des syllabes');
      const w = deck[round % deck.length];
      pips.replaceWith(pips = progressPips(round, ROUNDS));
      round++;
      let first = true;
      stage.innerHTML = '';
      const face = w.sprite ? el('img', { src: w.sprite, alt: '' })
        : w.icon ? iconEl(w.icon) : el('span', {}, w.emoji);
      const pic = el('button', { class: `tile big-pic ${w.sprite ? 'sprite' : w.icon ? 'ico' : 'emoji'}`, onclick: () => Voice.play(w.audio, w.text) }, face);
      const wordEl = el('div', { class: 'word arcade-word' }, w.text);
      const drums = el('div', { class: 'drum-row' });
      [1, 2, 3, 4].forEach((n) => {
        const b = el('button', { class: 'drum-btn' }, el('span', { class: 'drum-dots' }, '●'.repeat(n)), el('b', {}, String(n)));
        b.addEventListener('click', async () => {
          if (drums.classList.contains('done') || b.disabled) return;
          if (n === w.syllables.length) {
            drums.classList.add('done');
            b.classList.add('good');
            if (first) { score += this.gain(b); scoreChip.textContent = `🥁 ${score}`; }
            Sfx.correct();
            burst('🥁');
            // The answer, performed: each syllable on its own beat.
            wordEl.innerHTML = '';
            w.syllables.forEach((syl) => wordEl.append(el('span', { class: 'syl' }, syl)));
            const spans = [...wordEl.children];
            for (let k = 0; k < spans.length; k++) {
              spans[k].classList.add('beat');
              Sfx.drum();
              await Voice.play((w.syllable_audio || [])[k], w.syllables[k]);
              await sleep(250);
            }
            await Voice.play(w.audio, w.text);
            setTimeout(ask, 500);
          } else {
            first = false;
            b.disabled = true;
            b.classList.add('bad');
            Sfx.wrong(); this.miss();
            Voice.play(w.audio, w.text);
          }
        });
        drums.append(b);
      });
      stage.append(pic, wordEl, el('button', { class: 'btn btn-ghost', onclick: () => Voice.play(w.audio, w.text) }, '🔊 Réécouter'), drums);
      Voice.play(w.audio, w.text);
    };
    ask();
  },

  /* 🧩 Ten words with one sound blanked out; three graphemes to choose from,
     each one sounding itself when tapped. */
  async trou() {
    const sounds = new Map((this.pool.sounds || []).map((s) => [s.id, s]));
    const words = (this.pool.words || []).filter((w) =>
      (w.sprite || w.emoji || w.icon) && (w.graphemes || []).length >= 2
      && w.graphemes.some((g) => holeable(g, sounds)));
    if (words.length < 4 || sounds.size < 6) return this.hub();
    this.items = [];
    let score = 0;
    let round = 0;
    const ROUNDS = 10;
    const deck = [...words].sort(() => Math.random() - 0.5);

    const scoreChip = el('span', { class: 'chip' }, '🧩 0');
    let pips = el('div');
    const stage = el('div', { class: 'act', style: 'width:100%' });
    this.screen('🧩 Le son manquant', 'Quel son manque ? Écoute le mot, trouve les lettres !', scoreChip, pips, stage);

    const ask = () => {
      if (round >= ROUNDS) return this.gameOver('trou', score, false, '🧩 Le son manquant');
      const w = deck[round % deck.length];
      pips.replaceWith(pips = progressPips(round, ROUNDS));
      round++;
      const chunks = displayChunks(w.text, w.graphemes);
      const holes = chunks.map((c, i) => i).filter((i) => holeable(chunks[i].g, sounds));
      const hole = holes[Math.floor(Math.random() * holes.length)];
      const answer = chunks[hole].g;

      // Distractors: other taught graphemes of the same family (vowel sound
      // for a vowel gap) that do not sound like the answer.
      const kind = VOWELISH.has(sounds.get(answer).kind) ? 'v' : 'c';
      const pool = [...sounds.values()].filter((s) => s.kind !== 'muet' && s.id !== 'h'
        && (VOWELISH.has(s.kind) ? 'v' : 'c') === kind && !sameSound(s.id, answer));
      pool.sort(() => Math.random() - 0.5);
      const choices = [answer];
      for (const s of pool) {
        if (choices.length === 3) break;
        if (!choices.some((c) => sameSound(c, s.id))) choices.push(s.id);
      }
      choices.sort(() => Math.random() - 0.5);

      let first = true;
      stage.innerHTML = '';
      const face = w.sprite ? el('img', { src: w.sprite, alt: '' })
        : w.icon ? iconEl(w.icon) : el('span', {}, w.emoji);
      const pic = el('button', { class: `tile big-pic ${w.sprite ? 'sprite' : w.icon ? 'ico' : 'emoji'}`, onclick: () => Voice.play(w.audio, w.text) }, face);
      const gap = el('span', { class: 'gap' }, '?');
      const wordEl = el('div', { class: 'word arcade-word gap-word' },
        chunks.map((c, i) => (i === hole ? gap : c.sep + c.text)));
      const grid = el('div', { class: 'grid three' });
      choices.forEach((id) => {
        const t = el('button', { class: 'tile grapheme-tile' }, id);
        t.addEventListener('click', async () => {
          if (t.disabled || grid.classList.contains('done')) return;
          await Voice.play(sounds.get(id) && sounds.get(id).audio, id);
          if (id === answer) {
            grid.classList.add('done');
            t.classList.add('good');
            gap.textContent = chunks[hole].text;
            gap.classList.add('filled');
            if (first) { score += this.gain(t); scoreChip.textContent = `🧩 ${score}`; }
            Sfx.correct();
            burst('🧩');
            await Voice.play(w.audio, w.text);
            setTimeout(ask, 600);
          } else {
            first = false;
            t.disabled = true;
            t.classList.add('bad');
            Sfx.wrong(); this.miss();
          }
        });
        grid.append(t);
      });
      stage.append(pic, wordEl, grid);
      Voice.play(w.audio, w.text);
    };
    ask();
  },
});

// A gap is worth asking when its grapheme is audible and ours to sound out.
function holeable(g, sounds) {
  const s = sounds.get(g);
  return !!s && s.kind !== 'muet' && g !== 'h';
}

// displayChunks maps the server's lowercase grapheme cut back onto the word as
// written (capitals, hyphens, spaces kept), one chunk per grapheme.
function displayChunks(text, graphemes) {
  const out = [];
  let i = 0;
  for (const g of graphemes) {
    let sep = '';
    while (i < text.length && " -'’".includes(text[i])) sep += text[i++];
    out.push({ g, sep, text: text.slice(i, i + g.length) });
    i += g.length;
  }
  if (i < text.length && out.length) out[out.length - 1].text += text.slice(i);
  return out;
}

Arcade.GAMES.push(
  { id: 'livre', icon: '📚', name: 'Mon livre', desc: 'Relis tes histoires, page par page. La voix lit avec toi !', lower: false, unit: ' mots' },
  { id: 'tambour', icon: '🥁', name: 'Le tambour des syllabes', desc: 'Écoute le mot et compte ses syllabes. 10 mots !', lower: false, unit: '' },
  { id: 'trou', icon: '🧩', name: 'Le son manquant', desc: 'Un son a disparu du mot : retrouve-le ! 10 mots.', lower: false, unit: '' },
);
