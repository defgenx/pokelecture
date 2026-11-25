'use strict';

/* The overworld: the home screen as a little map the partner walks on —
   arrow keys (or ZQSD / WASD) on a keyboard; on a tablet, tap a tile and he
   walks there, or use the on-screen pad. One map per region: its routes are
   Poké Ball stops along the path, the tall grass hides wild Pokémon, and the
   Tower, the mini-games and the Pokédex are places to walk into. Stepping on
   anything opens a spoken dialog box with one big button. */

// One map per region (landscape, 13×9), each its own shape, obstacle and
// colours (styles.css .look-*). Legend:
//   T obstacle (the region's tree)  W water (lava, the void…)  . ground
//   = path  F flowers  g tall grass  1–5 route stops, in walking order
//   O Tour  A mini-jeux  P Pokédex  < previous region  > next region
//   D E the region's two trainers  X the professor of a surprise exam
// curriculum/overworld_test.go checks every map: all reachable, the stops in
// order from the entrance, no trainer next to an exit (README « Une carte par région »).
const OW_REGIONS = {
  'Forêt Émeraude': {
    look: 'foret', tree: '🌲',
    map: [
      'TTTTTTTTTTTTT',
      'TT1==XTT.ATTT',
      'TT=T=T3==T5=T',
      'TT=D=W=T=T==T',
      '<==T=W=O=E==>',
      'Tggg=W=T=T=FT',
      'TggP=T=T4==FT',
      'Tggg2==T.F.TT',
      'TTTTTTTTTTTTT',
    ],
  },
  'Colline Azur': {
    look: 'colline', tree: '🌳',
    map: [
      'TTTTTTTTTTTTT',
      'TT.F.TTTTWWWT',
      'TT2===OT=4=AT',
      'TT=DT=TT=T=WT',
      'TT=gg=3==E=.T',
      'TT=ggT.F.T=.T',
      '<=1ggTFX.T5=>',
      'TPggFT..FT.FT',
      'TTTTTTTTTTTTT',
    ],
  },
  'Grotte Violette': {
    look: 'grotte', tree: '⛰️',
    map: [
      'TTTTTTTTTTTTT',
      '<=1===TT=4==T',
      'TTTDT=TT=TE=T',
      'T2====T==OT=T',
      'T=ggTTT=ggg=T',
      'T===3===gTW=T',
      'TT.T=PTWWWW5T',
      'TX...ATWWWW=>',
      'TTTTTTTTTTTTT',
    ],
  },
  'Volcan Rougeoyant': {
    look: 'volcan', tree: '🌋',
    map: [
      'TTTTTTTTTTTTT',
      'T=2===3===4=T',
      'T=TDTOTXTET=T',
      'T=gWWWWWWWg=T',
      'T=gWWWWWWWg=T',
      'T1gWWWWWWWg5T',
      'T=PTTTTTTTA=T',
      '<=ggTTTTTgg=>',
      'TTTTTTTTTTTTT',
    ],
  },
  'Côte Saphir': {
    look: 'cote', tree: '🌴',
    map: [
      'TTTTTTTTTTTTT',
      'TOgg2gTX4gFAT',
      'T.gT=PTK=T..T',
      '<===========>',
      'TW=DWW=WWE=WT',
      'TW1WWW3WWW5WT',
      'TWWWWWWWWWWWT',
      'TWWWWWWWWWWWT',
      'TTTTTTTTTTTTT',
    ],
  },
  'Vallée Arc-en-ciel': {
    look: 'vallee', tree: '🌸',
    map: [
      'TTTTTTTTTTTTT',
      'TTFFTTWXFT5=>',
      'TT===2W..T=FT',
      'TF=DT=WF.T=TT',
      'TT=.T====T=AT',
      'TP1ggTWT3E4FT',
      'TT=ggOWT===TT',
      '<==gFFWTTFTTT',
      'TTTTTTTTTTTTT',
    ],
  },
  'Îles du Levant': {
    look: 'levant', tree: '🎋',
    map: [
      'TTTTTTTTTTTTT',
      'TWWWWPF.WWWWT',
      'T.D==2==W.5=>',
      'Tg.=WF.=W.=.T',
      '<=1=WWW=W4=FT',
      'Tgg.Wg.=W=EAT',
      'TOggW..3==.FT',
      'TWWWWXg.WWWWT',
      'TTTTTTTTTTTTT',
    ],
  },
  'Îles du Couchant': {
    look: 'couchant', tree: '🌵',
    map: [
      'TTTTTTTTTTTTT',
      'Tgg.D.OF.ggTT',
      'Tg.===3===.TT',
      'TA.2WWWWW4.PT',
      'T.T=WWWWW=T.T',
      'T.T=WWWWW=TTT',
      '<=1=TTTTE=5=>',
      'Tgg..XTTTFggT',
      'TTTTTTTTTTTTT',
    ],
  },
  'Plateau de la Ligue': {
    look: 'ligue', tree: '🗿',
    map: [
      'TTTTTTTTTTTTT',
      'TFTTTTTTTTTFT',
      'TOT1TTT3TTTPT',
      'T.T=DTT=TTT.T',
      '<==========5>',
      'TggTT=TTE4ggT',
      'TgATT2TTTTgXT',
      'TFTTTTTTTTTFT',
      'TTTTTTTTTTTTT',
    ],
  },
  'Terres Nouvelles': {
    look: 'terres', tree: '🌻',
    map: [
      'TTTTTTTTTTTTT',
      '<===FTTF.TTFT',
      'T.T=TO=2==.PT',
      'TAT=DT=TT=TTT',
      'TTT1===TT3.FT',
      'TggggTTTT=E.T',
      'TggggggXT4.TT',
      'TFggggFFT==5>',
      'TTTTTTTTTTTTT',
    ],
  },
  'Failles Ultra': {
    look: 'failles', tree: '🌌',
    map: [
      'TTTTTTTTTTTTT',
      'TWWWWWWW=4=WT',
      'TWWWWg3==W=XT',
      'TWWW===PWE=gT',
      'TWWg2AWWWW=gT',
      'TO===WWWWW5=>',
      'Tg=DWWWWWWWWT',
      '<=1WWWWWWWWWT',
      'TTTTTTTTTTTTT',
    ],
  },
  'Académie Pokémon': {
    look: 'academie', tree: '🏫',
    map: [
      'TTTTTTTTTTTTT',
      'TFFTTFTTFT.FT',
      'TD===2==3==.T',
      'T.=T.F.F.T=.T',
      '<=1TXWWW.T4ET',
      'TggT.WWW.T=.T',
      'TggTA.O.PT5=>',
      'TggTTTTTTTTTT',
      'TTTTTTTTTTTTT',
    ],
  },
};
const OW_DEFAULT = OW_REGIONS['Forêt Émeraude'];
// Water you can fish in: not lava, the Ultra void or the Académie fountain.
const OW_NO_FISH = new Set(['volcan', 'failles', 'academie']);
const owRegion = (name) => OW_REGIONS[name] || OW_DEFAULT;

// spoken() marks a literal as a voice line: speech_coverage_test.go finds
// it and checks it is in UIPhrases.
const spoken = (t) => t;

const OW_BLOCKED = new Set(['T', 'W', 'D', 'E']);
const OW_TRAINER = ['👦', '👧', '👨', '👩'];
const OW_STEP_MS = 170;
const OW_ENCOUNTER = 1 / 7; // chance per step in the tall grass

const Overworld = {
  region: null,   // index into regions()
  pos: null,      // {x, y}
  grid: null,     // rows of chars, transposed in portrait
  path: [],       // queued steps of a tap-to-walk
  walking: false,
  keyHandler: null,
  greeted: false,

  regions() {
    const out = [];
    (App.state.episodes || []).forEach((e) => {
      let r = out[out.length - 1];
      if (!r || r.name !== e.region) out.push(r = { name: e.region, eps: [] });
      r.eps.push(e);
    });
    return out;
  },

  // Portrait screens get the map turned a quarter: same places, taller grid.
  layout() {
    const m = owRegion(this.regions()[this.region].name).map;
    const portrait = window.innerHeight > window.innerWidth * 1.1;
    if (!portrait) return m.map((r) => r.split(''));
    const rows = m.length;
    const cols = m[0].length;
    const t = [];
    for (let x = 0; x < cols; x++) {
      const row = [];
      for (let y = 0; y < rows; y++) row.push(m[y][x]);
      t.push(row);
    }
    return t.map((row) => row.map((c) => ({ '<': '^', '>': 'v' }[c] || c)));
  },

  // Today's hidden objects in this region: two ground tiles, the same all
  // day, in landscape coordinates (what the server records).
  sparkles(region) {
    let h = 2166136261;
    for (const c of `${new Date().toDateString()}:${region}`) h = Math.imul(h ^ c.charCodeAt(0), 16777619);
    const rnd = () => { h = Math.imul(h ^ (h >>> 15), 2246822507); h ^= h >>> 13; return (h >>> 0) / 4294967296; };
    const ground = [];
    owRegion(this.regions()[region].name).map
      .forEach((row, y) => row.split('').forEach((c, x) => { if (c === '.' || c === 'F') ground.push(`${x},${y}`); }));
    const spots = [];
    while (spots.length < 2 && ground.length) spots.push(ground.splice(Math.floor(rnd() * ground.length), 1)[0]);
    const found = new Set(App.state.found || []);
    return spots.filter((sp) => !found.has(`${region}:${sp}`));
  },

  // Landscape "x,y" ↔ the grid on screen (turned in portrait).
  toScreen(spot) {
    const [x, y] = spot.split(',').map(Number);
    return this.grid.length > this.grid[0].length ? { x: y, y: x } : { x, y };
  },
  toSpot(p) {
    return this.grid.length > this.grid[0].length ? `${p.y},${p.x}` : `${p.x},${p.y}`;
  },

  find(ch) {
    for (let y = 0; y < this.grid.length; y++) {
      const x = this.grid[y].indexOf(ch);
      if (x >= 0) return { x, y };
    }
    return null;
  },

  /* Entry point (App.home): the region of the next episode, standing on its
     stop — the big JOUER is one tap away from the moment the app opens.
     spawn {from: ±1}: arriving from the previous/next region, one step inside
     the matching exit — found on THIS region's map, every map being different. */
  async open(regionIdx, spawn) {
    Voice.stop();
    WakeLock.off();
    this.stopWalking();
    const regs = this.regions();
    const s = App.state;
    const back = this.region;  // the region shown before (a battle, a game…)
    const backPos = this.pos;
    if (regionIdx === undefined) {
      const nextReg = Math.max(0, regs.findIndex((r) => r.eps.some((e) => e.id === s.next_episode)));
      // Coming back from a session: the same region (in free play the next
      // route is route 1, and every return used to land in the first region).
      // Only moving on by itself when that region's routes are now all done
      // and the next route waits in the region right after.
      const finished = back !== null && regs[back] && regs[back].eps.every((e) => e.done > 0);
      regionIdx = back === null || !regs[back] || (finished && nextReg === back + 1) ? nextReg : back;
    }
    this.region = regionIdx;
    this.grid = this.layout();
    const reg = regs[regionIdx];
    let arrival = null;
    if (spawn && spawn.from) {
      const portrait = this.grid.length > this.grid[0].length;
      const entry = this.find(spawn.from > 0 ? (portrait ? '^' : '<') : (portrait ? 'v' : '>'));
      const step = spawn.from > 0 ? 1 : -1;
      arrival = entry && (portrait ? { x: entry.x, y: entry.y + step } : { x: entry.x + step, y: entry.y });
    }

    // Slots without an episode in this region become plain path.
    for (let k = reg.eps.length + 1; k <= 5; k++) {
      const p = this.find(String(k));
      if (p) this.grid[p.y][p.x] = '=';
    }
    if (regionIdx === 0) this.block('<', '^');
    if (regionIdx === regs.length - 1) this.block('>', 'v');

    this.spots = new Set(this.sparkles(regionIdx).map((sp) => { const p = this.toScreen(sp); return `${p.x},${p.y}`; }));
    const nextIdx = reg.eps.findIndex((e) => e.id === s.next_episode);
    const stop = this.find(String(nextIdx >= 0 ? nextIdx + 1 : 1)) || this.find('1');
    // Back in the same region with no next route here: where he stood.
    const stay = !spawn && regionIdx === back && nextIdx < 0 && backPos && this.free(backPos.x, backPos.y) ? backPos : null;
    this.pos = arrival && this.free(arrival.x, arrival.y) ? arrival : stay || stop || { x: 1, y: 2 };
    if (!Wild.pool && s.tower) Wild.load();
    this.render(reg);
    this.arrive(true);
  },

  block(...chars) {
    chars.forEach((c) => { const p = this.find(c); if (p) this.grid[p.y][p.x] = 'T'; });
  },

  render(reg) {
    const s = App.state;
    const rows = this.grid.length;
    const cols = this.grid[0].length;
    const look = owRegion(reg.name);
    const tree = look.tree;

    const map = el('div', { class: `ow-map look-${look.look}` });
    this.map = map;
    map.style.gridTemplateColumns = `repeat(${cols}, 1fr)`;
    map.style.aspectRatio = `${cols} / ${rows}`;
    map.style.setProperty('--ow-cols', cols);
    map.style.setProperty('--ow-rows', rows);
    this.grid.forEach((row, y) => row.forEach((ch, x) => {
      map.append(this.tile(ch, x, y, reg, tree));
    }));

    const walker = el('div', { class: `ow-walker ${s.hungry ? 'hungry' : ''}` },
      el('img', { src: `sprites/art/${companionId()}.png`, alt: '' }),
      el('span', { class: 'ow-alert' }, '!'),
      el('span', { class: 'ow-hunger' }, '🍓'));
    map.append(walker);
    this.walker = walker;
    this.place(false);

    const dialog = el('div', { class: 'ow-dialog', hidden: true });
    this.dialog = dialog;

    const pad = el('div', { class: 'ow-pad' },
      ['up', 'left', 'right', 'down'].map((d) => {
        const b = el('button', { class: `ow-dir ow-${d}`, 'aria-label': d },
          { up: '▲', down: '▼', left: '◀', right: '▶' }[d]);
        let rep = null;
        const go = () => this.stepDir(d);
        b.addEventListener('pointerdown', (e) => {
          e.preventDefault();
          this.path = [];
          go();
          rep = setInterval(go, OW_STEP_MS + 30);
        });
        const stop = () => { clearInterval(rep); rep = null; };
        b.addEventListener('pointerup', stop);
        b.addEventListener('pointerleave', stop);
        b.addEventListener('pointercancel', stop);
        return b;
      }));

    const regs = this.regions();
    const done = reg.eps.filter((e) => e.done > 0).length;
    App.screen(
      App.topbar(
        // The stats strip shrinks (and scrolls) on a phone; the buttons after
        // it never do. chip-extra: bag items, hidden on a phone (styles.css).
        el('span', { class: 'topbar-stats' }, [
          el('span', { class: 'chip' }, `⭐ ${s.stars}`),
          el('span', { class: 'chip' }, pokeball('pokeball-xs'), ` ${s.balls || 0}`),
          s.streak > 1 ? el('span', { class: 'chip chip-extra' }, `🔥 ${s.streak}`) : null,
          (s.bag && s.bag.superball) ? el('span', { class: 'chip chip-extra' }, pokeball('pokeball-xs superball'), ` ${s.bag.superball}`) : null,
          (s.bag && s.bag.berry) ? el('span', { class: 'chip chip-extra' }, `🍓 ${s.bag.berry}`) : null,
          Trainer.chip(),
          // The day's objective, when the parent set a daily limit.
          s.daily ? el('span', { class: `chip daily-chip ${s.daily.over ? 'over' : ''}` },
            s.daily.over ? '🌙' : s.daily.lessons_over ? '📅 ✓' : `📅 ${s.daily.done}/${s.daily.goal}`) : null,
        ].filter(Boolean)),
        s.event ? el('button', { class: 'btn-icon exam-btn', title: 'Examen surprise', onclick: () => this.examDialog(s.event) }, '🎓') : null,
        el('button', { class: 'btn-icon', title: 'Mon équipe', onclick: () => Team.open() }, '👥'),
        el('button', { class: 'btn-icon quest-btn', title: 'Missions', onclick: () => Quests.panel() },
          `📜 ${(s.quests || []).filter((q) => q.done).length}/${(s.quests || []).length}`),
        // dex-btn: hidden on a phone — the Pokédex is also a place on the map (📘).
        el('button', { class: 'btn-icon dex-btn', title: 'Pokédex', onclick: () => App.pokedex() }, '📘'),
        el('button', { class: 'btn-icon', title: 'Menu', onclick: () => App.menu() }, '☰'),
      ),
      el('div', { class: 'page ow-page' },
        el('div', { class: 'ow-head' },
          Partner.card(),
          el('div', { class: 'ow-region' },
            el('b', {}, `${REGION_EMOJI[reg.name] || '📍'} ${reg.name}`),
            el('small', {}, `${done} / ${reg.eps.length} · région ${this.region + 1} sur ${regs.length}`)),
        ),
        el('div', { class: 'ow-stage' }, map, dialog),
        pad,
      ),
    );

    // Keyboard: arrows, ZQSD (French keyboards), WASD; Entrée/Espace = the
    // dialog's button.
    if (this.keyHandler) document.removeEventListener('keydown', this.keyHandler);
    this.keyHandler = (e) => {
      if (!document.body.contains(map)) {
        document.removeEventListener('keydown', this.keyHandler);
        this.keyHandler = null;
        return;
      }
      const dir = {
        ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right',
        z: 'up', w: 'up', s: 'down', q: 'left', a: 'left', d: 'right',
      }[e.key.length === 1 ? e.key.toLowerCase() : e.key];
      if (dir) { e.preventDefault(); this.path = []; this.stepDir(dir); return; }
      if (e.key === 'Enter' || e.key === ' ') {
        const b = this.dialog && !this.dialog.hidden && $('.btn', this.dialog);
        if (b) { e.preventDefault(); b.click(); }
      }
    };
    document.addEventListener('keydown', this.keyHandler);

    if (!this.greeted) {
      this.greeted = true;
      // Leads, so the line of the stop he stands on waits for it.
      const hello = spoken('Touche une case pour te déplacer, ou utilise les flèches !');
      Voice.lead(uiAudio(hello), hello);
    }
    // A surprise exam waiting today: said once per visit of the map screen.
    if (App.state.event && this.examSaid !== App.state.event.id) {
      this.examSaid = App.state.event.id;
      const line = spoken('Un examen surprise est arrivé ! Trouve le professeur sur la carte !');
      Voice.play(uiAudio(line), line);
    }
  },

  tile(ch, x, y, reg, tree) {
    const t = el('div', { class: 'ow-tile' });
    const ep = /[1-5]/.test(ch) ? reg.eps[Number(ch) - 1] : null;
    switch (ch) {
      case 'T': t.classList.add('ow-tree'); t.textContent = tree; break;
      case 'W': t.classList.add('ow-water'); break;
      case '=': t.classList.add('ow-path'); break;
      case 'F': t.classList.add('ow-flower'); t.textContent = '🌼'; break;
      case 'g': t.classList.add('ow-grass'); t.textContent = '🌿'; break;
      case 'O':
        t.classList.add('ow-place');
        t.textContent = '🗼';
        // The Pokémon the next Tower floor gives: the new ones in sight.
        if (App.state.tower && App.state.tower_next) {
          t.append(el('img', { class: 'ow-place-mon', src: `sprites/art/${App.state.tower_next.pokemon}.png`, alt: '', loading: 'lazy' }));
        }
        break;
      case 'A': t.classList.add('ow-place'); t.textContent = '🎮'; break;
      case 'P': t.classList.add('ow-place'); t.textContent = '📘'; break;
      case 'X':
        if (App.state.event) {
          t.classList.add('ow-trainer', 'ow-prof');
          put(t, el('span', {}, '🎓'), el('i', { class: 'ow-bang' }, '!'));
        }
        break;
      case 'K':
        t.classList.add('ow-trainer', 'ow-fisher');
        put(t, el('span', {}, '👴'), App.state.rod ? null : el('i', { class: 'ow-bang' }, '🎣'));
        break;
      case 'D': case 'E': {
        const n = ch === 'D' ? 0 : 1;
        const beaten = (App.state.beaten || []).includes(`${this.region}:${n}`);
        t.classList.add('ow-trainer');
        if (beaten) t.classList.add('beaten');
        put(t, el('span', {}, OW_TRAINER[(this.region * 2 + n) % OW_TRAINER.length]),
          beaten ? null : el('i', { class: 'ow-bang' }, '!'));
        break;
      }
      case '<': case '^': t.classList.add('ow-exit'); t.textContent = ch === '<' ? '⬅️' : '⬆️'; break;
      case '>': case 'v': t.classList.add('ow-exit'); t.textContent = ch === '>' ? '➡️' : '⬇️'; break;
      default:
        if (ep) {
          const isNext = ep.id === App.state.next_episode;
          t.classList.add('ow-path', 'ow-stop');
          if (ep.done > 0) t.classList.add('done');
          if (isNext) t.classList.add('next');
          if (!ep.unlocked) t.classList.add('locked');
          if (ep.gate) t.append(el('i', { class: 'ow-gate' }, '🎓'));
          t.append(pokeball('ow-ball'),
            el('img', { src: `sprites/art/${ep.reward_pokemon}.png`, alt: '', loading: 'lazy' }),
            el('small', {}, stopLabel(ep)));
        }
    }
    if (this.spots && this.spots.has(`${x},${y}`)) t.append(el('i', { class: 'ow-sparkle' }, '✨'));
    t.addEventListener('click', () => (ch === 'W' ? this.waterTap(x, y) : this.walkTo(x, y)));
    return t;
  },

  place(animate = true) {
    const rows = this.grid.length;
    const cols = this.grid[0].length;
    this.walker.style.transition = animate ? `left ${OW_STEP_MS}ms linear, top ${OW_STEP_MS}ms linear` : 'none';
    this.walker.style.left = `${(this.pos.x / cols) * 100}%`;
    this.walker.style.top = `${(this.pos.y / rows) * 100}%`;
    // On a route stop he steps aside, smaller: drawn over it, he hid the
    // stop he stands on — one of the Conseil's four doors looked missing.
    this.walker.classList.toggle('on-stop', /[1-5]/.test(this.grid[this.pos.y][this.pos.x]));
  },

  free(x, y) {
    const row = this.grid[y];
    return !!row && x >= 0 && x < row.length && !OW_BLOCKED.has(row[x]);
  },

  stepDir(dir) {
    if (this.walking) return;
    const [dx, dy] = { up: [0, -1], down: [0, 1], left: [-1, 0], right: [1, 0] }[dir];
    this.step(dx, dy);
  },

  step(dx, dy) {
    const nx = this.pos.x + dx;
    const ny = this.pos.y + dy;
    if (dx) this.walker.classList.toggle('face-left', dx < 0);
    if (!this.free(nx, ny)) { this.bump(); return false; }
    this.walking = true;
    this.pos = { x: nx, y: ny };
    this.place(true);
    this.walker.classList.remove('hop');
    void this.walker.offsetWidth;
    this.walker.classList.add('hop');
    setTimeout(() => {
      this.walking = false;
      this.arrive(false);
      if (this.path.length) {
        const n = this.path.shift();
        this.step(n.x - this.pos.x, n.y - this.pos.y);
      }
    }, OW_STEP_MS);
    return true;
  },

  bump() {
    this.walker.classList.remove('bump');
    void this.walker.offsetWidth;
    this.walker.classList.add('bump');
  },

  stopWalking() { this.path = []; this.walking = false; },

  // Tap-to-walk: breadth-first search over the free tiles. A tap on an
  // obstacle walks to the nearest free tile next to it.
  walkTo(tx, ty) {
    let goal = { x: tx, y: ty };
    if (!this.free(tx, ty)) {
      const near = [[0, 1], [0, -1], [1, 0], [-1, 0]].map(([dx, dy]) => ({ x: tx + dx, y: ty + dy }))
        .filter((p) => this.free(p.x, p.y));
      if (!near.length) return this.bump();
      goal = near[0];
    }
    const key = (p) => `${p.x},${p.y}`;
    const prev = new Map([[key(this.pos), null]]);
    const queue = [this.pos];
    while (queue.length) {
      const cur = queue.shift();
      if (cur.x === goal.x && cur.y === goal.y) break;
      for (const [dx, dy] of [[0, 1], [0, -1], [1, 0], [-1, 0]]) {
        const n = { x: cur.x + dx, y: cur.y + dy };
        if (this.free(n.x, n.y) && !prev.has(key(n))) { prev.set(key(n), cur); queue.push(n); }
      }
    }
    if (!prev.has(key(goal))) return this.bump();
    const route = [];
    for (let p = goal; p && key(p) !== key(this.pos); p = prev.get(key(p))) route.unshift(p);
    if (!route.length) return this.arrive(false);
    this.path = route;
    if (!this.walking) {
      const n = this.path.shift();
      this.step(n.x - this.pos.x, n.y - this.pos.y);
    }
  },

  /* What is under his feet now: a stop, a place, an exit, the tall grass. */
  arrive(first) {
    const ch = this.grid[this.pos.y][this.pos.x];
    const s = App.state;
    const reg = this.regions()[this.region];
    if (this.fishAt && !this.path.length) {
      const f = this.fishAt;
      this.fishAt = null;
      if (Math.abs(this.pos.x - f.x) + Math.abs(this.pos.y - f.y) === 1) return this.goFish();
    }
    if (this.path.length && !'<>^v'.includes(ch)) {
      // Walking through: only the grass can stop him on the way.
      if (ch === 'g') this.grassRoll();
      return;
    }
    const here = `${this.pos.x},${this.pos.y}`;
    if (this.spots && this.spots.has(here) && !first) return this.pickUp(here);
    const trainer = this.trainerNear();
    if (trainer !== null && !/[1-5OAP]/.test(ch)) return this.trainerDialog(trainer);
    if (/[1-5]/.test(ch)) return this.stopDialog(reg.eps[Number(ch) - 1], first);
    if (ch === 'X' && s.event) return this.examDialog(s.event);
    if (ch === 'K') return this.fisherDialog();
    switch (ch) {
      case 'O': return s.tower
        ? this.show('🗼 La Tour de Combat', [el('span', {}, `Étage ${s.tower_floor + 1}`),
          s.tower_next ? el('span', { class: 'ow-dialog-reward' }, '🎁 ', el('img', { class: 'ow-dialog-mon', src: `sprites/art/${s.tower_next.pokemon}.png`, alt: '' }), ` étage ${s.tower_next.floor}`) : null],
        'ENTRER ▶', () => Session.start('tour'), spoken('La Tour de Combat ! Monte les étages en lisant !'))
        : this.show('🗼 La Tour de Combat', '🔒', null, null, spoken('Termine la première route pour ouvrir ça !'));
      case 'A': return s.tower
        ? this.show('🎮 Les mini-jeux', '', 'ENTRER ▶', () => Arcade.hub(), spoken('Le coin des mini-jeux !'))
        : this.show('🎮 Les mini-jeux', '🔒', null, null, spoken('Termine la première route pour ouvrir ça !'));
      case 'P': return this.show('📘 Le Pokédex', '', 'OUVRIR ▶', () => App.pokedex(), spoken('Ton Pokédex ! Regarde tous tes Pokémon.'));
      case '<': case '^': return this.leave(-1);
      case '>': case 'v': return this.leave(1);
      case 'g':
        this.hide();
        if (!first) this.grassRoll();
        return;
      default: return this.hide();
    }
  },

  // A trainer standing on a tile next to him (n = 0 or 1), or null.
  trainerNear() {
    for (const [dx, dy] of [[0, 1], [0, -1], [1, 0], [-1, 0]]) {
      const c = (this.grid[this.pos.y + dy] || [])[this.pos.x + dx];
      if (c === 'D') return 0;
      if (c === 'E') return 1;
    }
    return null;
  },

  trainerDialog(n) {
    const s = App.state;
    const key = `${this.region}:${n}`;
    const face = OW_TRAINER[(this.region * 2 + n) % OW_TRAINER.length];
    if (!s.tower) return this.show(`${face} Un Dresseur`, '🔒', null, null, spoken('Termine la première route pour ouvrir ça !'));
    if ((s.beaten || []).includes(key)) {
      return this.show(`${face} Dresseur battu`, '⭐', null, null, spoken('Bravo, tu m\'as battu ! Reviens demain pour une revanche.'));
    }
    Sfx.encounter();
    const mon = (s.trainer_mons || {})[key];
    this.show(`${face} Un Dresseur te défie !`,
      [mon ? el('img', { class: 'ow-dialog-mon', src: `sprites/art/${mon.pokemon}.png`, alt: '' }) : null, el('span', {}, mon ? `${mon.name} · ⚔️ 3 mots` : '⚔️ 3 mots à lire')],
      'COMBAT ▶', () => Session.start(`dresseur-${this.region}-${n}`), spoken('Un Dresseur veut se battre ! Lis vite pour gagner !'));
    // Then the name of the Pokémon he sends out, in the track's voice.
    if (mon) Voice.play(mon.name_audio, mon.name);
  },

  // A sparkle under his feet: the server rolls what it was.
  async pickUp(here) {
    this.stopWalking();
    this.spots.delete(here);
    const tile = this.map && this.map.children[this.pos.y * this.grid[0].length + this.pos.x];
    const sp = tile && $('.ow-sparkle', tile);
    if (sp) sp.remove();
    let res;
    try {
      res = await api('api/find', { region: this.region, spot: this.toSpot(this.pos) });
      App.state = res.state;
    } catch (_) { return; }
    const r = res.reward;
    const icon = { balls: '🔴', superball: '🔵', candy: '🍬', berry: '🍓' }[r.kind] || '🎁';
    Sfx.catch_();
    burst(icon);
    confetti(30);
    this.show(`${icon} Objet trouvé !`, r.kind === 'balls' ? `+${r.count} Poké Ball${r.count > 1 ? 's' : ''}` : '', null, null, null);
    await Voice.play(r.line.audio, r.line.text);
    await celebrateAll(res);
  },

  // The professor of today's surprise exam (curriculum.EventToday).
  examDialog(ev) {
    this.stopWalking();
    Sfx.encounter();
    this.show(`🎓 ${ev.title}`,
      [el('img', { class: 'ow-dialog-mon', src: `sprites/art/${ev.pokemon}.png`, alt: '' }), el('span', {}, `🎯 ${ev.pass} %`)],
      'EXAMEN ▶', () => Session.start(`examen-${ev.id}`), spoken('C\'est l\'examen surprise ! Tu es prêt ?'));
  },

  stopDialog(ep, first) {
    // A door with an entry exam: the exam is how one goes in.
    if (ep.gate) {
      return this.show(`🎓 ${ep.title}`, '🔒', 'EXAMEN ▶', () => Session.start(`examen-${ep.gate}`),
        spoken('Pour entrer ici, réussis l\'examen du professeur ! Il t\'attend sur la carte.'));
    }
    const stars = starRow(ep.stars);
    const sounds = el('span', { class: 'sounds' }, (ep.sounds || []).map((g) => el('i', {}, g)));
    if (!ep.unlocked) return this.show(`🔒 ${ep.legendary ? 'Arène légendaire' : `Route ${ep.route}`}`, '', null, null, spoken('Termine la route précédente !'));
    const isNext = ep.id === App.state.next_episode && ep.done === 0;
    this.show(ep.title, [sounds, el('span', { class: 'stars' }, stars)],
      isNext ? 'JOUER ▶' : 'REJOUER ▶', () => Session.start(ep.id),
      first && this.greeted && !isNext ? null
        : isNext ? spoken('C\'est la prochaine route ! Touche JOUER !') : spoken('Tu as déjà gagné ici. Tu veux rejouer ?'));
  },

  show(title, body, label, action, phrase) {
    const d = this.dialog;
    d.innerHTML = '';
    // append() would print a null as the text "null": only real nodes go in.
    d.append(...[
      el('div', { class: 'ow-dialog-text' }, el('b', {}, title), body ? el('div', { class: 'ow-dialog-body' }, body) : null),
      label ? el('button', { class: 'btn btn-huge btn-go', onclick: () => { this.stopWalking(); action(); } }, label) : null,
    ].filter(Boolean));
    d.hidden = false;
    if (phrase) say(phrase);
  },

  hide() { if (this.dialog) this.dialog.hidden = true; },

  leave(dir) {
    const regs = this.regions();
    const to = this.region + dir;
    if (to < 0 || to >= regs.length) return this.hide();
    if (!regs[to].eps[0].unlocked && !(App.state.settings && App.state.settings.free_play)) {
      this.show(`${REGION_EMOJI[regs[to].name] || '📍'} ${regs[to].name}`, '🔒', null, null, spoken('Cette région est encore fermée ! Termine les routes d\'ici.'));
      return;
    }
    say(dir > 0 ? 'En route vers la région suivante !' : 'Retour à la région précédente !');
    this.open(to, { from: dir });
  },

  grassRoll() {
    const s = App.state;
    if (dayOver() || !s.tower || !Wild.pool || !Wild.available().length || (s.balls || 0) <= 0) return;
    if (Math.random() >= OW_ENCOUNTER) return;
    this.stopWalking();
    this.walker.classList.add('alert');
    Sfx.encounter();
    setTimeout(() => {
      document.removeEventListener('keydown', this.keyHandler);
      this.keyHandler = null;
      Wild.mode = 'grass';
      Wild.encounter();
    }, 700);
  },

  /* A tap on water: with the rod, walk to the bank and fish; without it,
     the fisherman's hint. README « La pêche ». */
  waterTap(x, y) {
    const reg = this.regions()[this.region];
    if (OW_NO_FISH.has(owRegion(reg.name).look)) return this.walkTo(x, y);
    if (!App.state.rod) {
      this.walkTo(x, y);
      return this.show('🎣 La pêche', '🔒', null, null,
        spoken('Il te faut une canne à pêche. Le Pêcheur de la Côte Saphir en a une !'));
    }
    if (!this.walking && Math.abs(this.pos.x - x) + Math.abs(this.pos.y - y) === 1) return this.goFish();
    this.fishAt = { x, y };
    this.walkTo(x, y);
  },

  goFish() {
    this.fishAt = null;
    this.stopWalking();
    document.removeEventListener('keydown', this.keyHandler);
    this.keyHandler = null;
    Wild.fish();
  },

  fisherDialog() {
    if (App.state.rod) {
      return this.show('👴 Le Pêcheur', '🎣', null, null, spoken('Bonne pêche ! Touche l\'eau pour pêcher.'));
    }
    return this.show('👴 Le Pêcheur', [el('span', {}, '🎣 = '), el('span', {}, '⭐'.repeat(4))], null, null,
      spoken('Je suis le Pêcheur ! Lis une route presque sans faute, et ma canne à pêche est à toi !'));
  },
};
