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

  // On load, pick up the token from the cookie if localStorage is empty.
  // The server sets a "token" cookie; read it so HTMX requests also carry it.
  document.addEventListener('DOMContentLoaded', function () {
    if (!getToken()) {
      const match = document.cookie.match(/(?:^|;\s*)token=([^;]+)/);
      if (match) setToken(decodeURIComponent(match[1]));
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

    status('Paŝklavo sukcese registrita!');
  } catch (err) {
    console.error('Passkey registration failed:', err);
    const statusEl = document.getElementById('passkey-status');
    if (statusEl) statusEl.textContent = 'Eraro: ' + err.message;
  }
}


// ---- WebAuthn Login ----

async function loginPasskey() {
  const statusEl = document.getElementById('passkey-status');
  function status(msg) { if (statusEl) statusEl.textContent = msg; }

  try {
    status('Komencas ensaluton…');

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

    // Step 2: call browser WebAuthn API.
    status('Bonvolu sekvi la instrukciojn de via aparato…');
    const assertion = await navigator.credentials.get(options);

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
      status('Ensaluto sukcesa! Reŝarĝas…');
      setTimeout(() => window.location.href = '/', 1000);
    } else {
      status('Ensaluto sukcesa!');
    }
  } catch (err) {
    console.error('Passkey login failed:', err);
    const statusEl = document.getElementById('passkey-status');
    if (statusEl) statusEl.textContent = 'Eraro: ' + err.message;
  }
}


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


// ---- Admin: Show/Hide Content Fields ----

function showContentFields(type) {
  // All toggleable field groups.
  const allFields = [
    'field-question',
    'field-answer',
    'field-options',
    'field-vocab',
    'field-reading',
    'field-audio',
  ];

  // Which fields are visible for each type.
  const fieldMap = {
    multiplechoice: ['field-question', 'field-options'],
    fillin:         ['field-question', 'field-answer'],
    listening:      ['field-question', 'field-answer', 'field-audio'],
    vocab:          ['field-vocab'],
    reading:        ['field-reading', 'field-question', 'field-answer'],
    phrasebook:     ['field-question', 'field-answer'],
    image:          ['field-question', 'field-answer'],
  };

  const visible = fieldMap[type] || ['field-question', 'field-answer'];

  allFields.forEach(function (id) {
    const el = document.getElementById(id);
    if (!el) return;
    el.style.display = visible.includes(id) ? '' : 'none';
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
