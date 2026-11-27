/*
ripetoj.js — spaced repetition (Ripetoj) without the page: which new card
comes next, what is due, merging two saved states, and rebuilding a state
from the server's trace. Pure functions only (tested by
tools/test-ripetoj.mjs); the scheduling itself is FSRS (fsrs.js), storage
and the page side are in eo-ripetoj.js.

Copied from angla.org (angla-kurso, wiki/plugins/angla-kurso/ripetoj.js)
unchanged but for the wrapper (window.EoRipetoj in the browser), without its
vocabulary-list parsing (this site's words are items, one card each), and
with the rebuild from the trace added at the end.

Cards are the site's items, by slug: /ripetoj-api/kartoj gives them as
chains, one per series in authored order, each card [slug, rating].

New cards: at most a daily number; in each series the authored order is a
weak chain (a card is open once one of the two before it has been seen); of
the open cards the three nearest the learner's rating make the window, and
one of them is taken by a seed, so the order is shuffled a little.
*/
(function (exports) {
  "use strict";

  var WINDOW = 3, MIX = 3, AHEAD = 20 * 60000, ROLLOVER_H = 4;

  // A card in a chain is open when it is the first, or one of the two before it was seen.
  function open(chain, i, seen) {
    return i === 0 || seen(chain[i - 1].id) || (i >= 2 && seen(chain[i - 2].id));
  }
  function hash(s) {
    var h = 2166136261;
    s = String(s);
    for (var i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 16777619) >>> 0; }
    return h;
  }
  // The open, unseen cards nearest the rating (at most WINDOW), nearest first.
  function window_(chains, seen, rating, skip) {
    var c = [], order = 0;
    chains.forEach(function (chain) {
      chain.forEach(function (card, i) {
        var o = order++;
        if (seen(card.id) || (skip && skip(card)) || !open(chain, i, seen)) return;
        c.push({ card: card, o: o, d: card.r === null || card.r === undefined ? Infinity : Math.abs(card.r - rating) });
      });
    });
    c.sort(function (a, b) { return a.d - b.d || a.o - b.o; });
    return c.slice(0, WINDOW).map(function (x) { return x.card; });
  }
  function pickNew(chains, seen, rating, seed, skip) {
    var w = window_(chains, seen, rating, skip);
    return w.length ? w[hash(seed) % w.length] : null;
  }

  // Days roll over at 04:00 local time, as in Anki.
  function dayStart(now) {
    var d = new Date(now - ROLLOVER_H * 3600000);
    d.setHours(ROLLOVER_H, 0, 0, 0);
    return d.getTime();
  }
  function dayEnd(now) {
    var d = new Date(dayStart(now));
    d.setDate(d.getDate() + 1);
    d.setHours(ROLLOVER_H, 0, 0, 0);
    return d.getTime();
  }
  function dayKey(now) {
    var d = new Date(dayStart(now));
    return d.getFullYear() + "-" + ("0" + (d.getMonth() + 1)).slice(-2) + "-" + ("0" + d.getDate()).slice(-2);
  }

  // A stored state, checked: { c: {id: card}, k: {day, n} } (n new cards begun that day).
  function clean(s) {
    var out = { c: {}, k: { day: "", n: 0, extra: 0 } };
    if (!s || typeof s !== "object") return out;
    var c = s.c && typeof s.c === "object" ? s.c : {};
    Object.keys(c).forEach(function (id) {
      var x = c[id];
      if (x && isFinite(x.due) && isFinite(x.s) && x.s > 0 && isFinite(x.d) && (x.t === 1 || x.t === 2 || x.t === 3)) out.c[id] = x;
    });
    if (s.k && typeof s.k.day === "string") out.k = { day: s.k.day, n: Math.max(0, parseInt(s.k.n, 10) || 0), extra: Math.max(0, parseInt(s.k.extra, 10) || 0) };
    return out;
  }
  // How many new cards may still begin today.
  function newLeft(state, now, limit) {
    var k = state.k.day === dayKey(now) ? state.k : { n: 0, extra: 0 };
    return Math.max(0, limit + k.extra - k.n);
  }
  function countNew(state, now) {
    var today = dayKey(now);
    if (state.k.day !== today) state.k = { day: today, n: 0, extra: 0 };
    state.k.n += 1;
  }
  function moreNew(state, now, n) {
    var today = dayKey(now);
    if (state.k.day !== today) state.k = { day: today, n: 0, extra: 0 };
    state.k.extra += n;
  }

  // What is due among the known cards (ids in `ids`): learning cards (state 1
  // or 3) and review cards due before the day ends.
  function due(state, ids, now) {
    var end = dayEnd(now), learn = [], rev = [];
    Object.keys(state.c).forEach(function (id) {
      if (ids && !ids[id]) return;
      var c = state.c[id];
      if (c.t === 2) { if (c.due < end) rev.push(id); } else if (c.due < end) learn.push(id);
    });
    var by = function (a, b) { return state.c[a].due - state.c[b].due || (a < b ? -1 : 1); };
    learn.sort(by);
    rev.sort(by);
    return { learn: learn, rev: rev };
  }
  // The next card of a session: "learn:id", "rev:id", "new" or "" (done).
  // `sinceNew` counts the cards shown since the last new one.
  function next(state, ids, now, canNew, sinceNew, skipId) {
    var d = due(state, ids, now);
    var notSkip = function (id) { return id !== skipId; };
    var ready = d.learn.filter(function (id) { return state.c[id].due <= now && notSkip(id); });
    if (ready.length) return "learn:" + ready[0];
    var rev = d.rev.filter(notSkip);
    if (canNew && (!rev.length || sinceNew >= MIX)) return "new";
    if (rev.length) return "rev:" + rev[0];
    var ahead = d.learn.filter(function (id) { return state.c[id].due <= now + AHEAD && notSkip(id); });
    if (ahead.length) return "learn:" + ahead[0];
    if (skipId && state.c[skipId] && state.c[skipId].due <= now + AHEAD) return (state.c[skipId].t === 2 ? "rev:" : "learn:") + skipId;
    return "";
  }

  // Two saved states as one: per card the one reviewed last; the day count of `a`.
  function merge(a, b) {
    a = clean(a);
    b = clean(b);
    var out = { c: {}, k: a.k };
    Object.keys(a.c).forEach(function (id) { out.c[id] = a.c[id]; });
    Object.keys(b.c).forEach(function (id) {
      if (!out.c[id] || (b.c[id].l || 0) > (out.c[id].l || 0)) out.c[id] = b.c[id];
    });
    return out;
  }

  // A short Esperanto label for a wait in ms: "1 min", "10 min", "3 h", "4 t", "2 mon", "1,5 j".
  function wait(ms) {
    var m = ms / 60000;
    if (m < 60) return Math.max(1, Math.round(m)) + " min";
    var h = m / 60;
    if (h < 24) return Math.round(h) + " h";
    var d = h / 24;
    if (d < 30.5) return Math.round(d) + " t";
    if (d < 365) return Math.round(d / 30.4) + " mon";
    return String(Math.round(d / 36.5) / 10).replace(".", ",") + " j";
  }


  // ---- esperanto-kurso.net: rebuilding from the server's trace

  // The answers a rebuilt card replays at most: the trace counts every
  // answer, a repeat on the same day too, so a long count is no proof of as
  // many spaced reviews. Four Good answers make a 46-day interval (py-fsrs:
  // 0, 2, 11, 46 days); a card known better grows again from there.
  var REPLAY = 4;

  // A reasonable card from the server's rough trace of one item (its
  // answers, the day of the last, the last grade): a synthetic history
  // replayed through FSRS (F, fsrs.js), each answer Good at the time the
  // one before made it due and the last one the grade given, then moved so
  // the last answer falls at noon (UTC) of its day. Never later than now.
  function rebuildCard(F, day, count, grade, now) {
    var last = Math.min(day * F.DAY + F.DAY / 2, now);
    var n = Math.max(1, Math.min(count | 0, REPLAY));
    var g = grade >= 1 && grade <= 4 ? grade : F.GOOD;
    var card = null, t = last; // any real moment: fsrs.js reads a last review of 0 as none
    for (var i = 0; i < n - 1; i++) {
      card = F.review(card, F.GOOD, t);
      t = card.due;
    }
    card = F.review(card, g, t);
    var shift = last - card.l;
    card.l += shift;
    card.due += shift;
    card.r = count | 0 || 1;
    return card;
  }
  // The trace's entries ([slug, unix day, answers, last grade]) as cards
  // the state lacks; the cards it has stay as they are. → how many added.
  function addMissing(F, state, entries, now) {
    var added = 0;
    (entries || []).forEach(function (e) {
      if (!e || typeof e[0] !== "string" || state.c[e[0]]) return;
      state.c[e[0]] = rebuildCard(F, +e[1], +e[2], +e[3], now);
      added++;
    });
    return added;
  }

  exports.WINDOW = WINDOW;
  exports.open = open;
  exports.window = window_;
  exports.pickNew = pickNew;
  exports.hash = hash;
  exports.dayStart = dayStart;
  exports.dayEnd = dayEnd;
  exports.dayKey = dayKey;
  exports.clean = clean;
  exports.newLeft = newLeft;
  exports.countNew = countNew;
  exports.moreNew = moreNew;
  exports.due = due;
  exports.next = next;
  exports.merge = merge;
  exports.wait = wait;
  exports.REPLAY = REPLAY;
  exports.rebuildCard = rebuildCard;
  exports.addMissing = addMissing;
})(typeof exports !== "undefined" ? exports : (window.EoRipetoj = {}));
