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
  if (getToken()) return; // already have a session
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

// ---- Exercise feedback: sounds + level-up celebration ----

(function () {
  'use strict';

  // Web Audio beeps — no external files needed.
  function beep(freq, duration, type, vol) {
    try {
      const ctx = new (window.AudioContext || window.webkitAudioContext)();
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.type = type || 'sine';
      osc.frequency.setValueAtTime(freq, ctx.currentTime);
      gain.gain.setValueAtTime(vol || 0.15, ctx.currentTime);
      gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + duration);
      osc.start(ctx.currentTime);
      osc.stop(ctx.currentTime + duration);
    } catch (e) { /* AudioContext may not be available */ }
  }

  function playCorrect() {
    beep(660, 0.12, 'sine', 0.12);
    setTimeout(() => beep(880, 0.15, 'sine', 0.10), 100);
  }

  function playIncorrect() {
    beep(300, 0.2, 'sawtooth', 0.10);
  }

  // Minimal confetti burst for level-up (no external lib).
  function launchConfetti() {
    const canvas = document.createElement('canvas');
    canvas.style.cssText = 'position:fixed;top:0;left:0;width:100%;height:100%;pointer-events:none;z-index:9999';
    document.body.appendChild(canvas);
    const ctx = canvas.getContext('2d');
    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;

    const colors = ['#009900','#f5c400','#fff','#00c853','#ffd600'];
    const particles = Array.from({length: 80}, () => ({
      x: Math.random() * canvas.width,
      y: Math.random() * -100,
      r: Math.random() * 6 + 3,
      d: Math.random() * 3 + 1,
      color: colors[Math.floor(Math.random() * colors.length)],
      tilt: Math.random() * 10 - 5,
      tiltAngle: 0,
      tiltSpeed: Math.random() * 0.05 + 0.02,
    }));

    let frame = 0;
    function draw() {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      particles.forEach(p => {
        p.tiltAngle += p.tiltSpeed;
        p.y += p.d + 1;
        p.tilt = Math.sin(p.tiltAngle) * 12;
        ctx.beginPath();
        ctx.lineWidth = p.r;
        ctx.strokeStyle = p.color;
        ctx.moveTo(p.x + p.tilt + p.r / 2, p.y);
        ctx.lineTo(p.x + p.tilt, p.y + p.tilt + p.r / 2);
        ctx.stroke();
      });
      frame++;
      if (frame < 120) requestAnimationFrame(draw);
      else canvas.remove();
    }
    draw();
  }

  // After HTMX swaps in a result, play sound and check for level-up.
  document.addEventListener('htmx:afterSwap', function (evt) {
    const target = evt.detail.target;
    if (!target || target.id !== 'rezulto') return;

    const result = target.querySelector('.rezulto');
    if (!result) return;

    if (result.classList.contains('correct')) {
      playCorrect();
    } else {
      playIncorrect();
    }

    if (target.querySelector('.level-up-banner')) {
      launchConfetti();
    }
  });

})();

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

// ---- Theme picker (light / dark / auto) ----
(function () {
  var THEME_KEY = 'eo_theme';
  var ICONS = { light: '☀️', dark: '🌙', auto: '🔆' };

  function getTheme() { return localStorage.getItem(THEME_KEY) || 'auto'; }

  function applyTheme(theme) {
    if (theme === 'auto') {
      document.documentElement.removeAttribute('data-theme');
    } else {
      document.documentElement.setAttribute('data-theme', theme);
    }
    var btn = document.getElementById('theme-toggle');
    if (btn) btn.textContent = ICONS[theme] || ICONS.auto;
  }

  // Sync icon once DOM is ready (theme attribute already applied by inline script).
  document.addEventListener('DOMContentLoaded', function () { applyTheme(getTheme()); });

  window.cycleTheme = function () {
    var cur = getTheme();
    var next = cur === 'auto' ? 'light' : cur === 'light' ? 'dark' : 'auto';
    localStorage.setItem(THEME_KEY, next);
    applyTheme(next);
  };
})();

// ---- Focus mode ----
(function () {
  var FM_KEY = 'eo_focus_mode';
  function applyFocusMode(on) {
    document.body.classList.toggle('focus-mode', on);
  }
  // Restore on load.
  if (localStorage.getItem(FM_KEY) === '1') applyFocusMode(true);
  window.toggleFocusMode = function () {
    var on = !document.body.classList.contains('focus-mode');
    applyFocusMode(on);
    localStorage.setItem(FM_KEY, on ? '1' : '0');
  };
})();

// ---- Language pickers (datalist with code or name search) ----
function _langInput(input, datalistId, formId) {
  var dl = document.getElementById(datalistId);
  if (!dl) return;
  var val = input.value.trim().toLowerCase();
  var options = dl.options;
  for (var i = 0; i < options.length; i++) {
    var opt = options[i];
    if (opt.value.toLowerCase() === val || opt.label.toLowerCase() === val) {
      input.value = opt.value;
      document.getElementById(formId).submit();
      return;
    }
  }
}
function vocabLangInput(input) { _langInput(input, 'vocab-lang-list', 'vocab-lang-form'); }
function uiLangInput(input)    { _langInput(input, 'ui-lang-list',    'ui-lang-form');    }

// ---- Reveal community sections after answer ----
document.addEventListener('htmx:afterSwap', function (evt) {
  if (evt.detail.target && evt.detail.target.id === 'rezulto') {
    const postAnswer = document.getElementById('post-answer');
    if (postAnswer) {
      postAnswer.classList.remove('post-answer-hidden');
      postAnswer.classList.add('post-answer-visible');
    }
  }
});
