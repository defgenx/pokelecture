'use strict';

/* Our own picture set, as inline SVG.
   Emoji are not usable for this content: an abstract term like the type
   "Normal" or the stat "Vitesse" has no emoji that means anything to a
   five-year-old, and the newer ones (🪽, 🪺, 🪨) are simply missing from the
   emoji font on older Android builds — they render as an empty box. These
   always draw, scale cleanly, and carry the type colours.

   Every name here must exist in curriculum.Icons on the Go side, which is what
   makes `pokecontent check` fail on a typo instead of the child seeing a blank
   tile. All shapes are authored on a 48×48 grid. */

const ICONS = {
  // A bite mark: dark mouth with big white teeth closing from both sides.
  bite: { c: '#ce4069', b: '<circle cx="24" cy="24" r="21" fill="#ce4069"/><path d="M9 18h30v12H9Z" fill="#5c1226"/><path d="M11 18l5 9 5-9 5 9 5-9 5 9 4-9Z" fill="#fff"/><path d="M11 30l5-9 5 9 5-9 5 9 5-9 4 9Z" fill="#fff"/>' },
  // One big white canine on a dark disc — the same visual family as `bite`, so
  // the two read as related but never as each other.
  fang: { c: '#5c1226', b: '<circle cx="24" cy="24" r="21" fill="#5c1226"/><path d="M12 9c8-3 16-3 24 0l-4 14c-2 10-5 16-8 20-3-4-6-10-8-20L12 9Z" fill="#fff"/><path d="M13 12c7-2 15-2 22 0" stroke="#e2b3bf" stroke-width="4" fill="none"/>' },
  water: { c: '#4d90d5', b: '<path d="M24 3c8 12 14 19 14 27a14 14 0 0 1-28 0C10 22 16 15 24 3Z"/><ellipse cx="18" cy="31" rx="3.5" ry="5" fill="#fff" opacity=".55"/>' },
  flame: { c: '#ff9d55', b: '<path d="M24 2c10 11 16 18 16 27a16 16 0 0 1-32 0C8 20 14 13 24 2Z" fill="#ff7a2f"/><path d="M24 17c5 6 8 10 8 14a8 8 0 0 1-16 0c0-4 3-8 8-14Z" fill="#ffd23c"/>' },
  leaf: { c: '#63bb5b', b: '<path d="M40 6C18 6 8 16 8 30c0 5 2 9 5 12L40 6Z"/><path d="M40 6 13 42" stroke="#2f7a2c" stroke-width="3" fill="none"/>' },
  bolt: { c: '#f4d23c', b: '<path d="M28 2 10 27h10l-4 19 20-27H26l4-17Z" stroke="#c99b00" stroke-width="2"/>' },
  wing: { c: '#8fa9de', b: '<path d="M45 7C28 7 6 16 3 35c8-4 13-3 17-1-2-5 0-9 4-11-1 5 1 8 5 9 2-6 8-12 16-25Z" fill="#8fa9de"/><path d="M20 34c3-8 9-16 20-25" stroke="#6b86c4" stroke-width="2.5" fill="none"/>' },
  rock: { c: '#c7b78b', b: '<path d="M10 34 6 20l10-10 16-4 10 12-4 16-14 6-14-6Z" stroke="#8d7f57" stroke-width="2.5"/><path d="M16 10l8 12 8-16M24 22l-14 12M24 22l16 6" stroke="#8d7f57" stroke-width="2" fill="none"/>' },
  shield: { c: '#5a8ea2', b: '<path d="M24 3 42 9v14c0 12-8 19-18 22C14 42 6 35 6 23V9L24 3Z"/><path d="M24 12v22" stroke="#fff" stroke-width="3" opacity=".7"/>' },
  star: { c: '#f4d23c', b: '<path d="m24 3 6 14 15 1-11 10 3 15-13-8-13 8 3-15L3 18l15-1 6-14Z"/>' },
  sparkle: { c: '#ec8fe6', b: '<path d="m24 2 5 13 13 5-13 5-5 13-5-13-13-5 13-5 5-13Z"/><circle cx="40" cy="10" r="3.5"/><circle cx="9" cy="37" r="2.5"/>' },
  tornado: { c: '#8fa9de', b: '<path d="M5 8h38M9 17h30M14 26h21M19 34h12M23 42h5" stroke="#6b86c4" stroke-width="5" stroke-linecap="round" fill="none"/>' },
  sleep: { c: '#5269ad', b: '<path d="M14 8h14L14 22h14" stroke="#5269ad" stroke-width="4.5" fill="none" stroke-linecap="round"/><path d="M28 26h13L28 38h13" stroke="#5269ad" stroke-width="4" fill="none" stroke-linecap="round"/>' },
  powder: { c: '#ab6ac8', b: '<circle cx="14" cy="14" r="5"/><circle cx="30" cy="9" r="3.5"/><circle cx="38" cy="20" r="5"/><circle cx="22" cy="26" r="4"/><circle cx="33" cy="35" r="3.5"/><circle cx="12" cy="34" r="4.5"/>' },
  cut: { c: '#5a8ea2', b: '<path d="M12 4l22 30M36 4 14 34" stroke="#5a8ea2" stroke-width="4.5" fill="none" stroke-linecap="round"/><circle cx="13" cy="40" r="6" fill="#fff" stroke="#5a8ea2" stroke-width="4"/><circle cx="35" cy="40" r="6" fill="#fff" stroke="#5a8ea2" stroke-width="4"/>' },
  ball: { c: '#ff9d55', b: '<circle cx="24" cy="24" r="19" fill="#ff9d55"/><path d="M24 5a19 19 0 0 1 0 38" fill="#e07a2c"/><ellipse cx="17" cy="15" rx="6" ry="4" fill="#fff" opacity=".55"/>' },
  trainer: { c: '#ee1515', b: '<path d="M8 30c0-11 7-18 16-18s16 7 16 18H8Z"/><path d="M40 30c4 0 5 3 5 5H3c0-2 1-5 5-5h32Z" fill="#c40d0d"/><circle cx="24" cy="20" r="5" fill="#fff"/>' },
  heart: { c: '#ec8fe6', b: '<path d="M24 43C10 33 4 26 4 18A11 11 0 0 1 24 11 11 11 0 0 1 44 18c0 8-6 15-20 25Z"/>' },
  fall: { c: '#4d90d5', b: '<path d="M24 4v26" stroke="#4d90d5" stroke-width="5" fill="none" stroke-linecap="round"/><path d="M11 26l13 17 13-17Z" fill="#4d90d5"/>' },
  bomb: { c: '#3f4552', b: '<circle cx="22" cy="30" r="15"/><path d="M33 16c3-5 7-6 11-4" stroke="#9099a1" stroke-width="4" fill="none" stroke-linecap="round"/><path d="m30 15 6 6" stroke="#3f4552" stroke-width="7" stroke-linecap="round"/><circle cx="44" cy="10" r="4" fill="#ff9d55"/>' },
  shadow: { c: '#5269ad', b: '<ellipse cx="24" cy="44" rx="19" ry="3.5" fill="#3d4a7a" opacity=".45"/><circle cx="24" cy="14" r="9" fill="#5269ad"/><path d="M6 42c0-10 8-16 18-16s18 6 18 16Z" fill="#5269ad"/>' },
  cocoon: { c: '#90c12c', b: '<ellipse cx="24" cy="24" rx="13" ry="20"/><path d="M12 17h24M11 24h26M12 31h24M15 38h18" stroke="#5d7f16" stroke-width="2.5" fill="none"/>' },
  candy: { c: '#ce4069', b: '<circle cx="24" cy="24" r="11"/><path d="M13 17 4 10l3 10-3 10 9-7ZM35 17l9-7-3 10 3 10-9-7Z"/>' },
  // Tranche: three claw slashes, the standard visual shorthand for a cut.
  slash: { c: '#4d90d5', b: '<path d="M8 4c6 10 9 24 8 40-5-9-8-24-8-40Z" fill="#4d90d5"/><path d="M23 2c6 10 9 24 8 40-5-9-8-24-8-40Z" fill="#4d90d5"/><path d="M38 4c6 10 9 24 8 40-5-9-8-24-8-40Z" fill="#4d90d5"/>' },
  dance: { c: '#f97176', b: '<path d="M18 38V14l18-6v24" stroke="#f97176" stroke-width="4.5" fill="none" stroke-linecap="round"/><ellipse cx="13" cy="38" rx="6.5" ry="5" fill="#f97176"/><ellipse cx="31" cy="32" rx="6.5" ry="5" fill="#f97176"/>' },
  sand: { c: '#e0c068', b: '<circle cx="37" cy="11" r="6" fill="#f4d23c"/><path d="M2 42c6-11 11-8 16-15 4-6 10-9 28-11v26H2Z" fill="#e0c068"/><path d="M2 42c8-6 14-4 20-9 5-4 12-6 24-7" stroke="#c2a047" stroke-width="2.5" fill="none"/>' },
  impact: { c: '#ff9d55', b: '<path d="m24 1 6 12 12-7-6 13 13 5-13 5 6 13-12-7-6 12-6-12-12 7 6-13-13-5 13-5-6-13 12 7 6-12Z"/>' },
  rage: { c: '#ce4069', b: '<path d="m24 2 5 11 11-4-4 11 11 5-11 5 4 11-11-4-5 11-5-11-11 4 4-11L1 30l11-5-4-11 11 4 5-11Z" fill="#ce4069"/><path d="M16 20l7 4-7 4M32 20l-7 4 7 4" stroke="#fff" stroke-width="3.5" fill="none" stroke-linecap="round"/>' },
  speed: { c: '#4d90d5', b: '<path d="M4 14h26M10 24h22M4 34h26" stroke="#4d90d5" stroke-width="5" fill="none" stroke-linecap="round"/><path d="m32 10 12 14-12 14Z" fill="#4d90d5"/>' },
  bulb: { c: '#63bb5b', b: '<circle cx="24" cy="18" r="13"/><path d="M24 31v13" stroke="#2f7a2c" stroke-width="4" fill="none" stroke-linecap="round"/><path d="M24 38c-6 0-9-3-9-8 5 0 9 3 9 8ZM24 38c6 0 9-3 9-8-5 0-9 3-9 8Z" fill="#2f7a2c"/>' },
  normal: { c: '#9099a1', b: '<circle cx="24" cy="24" r="19"/><circle cx="24" cy="24" r="8" fill="#fff"/>' },
  psy: { c: '#f97176', b: '<circle cx="24" cy="24" r="19" fill="#f97176"/><path d="M24 34c-6 0-9-4-9-9s4-9 9-9 8 3 8 7-3 6-6 6-4-2-4-4" fill="none" stroke="#fff" stroke-width="4" stroke-linecap="round"/>' },
  ice: { c: '#74cec0', b: '<path d="M24 3v42M6 12l36 24M42 12 6 36" stroke="#37b3a2" stroke-width="4.5" stroke-linecap="round" fill="none"/><path d="M24 9l-5-5M24 9l5-5M24 39l-5 5M24 39l5 5M11 16l-6-1M11 16l1-6M37 32l6 1M37 32l-1 6M37 16l6-1M37 16l-1-6M11 32l-6 1M11 32l1 6" stroke="#37b3a2" stroke-width="3.5" stroke-linecap="round" fill="none"/>' },
};

/* iconEl builds the SVG node. An unknown name falls back to a star and shouts in
   the console rather than leaving an empty tile — the exact bug this file exists
   to eliminate. */
function iconEl(name, cls) {
  let ic = ICONS[name];
  if (!ic) {
    console.warn(`[pokelecture] icône inconnue: ${name}`);
    ic = ICONS.star;
  }
  const wrap = document.createElement('span');
  wrap.className = cls || 'icon';
  wrap.style.color = ic.c;
  wrap.innerHTML =
    `<svg viewBox="0 0 48 48" fill="${ic.c}" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">${ic.b}</svg>`;
  return wrap;
}
