'use strict';

/* Parent pages (admin, voice studio) behind the Google sign-in: any call the
   server answers 401 brings back the page's sign-in button, and the header
   shows who is signed in, with a link back to the game. With the sign-in off (no GOOGLE_CLIENT_ID), nothing shows. */

(() => {
  const page = location.pathname.split('/').pop() || 'admin';
  // A call refused mid-use (the session ended): reload the page, which then
  // shows its sign-in button — never a jump straight to Google.
  const login = () => { location.href = page; };

  const raw = window.fetch.bind(window);
  window.fetch = async (...args) => {
    const res = await raw(...args);
    if (res.status === 401) login();
    return res;
  };

  raw('auth/me').then((r) => r.json()).then((me) => {
    if (!me.enabled || !me.email) return;
    const bar = document.createElement('div');
    bar.className = 'auth-bar';
    const who = document.createElement('span');
    who.textContent = `Connecté : ${me.email}`;
    const out = document.createElement('a');
    out.href = 'auth/logout';
    out.textContent = 'Se déconnecter';
    const game = document.createElement('a');
    game.href = './';
    game.textContent = '← Retour au jeu';
    bar.append(game, who, out);
    (document.querySelector('header') || document.body).append(bar);
  }).catch(() => {});
})();
