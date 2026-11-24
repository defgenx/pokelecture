'use strict';

/* Parent pages (admin, voice studio) behind the Google sign-in: any call the
   server answers 401 sends the browser to the sign-in, and the header shows
   who is signed in. With the sign-in off (no GOOGLE_CLIENT_ID), nothing shows. */

(() => {
  const page = location.pathname.split('/').pop() || 'admin.html';
  const login = () => { location.href = `auth/login?next=${encodeURIComponent(page)}`; };

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
    bar.append(who, out);
    (document.querySelector('header') || document.body).append(bar);
  }).catch(() => {});
})();
