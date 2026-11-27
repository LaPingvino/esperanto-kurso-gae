/*
eo-ripetoj.js — spaced repetition (Ripetoj) in the browser, as on angla.org
(its ripetoj-starto.js and ripetoj-vido.js): the pure parts are in
ripetoj.js, the scheduler in fsrs.js, the server's side in
internal/handler/ripetoj.go.

  state    localStorage "eo-ripetoj": {c: {slug: card}, k: {day, n, extra},
           u: owner}. Only cards seen. "u" names the account (a hash of
           it, <meta name="eo-ripetoj">); a state of another account is not
           this one's (a login on this device): the two are merged.
  grading  every verdict on the site (rezulto.html, data-srs/data-grade)
           grades its card: a right answer is Good, a wrong one Again; a
           self-judgement ne sciis / malfacile / bone / facile is
           Again / Hard / Good / Easy. The server writes the same grade into
           its trace while saving the answer.
  rebuild  when the server's trace holds items the state lacks (a new
           browser, another device), /ripetoj/spuro gives them and each is
           rebuilt by FSRS from its rough record (ripetoj.js rebuildCard).
  header   the count beside "Ripetoj": due cards and new ones still allowed
           today, from the state alone (no request).
  session  /ripetoj picks the next card (due learning, due reviews, every
           third a new one) and shows it on its exercise page (?ripeto=1);
           Daŭrigi leads to the next. The deck is /ripetoj-api/kartoj.
  backup   Elŝuti / Restarigi save and merge the state as a file;
           Forgesi clears it here and the trace on the server.
*/
(function () {
  "use strict";
  var F = window.EoFSRS, R = window.EoRipetoj;
  if (!F || !R) return;
  var KEY = "eo-ripetoj", SESSION = "eo-ripetoj-sesio", TRIED = "eo-ripetoj-provis";
  var NEW_PER_DAY = 10, MORE = 5;

  function ls() { try { return window.localStorage; } catch (e) { return null; } }
  function ss() { try { return window.sessionStorage; } catch (e) { return null; } }
  function readJSON(store, key) {
    try { var raw = store && store.getItem(key); return raw ? JSON.parse(raw) : null; } catch (e) { return null; }
  }
  function writeJSON(store, key, v) {
    try { if (store) { if (v === null) store.removeItem(key); else store.setItem(key, JSON.stringify(v)); } } catch (e) { /* full or blocked */ }
  }

  // <meta name="eo-ripetoj" content="owner traceItems rating">
  var meta = (function () {
    var m = document.querySelector('meta[name="eo-ripetoj"]');
    var p = m ? (m.getAttribute("content") || "").split(" ") : [];
    return { owner: p[0] || "", trace: parseInt(p[1], 10) || 0, rating: parseInt(p[2], 10) || 900 };
  })();

  var raw = readJSON(ls(), KEY);
  var state = R.clean(raw);
  var owner = raw && typeof raw.u === "string" ? raw.u : "";
  // Another account's state (a login on this device): the same person's
  // answers, so it stays, merged with this account's trace below.
  if (meta.owner && owner !== meta.owner) owner = meta.owner;

  function store() {
    var empty = !Object.keys(state.c).length && !state.k.n && !state.k.extra;
    writeJSON(ls(), KEY, empty ? null : { c: state.c, k: state.k, u: owner });
  }
  function token() { return window.getEoToken ? window.getEoToken() : null; }
  function headers() { var h = {}; var t = token(); if (t) h["X-Auth-Token"] = t; return h; }

  // ---- counts
  function counts(now, deck) {
    var ids = deck ? deck.ids : null, d = R.due(state, ids, now);
    var seen = 0;
    Object.keys(state.c).forEach(function (id) { if (!ids || ids[id]) seen++; });
    var nov = R.newLeft(state, now, NEW_PER_DAY);
    if (deck) nov = Math.min(nov, deck.size - seen);
    return { nun: d.learn.length + d.rev.length + nov, ripetoj: d.learn.length + d.rev.length, novaj: nov, konataj: seen };
  }
  function showCount(deck) {
    var c = counts(Date.now(), deck);
    document.querySelectorAll("[data-ripetoj-nombro]").forEach(function (el) {
      el.textContent = c.nun ? String(c.nun) : "";
      el.hidden = !c.nun;
    });
    return c;
  }

  // ---- grading
  function respondi(id, grade, now) {
    now = now || Date.now();
    var old = state.c[id] || null;
    var h = R.hash(id + ":" + (old ? old.r : 0));
    state.c[id] = F.review(old, grade, now, { fuzz: function () { return (h % 100000) / 100000; } });
    store();
    return state.c[id];
  }

  // ---- the deck
  var deckPromise = null;
  function loadDeck() {
    if (!deckPromise) {
      deckPromise = fetch("/ripetoj-api/kartoj").then(function (r) {
        if (!r.ok) throw new Error("kartoj " + r.status);
        return r.json();
      }).then(function (j) {
        var chains = [], ids = {}, size = 0;
        (j.c || []).forEach(function (ch) {
          var chain = [];
          ch.forEach(function (x) { if (!ids[x[0]]) { ids[x[0]] = true; size++; chain.push({ id: x[0], r: x[1] }); } });
          if (chain.length) chains.push(chain);
        });
        return { chains: chains, ids: ids, size: size };
      });
      deckPromise.catch(function () { deckPromise = null; });
    }
    return deckPromise;
  }

  // ---- the session
  function session() { return readJSON(ss(), SESSION) || { sinceNew: 0, lastId: null, done: 0, nova: null }; }
  function saveSession(s) { writeJSON(ss(), SESSION, s); }

  // The next card: {id, nova} or null (done for today).
  function sekva(deck, now, s) {
    var canNew = R.newLeft(state, now, NEW_PER_DAY) > 0;
    var n = R.next(state, deck.ids, now, canNew, s.sinceNew, s.lastId);
    if (n === "new") {
      var seed = R.dayKey(now) + ":" + (state.k.day === R.dayKey(now) ? state.k.n : 0);
      var card = R.pickNew(deck.chains, function (id) { return !!state.c[id]; }, meta.rating, seed);
      if (card) return { id: card.id, nova: true };
      n = R.next(state, deck.ids, now, false, s.sinceNew, s.lastId);
    }
    if (!n) return null;
    return { id: n.slice(n.indexOf(":") + 1), nova: false };
  }
  function cardHref(n) { return "/ekzerco/" + encodeURIComponent(n.id) + "?ripeto=1"; }
  function go(deck, replace) {
    var s = session(), n = sekva(deck, Date.now(), s);
    if (!n) return false;
    s.nova = n.nova ? n.id : null;
    saveSession(s);
    if (replace) location.replace(cardHref(n)); else location.href = cardHref(n);
    return true;
  }
  var inSession = /[?&]ripeto=1(&|$)/.test(location.search);

  // A verdict swapped into the stage's dock: grade its card; in a session,
  // count it and point Daŭrigi at the next card.
  document.addEventListener("htmx:afterSwap", function (evt) {
    var dock = evt.detail && evt.detail.target;
    if (!dock || dock.id !== "dock") return;
    var v = dock.querySelector("[data-srs]");
    if (!v) return;
    var id = v.getAttribute("data-srs"), g = parseInt(v.getAttribute("data-grade"), 10);
    if (!id || !(g >= 1 && g <= 4)) return;
    var wasNew = !state.c[id];
    respondi(id, g);
    if (inSession) {
      var s = session();
      if (wasNew && s.nova === id) { R.countNew(state, Date.now()); store(); s.sinceNew = 0; } else s.sinceNew++;
      s.done++;
      s.lastId = id;
      s.nova = null;
      saveSession(s);
      var cont = dock.querySelector("[data-continue]");
      loadDeck().then(function (deck) {
        showCount(deck);
        if (!cont) return;
        var n = sekva(deck, Date.now(), s);
        if (!n) { cont.setAttribute("href", "/ripetoj?plu=1"); return; }
        cont.setAttribute("href", cardHref(n));
        cont.addEventListener("click", function () { var t = session(); t.nova = n.nova ? n.id : null; saveSession(t); });
      }).catch(function () {});
    } else showCount(null);
  });

  // ---- rebuilding from the server's trace
  function rebuild() {
    if (!meta.owner || !meta.trace || Object.keys(state.c).length >= meta.trace) return Promise.resolve(0);
    var tried = meta.owner + ":" + meta.trace;
    if ((ss() && readJSON(ss(), TRIED)) === tried) return Promise.resolve(0);
    writeJSON(ss(), TRIED, tried);
    return fetch("/ripetoj/spuro", { headers: headers(), credentials: "same-origin" }).then(function (r) {
      return r.ok ? r.json() : null;
    }).then(function (j) {
      if (!j || j.o !== meta.owner) return 0;
      var n = R.addMissing(F, state, j.e, Date.now());
      if (n) store();
      return n;
    }).catch(function () { return 0; });
  }

  // ---- backup
  function eksporti() {
    return JSON.stringify({ "esperanto-kurso.net": 1, dato: new Date().toISOString(), ripetoj: { c: state.c, k: state.k } });
  }
  function importi(text) {
    var o = JSON.parse(text);
    if (!o || o["esperanto-kurso.net"] !== 1) throw new Error("ne kopio");
    var merged = R.merge(state, o.ripetoj);
    var changed = Object.keys(merged.c).filter(function (id) { return merged.c[id] !== state.c[id]; }).length;
    state = merged;
    store();
    return changed;
  }
  function forgesi() {
    state = R.clean(null);
    store();
    writeJSON(ss(), SESSION, null);
    return fetch("/ripetoj/forgesi", { method: "POST", headers: headers(), credentials: "same-origin" });
  }

  // ---- the page /ripetoj
  function page(root) {
    var $ = function (k) { return root.querySelector('[data-r="' + k + '"]'); };
    var note = $("noto");
    function render(deck) {
      var c = showCount(deck), s = session();
      $("ripetoj").textContent = String(c.ripetoj);
      $("novaj").textContent = String(c.novaj);
      $("konataj").textContent = String(c.konataj);
      var done = !c.nun;
      $("komenci").hidden = done;
      $("fino").hidden = !done;
      $("fino-teksto").textContent = s.done ? "Bone farite!" : "Nenio por hodiaŭ.";
      $("pli").hidden = !done || c.konataj >= deck.size;
      root.removeAttribute("aria-busy");
    }
    loadDeck().then(function (deck) {
      if (root.hasAttribute("data-plu") && go(deck, true)) return;
      render(deck);
      $("komenci").addEventListener("click", function () {
        var s = session();
        if (!root.hasAttribute("data-plu")) { s.done = 0; s.sinceNew = 0; s.lastId = null; saveSession(s); }
        if (!go(deck, false)) render(deck);
      });
      $("pli").addEventListener("click", function () { R.moreNew(state, Date.now(), MORE); store(); go(deck, false) || render(deck); });
      $("elsxuti").addEventListener("click", function () {
        try {
          var a = document.createElement("a");
          a.href = URL.createObjectURL(new Blob([eksporti()], { type: "application/json" }));
          a.download = "esperanto-kurso-ripetoj-" + new Date().toISOString().slice(0, 10) + ".json";
          document.body.appendChild(a);
          a.click();
          document.body.removeChild(a);
          setTimeout(function () { URL.revokeObjectURL(a.href); }, 1000);
          note.textContent = "Konservu la dosieron.";
        } catch (e) { note.textContent = "Ne eblis elŝuti."; }
      });
      var input = $("dosiero");
      $("restarigi").addEventListener("click", function () { input.click(); });
      input.addEventListener("change", function () {
        var f = input.files && input.files[0];
        if (!f) return;
        var rd = new FileReader();
        rd.onload = function () {
          try { note.textContent = "Restarigita: " + importi(String(rd.result)) + " kartoj."; render(deck); }
          catch (e) { note.textContent = "Ĉi tiu dosiero ne estas kopio de Esperanto-kurso."; }
          input.value = "";
        };
        rd.readAsText(f);
      });
      $("forgesi").addEventListener("click", function () {
        if (!confirm("Ĉu forgesi ĉiujn ripetojn? Tion oni ne povas malfari.")) return;
        forgesi().then(function () { note.textContent = "Forgesita."; render(deck); }, function () { note.textContent = "Forgesita ĉi tie; la servilo ne respondis."; render(deck); });
      });
    }).catch(function () { note.textContent = "La kartoj ne ŝargiĝis. Provu denove poste."; root.removeAttribute("aria-busy"); });
  }

  function start() {
    showCount(null);
    rebuild().then(function (n) {
      if (n) showCount(null);
      var root = document.getElementById("ripetoj");
      if (root) page(root);
    });
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start); else start();
})();
