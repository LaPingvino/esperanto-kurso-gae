/**
 * eo.js — Client-side logic for Esperanto-kurso
 */

// ---- Token Management ----

(function () {
  'use strict';

  const TOKEN_KEY = 'eo_token';

  function getToken() {
    return localStorage.getItem(TOKEN_KEY);
  }

  function setToken(token) {
    if (token) localStorage.setItem(TOKEN_KEY, token);
  }

  // Token is now created server-side on first page load (via AuthMiddleware).
  // This function checks if we already have it in localStorage; if not,
  // pick it up from the X-New-Token response header sent by the server.
  function ensureToken() {
    return getToken();
  }

  // Attach token to every HTMX request.
  document.addEventListener('htmx:configRequest', function (evt) {
    const token = getToken();
    if (token) {
      evt.detail.headers['X-Auth-Token'] = token;
    }
  });

  // Capture new token from HTMX responses.
  document.addEventListener('htmx:afterRequest', function (evt) {
    const newToken = evt.detail.xhr && evt.detail.xhr.getResponseHeader('X-New-Token');
    if (newToken) {
      setToken(newToken);
    }
  });

  // On load, sync localStorage token from the server-set cookie.
  // The cookie is authoritative after passkey/magic-link login, so always
  // overwrite localStorage when they differ (avoids stale anonymous token).
  document.addEventListener('DOMContentLoaded', function () {
    const match = document.cookie.match(/(?:^|;\s*)token=([^;]+)/);
    if (match) {
      const cookieToken = decodeURIComponent(match[1]);
      if (cookieToken !== getToken()) setToken(cookieToken);
    }
  });

  // Expose ensureToken globally for use by WebAuthn flows.
  window.getEoToken = getToken;
  window.setEoToken = setToken;
  window.ensureEoToken = ensureToken;

})();


// ---- WebAuthn Registration ----

async function registerPasskey() {
  const statusEl = document.getElementById('passkey-status');
  function status(msg) { if (statusEl) statusEl.textContent = msg; }

  try {
    status('Komencas registradon…');
    const token = window.getEoToken ? window.getEoToken() : null;
    const headers = { 'Content-Type': 'application/json' };
    if (token) headers['X-Auth-Token'] = token;

    // Step 1: get options from server.
    const beginRes = await fetch('/auth/passkey/register/begin', {
      method: 'POST',
      headers: headers
    });
    if (!beginRes.ok) throw new Error(await beginRes.text());
    const options = await beginRes.json();

    // Decode base64url fields required by the WebAuthn API.
    options.publicKey.challenge = base64urlToBuffer(options.publicKey.challenge);
    options.publicKey.user.id = base64urlToBuffer(options.publicKey.user.id);
    if (options.publicKey.excludeCredentials) {
      options.publicKey.excludeCredentials = options.publicKey.excludeCredentials.map(c => ({
        ...c,
        id: base64urlToBuffer(c.id)
      }));
    }

    // Step 2: call browser WebAuthn API.
    status('Bonvolu sekvi la instrukciojn de via aparato…');
    const credential = await navigator.credentials.create(options);

    // Step 3: send credential to server.
    const credJSON = credentialToJSON(credential);
    const finishRes = await fetch('/auth/passkey/register/finish', {
      method: 'POST',
      headers: headers,
      body: JSON.stringify(credJSON)
    });
    if (!finishRes.ok) throw new Error(await finishRes.text());

    status('Ensalutŝlosilo sukcese registrita!');
  } catch (err) {
    console.error('Passkey registration failed:', err);
    const statusEl = document.getElementById('passkey-status');
    if (statusEl) statusEl.textContent = 'Eraro: ' + err.message;
  }
}


// ---- WebAuthn Login ----

async function loginPasskey(mediation) {
  const statusEl = document.getElementById('passkey-status');
  function status(msg) { if (statusEl) statusEl.textContent = msg; }

  try {
    if (!mediation) status('Komencas ensaluton…');

    // Step 1: get options from server.
    const beginRes = await fetch('/auth/passkey/login/begin', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    });
    if (!beginRes.ok) throw new Error(await beginRes.text());
    const options = await beginRes.json();

    options.publicKey.challenge = base64urlToBuffer(options.publicKey.challenge);
    if (options.publicKey.allowCredentials) {
      options.publicKey.allowCredentials = options.publicKey.allowCredentials.map(c => ({
        ...c,
        id: base64urlToBuffer(c.id)
      }));
    }
    if (mediation) options.mediation = mediation;

    // Step 2: call browser WebAuthn API.
    if (!mediation) status('Bonvolu sekvi la instrukciojn de via aparato…');
    const assertion = await navigator.credentials.get(options);
    if (!assertion) return; // user dismissed or no credential available

    // Step 3: send assertion to server.
    const token = window.getEoToken ? window.getEoToken() : null;
    const headers = { 'Content-Type': 'application/json' };
    if (token) headers['X-Auth-Token'] = token;

    const assertJSON = credentialToJSON(assertion);
    const finishRes = await fetch('/auth/passkey/login/finish', {
      method: 'POST',
      headers: headers,
      body: JSON.stringify(assertJSON)
    });
    if (!finishRes.ok) throw new Error(await finishRes.text());

    const data = await finishRes.json();
    if (data.token) {
      window.setEoToken(data.token);
      if (!mediation || mediation === 'optional') {
        status('Ensaluto sukcesa! Reŝarĝas…');
        setTimeout(() => window.location.href = '/', 800);
      } else {
        // Silent/conditional: just reload to pick up the new token.
        window.location.reload();
      }
    }
  } catch (err) {
    // Suppress AbortError from conditional mediation (user didn't interact).
    if (err.name === 'AbortError' || err.name === 'NotAllowedError') return;
    console.error('Passkey login failed:', err);
    const statusEl = document.getElementById('passkey-status');
    if (statusEl) statusEl.textContent = 'Eraro: ' + err.message;
  }
}

// On page load: if the user has no token and the browser supports conditional
// mediation, silently prompt for a saved passkey via the autofill UI.
document.addEventListener('DOMContentLoaded', async function () {
  if (window.getEoToken && window.getEoToken()) return; // already have a session
  if (!window.PublicKeyCredential) return;
  try {
    const supported = await PublicKeyCredential.isConditionalMediationAvailable?.();
    if (supported) {
      loginPasskey('conditional');
    } else {
      // Fallback: try a silent get — resolves immediately if a credential exists.
      loginPasskey('silent');
    }
  } catch (e) { /* ignore — not all browsers support this */ }
});


// ---- Copy Magic Link ----

function copyMagicLink() {
  const input = document.getElementById('magic-link');
  const confirm = document.getElementById('copy-confirm');
  if (!input) return;

  if (navigator.clipboard) {
    navigator.clipboard.writeText(input.value).then(function () {
      if (confirm) confirm.textContent = 'Kopiita!';
    });
  } else {
    input.select();
    document.execCommand('copy');
    if (confirm) confirm.textContent = 'Kopiita!';
  }
}


// ---- Add vocab definition (bypasses form nesting issues) ----

async function addVocabDef(slug, lang, prefix) {
  const id = prefix ? 'add-def-input-' + prefix + '-' + slug : 'add-def-input-' + slug;
  const input = document.getElementById(id);
  if (!input) return;
  const text = input.value.trim();
  if (!text) { input.focus(); return; }

  const token = window.getEoToken ? window.getEoToken() : null;
  const headers = { 'Content-Type': 'application/x-www-form-urlencoded' };
  if (token) headers['X-Auth-Token'] = token;

  const body = new URLSearchParams({ text: text, language: lang, from: 'ekzerco' });
  try {
    const resp = await fetch('/tradukoj/' + slug, { method: 'POST', headers: headers, body: body, redirect: 'follow' });
    // Server responds with a redirect to /ekzerco/{slug}?added_lang=...&added_def=...
    if (resp.redirected) { window.location.href = resp.url; return; }
  } catch (e) {
    console.error('addVocabDef failed:', e);
  }
  window.location.href = '/ekzerco/' + slug;
}


// ---- Admin: Show/Hide Content Fields ----

function showContentFields(type) {
  const allFields = [
    'field-question',
    'field-answer',
    'field-hint',
    'field-image-url',
    'field-fillin',
    'field-options',
    'field-vocab',
    'field-reading',
    'field-audio',
    'field-video',
    'field-title',
  ];

  const fieldMap = {
    multiplechoice: ['field-question', 'field-options', 'field-hint'],
    fillin:         ['field-question', 'field-fillin', 'field-hint'],
    listening:      ['field-audio', 'field-question', 'field-answer', 'field-hint'],
    vocab:          ['field-vocab', 'field-image-url'],
    reading:        ['field-reading'],
    phrasebook:     ['field-question', 'field-image-url'],
    image:          ['field-image-url', 'field-question', 'field-answer', 'field-hint'],
    video:          ['field-title', 'field-video'],
  };

  const visible = fieldMap[type] || ['field-question', 'field-answer'];

  allFields.forEach(function (id) {
    const el = document.getElementById(id);
    if (!el) return;
    const show = visible.includes(id);
    el.style.display = show ? '' : 'none';
    el.querySelectorAll('input, textarea, select').forEach(function(f) {
      if (show) { f.removeAttribute('disabled'); }
      else       { f.setAttribute('disabled', ''); }
    });
  });
}


// ---- Utility: base64url helpers ----

function base64urlToBuffer(base64url) {
  if (base64url instanceof ArrayBuffer) return base64url;
  if (typeof base64url !== 'string') base64url = base64url.toString();
  const pad = base64url.length % 4;
  const padded = pad ? base64url + '='.repeat(4 - pad) : base64url;
  const binary = atob(padded.replace(/-/g, '+').replace(/_/g, '/'));
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return bytes.buffer;
}

function bufferToBase64url(buffer) {
  const bytes = new Uint8Array(buffer);
  let binary = '';
  bytes.forEach(b => binary += String.fromCharCode(b));
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
}

function credentialToJSON(cred) {
  if (cred instanceof PublicKeyCredential) {
    const resp = cred.response;
    const json = {
      id: cred.id,
      rawId: bufferToBase64url(cred.rawId),
      type: cred.type,
      response: {}
    };
    if (resp.clientDataJSON)    json.response.clientDataJSON    = bufferToBase64url(resp.clientDataJSON);
    if (resp.attestationObject) json.response.attestationObject = bufferToBase64url(resp.attestationObject);
    if (resp.authenticatorData) json.response.authenticatorData = bufferToBase64url(resp.authenticatorData);
    if (resp.signature)         json.response.signature         = bufferToBase64url(resp.signature);
    if (resp.userHandle)        json.response.userHandle        = bufferToBase64url(resp.userHandle);
    return json;
  }
  return cred;
}

// ---- X-system auto-correction ----
// Converts cx→ĉ, gx→ĝ, hx→ĥ, jx→ĵ, sx→ŝ, ux→ŭ (case-preserving) in text inputs.

(function () {
  'use strict';
  const xMap = {
    c: 'ĉ', C: 'Ĉ',
    g: 'ĝ', G: 'Ĝ',
    h: 'ĥ', H: 'Ĥ',
    j: 'ĵ', J: 'Ĵ',
    s: 'ŝ', S: 'Ŝ',
    u: 'ŭ', U: 'Ŭ',
  };

  function applyXSystem(el) {
    const val = el.value;
    const pos = el.selectionStart;
    // Check if the character just before the cursor is 'x' or 'X'
    // and the character before that is a convertible letter.
    if (pos < 2) return;
    const xChar = val[pos - 1];
    if (xChar !== 'x' && xChar !== 'X') return;
    const base = val[pos - 2];
    const replacement = xMap[base];
    if (!replacement) return;
    // Replace the two chars with the Esperanto char.
    el.value = val.slice(0, pos - 2) + replacement + val.slice(pos);
    el.selectionStart = el.selectionEnd = pos - 1;
  }

  document.addEventListener('input', function (evt) {
    const el = evt.target;
    if (el.tagName !== 'INPUT' && el.tagName !== 'TEXTAREA') return;
    // Only apply to fields where the user types Esperanto (not emails, URLs, etc.)
    const type = (el.type || '').toLowerCase();
    if (type === 'email' || type === 'url' || type === 'number') return;
    // Skip fields with data-no-x-system attribute.
    if (el.dataset.noXSystem !== undefined) return;
    applyXSystem(el);
  }, true);
})();

// ---- Theme picker (light / dark / auto), in the profile menu ----
(function () {
  var THEME_KEY = 'eo_theme';
  function getTheme() { try { return localStorage.getItem(THEME_KEY) || 'auto'; } catch (e) { return 'auto'; } }
  function applyTheme(theme) {
    if (theme === 'auto') document.documentElement.removeAttribute('data-theme');
    else document.documentElement.setAttribute('data-theme', theme);
    document.querySelectorAll('[data-theme-set]').forEach(function (b) {
      b.setAttribute('aria-pressed', String(b.dataset.themeSet === theme));
    });
  }
  document.addEventListener('DOMContentLoaded', function () {
    applyTheme(getTheme());
    document.querySelectorAll('[data-theme-set]').forEach(function (b) {
      b.addEventListener('click', function () {
        try { localStorage.setItem(THEME_KEY, b.dataset.themeSet); } catch (e) {}
        applyTheme(b.dataset.themeSet);
      });
    });
    // The header place you are on.
    var path = location.pathname.replace(/\/+$/, '') || '/';
    document.querySelectorAll('[data-loko]').forEach(function (a) {
      var l = a.dataset.loko;
      if (l === path || (l !== '/' && path.indexOf(l + '/') === 0)) a.setAttribute('aria-current', 'page');
    });
    // Close an open menu on a click outside it or on Escape.
    document.addEventListener('click', function (e) {
      document.querySelectorAll('details.menu[open]').forEach(function (d) { if (!d.contains(e.target)) d.open = false; });
    });
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') document.querySelectorAll('details.menu[open]').forEach(function (d) { d.open = false; });
    });
  });
})();

// ---- Language pickers (datalist with code or name search) ----

function _getLangName(datalistId, code) {
  var dl = document.getElementById(datalistId);
  if (!dl) return code;
  var opts = dl.options;
  for (var i = 0; i < opts.length; i++) {
    if (opts[i].value === code) return opts[i].label || code;
  }
  return code;
}

function _langInput(input, datalistId, formId) {
  var dl = document.getElementById(datalistId);
  if (!dl) return;
  var val = input.value.trim().toLowerCase();
  var opts = dl.options;
  for (var i = 0; i < opts.length; i++) {
    var opt = opts[i];
    var label = (opt.label || '').toLowerCase();
    if (opt.value.toLowerCase() === val || label === val) {
      input.dataset.code = opt.value;
      input.value = opt.value;
      document.getElementById(formId).submit();
      return;
    }
  }
}

function langInputFocus(input) {
  // Save current code, clear field so the full list is visible immediately
  var code = input.dataset.code || input.value;
  input.dataset.code = code;
  input.value = '';
}

function langInputBlur(input, datalistId) {
  // Restore human-readable name if unchanged or invalid
  var code = input.dataset.code || input.value;
  input.dataset.code = code;
  input.value = _getLangName(datalistId, code);
}

function vocabLangInput(input) { _langInput(input, 'vocab-lang-list', 'vocab-lang-form'); }
function uiLangInput(input)    { _langInput(input, 'ui-lang-list',    'ui-lang-form');    }

document.addEventListener('DOMContentLoaded', function () {
  var ui = document.getElementById('ui-lang-input');
  if (ui) langInputBlur(ui, 'ui-lang-list');
  var vl = document.getElementById('vocab-lang-input');
  if (vl) langInputBlur(vl, 'vocab-lang-list');
});

// ---- The activity stage ----
// Kontroli stays disabled until there is an answer; after the verdict
// (rezulto.html swapped into #dock) the tiles or gaps get their state with a
// second cue besides colour (a ✓/✗ badge), the dock its tint, and focus goes
// to Daŭrigi. Keys: 1–4 pick a tile or a self-judgement, Space flips a card.
(function () {
  'use strict';
  var SVG = function (n) { return '<svg class="ico" aria-hidden="true"><use href="#i-' + n + '"/></svg>'; };
  var norm = function (s) { return (s || '').trim().toLowerCase(); };

  function answered(form) {
    var fields = form.querySelectorAll('[name="answer"]');
    if (!fields.length) return false;
    if (fields[0].type === 'radio') return !!form.querySelector('[name="answer"]:checked');
    for (var i = 0; i < fields.length; i++) if (!fields[i].value.trim()) return false;
    return true;
  }
  function syncCheck() {
    var form = document.getElementById('answer-form');
    var btn = document.querySelector('[data-check]');
    if (form && btn) btn.disabled = !answered(form);
  }
  document.addEventListener('input', syncCheck);
  document.addEventListener('change', syncCheck);
  document.addEventListener('DOMContentLoaded', syncCheck);

  function badge(kind) { var b = document.createElement('span'); b.className = 'badge'; b.innerHTML = SVG(kind === 'right' ? 'check' : 'x'); return b; }
  function gapMark(kind) { var b = document.createElement('span'); b.className = 'gap-mark ' + kind; b.innerHTML = SVG(kind === 'right' ? 'check' : 'x'); return b; }

  function markAnswer(v) {
    var form = document.getElementById('answer-form');
    if (!form) return;
    var correct = v.dataset.correct || '';
    var verdict = v.dataset.verdict;
    var radios = form.querySelectorAll('input[type="radio"][name="answer"]');
    if (radios.length) {
      radios.forEach(function (r) {
        var tile = r.closest('.tile');
        r.disabled = true;
        if (norm(r.value) === norm(correct)) { tile.dataset.s = 'right'; tile.appendChild(badge('right')); }
        else if (r.checked) { tile.dataset.s = 'wrong'; tile.appendChild(badge('wrong')); }
        else { tile.dataset.s = 'done'; }
      });
      var fs = form.querySelector('.tiles'); if (fs) fs.dataset.answered = '';
      return;
    }
    var fields = form.querySelectorAll('input[name="answer"]');
    var parts = correct.split(' / ');
    fields.forEach(function (f, i) {
      var ok = verdict === 'right' || (fields.length > 1 && parts.length && norm(f.value) === norm(parts[i % parts.length]));
      f.readOnly = true;
      f.classList.add(ok ? 'is-right' : 'is-wrong');
      f.insertAdjacentElement('afterend', gapMark(ok ? 'right' : 'wrong'));
    });
  }

  document.addEventListener('htmx:afterSwap', function (evt) {
    var dock = evt.detail.target;
    if (!dock || dock.id !== 'dock') return;
    var v = dock.querySelector('[data-verdict]');
    if (!v) return;
    dock.classList.remove('right', 'wrong');
    if (v.dataset.verdict === 'right' || v.dataset.verdict === 'wrong') dock.classList.add(v.dataset.verdict);
    markAnswer(v);
    var post = document.getElementById('post-answer');
    if (post) post.classList.remove('post-answer-hidden');
    var go = dock.querySelector('[data-continue]');
    if (go) go.focus({ preventScroll: true });
  });

  function flip() {
    var back = document.querySelector('[data-back]');
    var front = document.querySelector('#dock [data-front]');
    var judge = document.querySelector('#dock [data-judge]');
    if (!front || front.hidden) return;
    if (back) back.hidden = false;
    front.hidden = true;
    if (judge) { judge.hidden = false; var first = judge.querySelector('button'); if (first) first.focus({ preventScroll: true }); }
  }
  document.addEventListener('click', function (e) {
    if (e.target.closest('[data-flip]')) flip();
    var play = e.target.closest('[data-play]');
    if (play) {
      var a = document.getElementById('sono');
      if (a) { a.playbackRate = parseFloat(play.dataset.play) || 1; a.currentTime = 0; a.play(); }
    }
    var reveal = e.target.closest('[data-reveal-after]');
    if (reveal) { var post = document.getElementById('post-answer'); if (post) post.classList.remove('post-answer-hidden'); }
  });

  document.addEventListener('keydown', function (e) {
    if (!document.body.classList.contains('on-stage') || e.ctrlKey || e.metaKey || e.altKey) return;
    var t = e.target;
    if (t && (t.tagName === 'INPUT' && t.type !== 'radio' || t.tagName === 'TEXTAREA')) return;
    if (e.key === ' ' && document.querySelector('#dock [data-front]:not([hidden])')) { e.preventDefault(); flip(); return; }
    if (/^[1-9]$/.test(e.key)) {
      var judge = document.querySelector('#dock [data-judge]:not([hidden]) [data-key="' + e.key + '"]');
      if (judge) { judge.click(); return; }
      var radio = document.querySelector('#answer-form input[type="radio"][data-key="' + e.key + '"]:not(:disabled)');
      if (radio) { radio.checked = true; radio.focus(); syncCheck(); }
    }
  });
})();

// ---- Account nudge banner ----
// Server renders it (hidden) for users with progress but no username/passkey;
// we reveal it unless snoozed, and never on the profile page itself.
const NUDGE_SNOOZE_KEY = 'eo_nudge_until';

document.addEventListener('DOMContentLoaded', function () {
  const nudge = document.getElementById('account-nudge');
  if (!nudge) return;
  if (window.location.pathname === '/enskribi') return;
  const until = parseInt(localStorage.getItem(NUDGE_SNOOZE_KEY) || '0', 10);
  if (Date.now() < until) return;
  nudge.hidden = false;
});

function dismissAccountNudge() {
  const nudge = document.getElementById('account-nudge');
  if (nudge) nudge.hidden = true;
  // Snooze for 7 days.
  localStorage.setItem(NUDGE_SNOOZE_KEY, String(Date.now() + 7 * 24 * 60 * 60 * 1000));
}

// ---- Support corner: copy helper ----
function copySupport(id, btn) {
  const el = document.getElementById(id);
  if (!el) return;
  // data-copy wins over the (possibly truncated) display text.
  const text = (el.dataset.copy || el.textContent).trim();
  navigator.clipboard.writeText(text).then(function () {
    const old = btn.innerHTML;
    btn.innerHTML = '<svg class="ico" aria-hidden="true"><use href="#i-check"/></svg>';
    setTimeout(function () { btn.innerHTML = old; }, 1500);
  });
}
