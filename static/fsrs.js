/*
fsrs.js — FSRS-6, the Free Spaced Repetition Scheduler (the algorithm Anki
uses): a port of the reference implementation py-fsrs 6.3.3
(fsrs/scheduler.py, github.com/open-spaced-repetition/py-fsrs), with its 21
default parameters, desired retention 0.9, learning steps 1 min and 10 min,
one relearning step of 10 min, a maximum interval of 36500 days and its fuzz
ranges. Pure functions only; time is passed in (ms since the epoch). Tested
against py-fsrs's own expected values by tools/test-fsrs.mjs.

Copied from angla.org (angla-kurso, wiki/plugins/angla-kurso/fsrs.js)
unchanged but for the wrapper: in the browser it is window.EoFSRS, under node
(the test) the `exports` it is given.

A card is a plain object, or null when new:
  { t: state (1 learning, 2 review, 3 relearning), p: step (or null),
    s: stability, d: difficulty, due: ms, l: last review ms, r: reviews, f: lapses }
Grades: 1 Again (Ne sciis), 2 Hard (Malfacile), 3 Good (Bone), 4 Easy (Facile).
*/
(function (exports) {
  "use strict";

  var PARAMS = [0.212, 1.2931, 2.3065, 8.2956, 6.4133, 0.8334, 3.0194, 0.001, 1.8722, 0.1666, 0.796,
    1.4835, 0.0614, 0.2629, 1.6483, 0.6014, 1.8729, 0.5425, 0.0912, 0.0658, 0.1542];
  var MIN = 60000, DAY = 86400000;
  var DEFAULTS = { w: PARAMS, retention: 0.9, learning: [1 * MIN, 10 * MIN], relearning: [10 * MIN], maxInterval: 36500, fuzz: null };
  var STABILITY_MIN = 0.001, D_MIN = 1, D_MAX = 10;
  var LEARNING = 1, REVIEW = 2, RELEARNING = 3;
  var AGAIN = 1, HARD = 2, GOOD = 3, EASY = 4;
  var FUZZ = [{ start: 2.5, end: 7, factor: 0.15 }, { start: 7, end: 20, factor: 0.1 }, { start: 20, end: Infinity, factor: 0.05 }];

  function opts(o) {
    var out = {};
    for (var k in DEFAULTS) out[k] = o && o[k] !== undefined ? o[k] : DEFAULTS[k];
    out.decay = -out.w[20];
    out.factor = Math.pow(0.9, 1 / out.decay) - 1;
    return out;
  }
  // Python's round(): halves to even.
  function pyRound(x) {
    var f = Math.floor(x), diff = x - f;
    if (diff > 0.5) return f + 1;
    if (diff < 0.5) return f;
    return f % 2 === 0 ? f : f + 1;
  }
  function clampD(d) { return Math.min(Math.max(d, D_MIN), D_MAX); }
  function clampS(s) { return Math.max(s, STABILITY_MIN); }
  // Whole days between two moments, as timedelta.days (floored).
  function days(from, to) { return Math.floor((to - from) / DAY); }

  function initialStability(o, g) { return clampS(o.w[g - 1]); }
  function initialDifficulty(o, g, clamp) {
    var d = o.w[4] - Math.exp(o.w[5] * (g - 1)) + 1;
    return clamp ? clampD(d) : d;
  }
  function nextInterval(o, s) {
    var i = (s / o.factor) * (Math.pow(o.retention, 1 / o.decay) - 1);
    return Math.min(Math.max(pyRound(i), 1), o.maxInterval);
  }
  function shortTermStability(o, s, g) {
    var inc = Math.exp(o.w[17] * (g - 3 + o.w[18])) * Math.pow(s, -o.w[19]);
    if (g >= HARD) inc = Math.max(inc, 1);
    return clampS(s * inc);
  }
  function nextDifficulty(o, d, g) {
    var delta = -(o.w[6] * (g - 3));
    var damped = d + (10 - d) * delta / 9;
    return clampD(o.w[7] * initialDifficulty(o, EASY, false) + (1 - o.w[7]) * damped);
  }
  function forgetStability(o, d, s, r) {
    var long = o.w[11] * Math.pow(d, -o.w[12]) * (Math.pow(s + 1, o.w[13]) - 1) * Math.exp((1 - r) * o.w[14]);
    var short = s / Math.exp(o.w[17] * o.w[18]);
    return Math.min(long, short);
  }
  function recallStability(o, d, s, r, g) {
    var hard = g === HARD ? o.w[15] : 1, easy = g === EASY ? o.w[16] : 1;
    return s * (1 + Math.exp(o.w[8]) * (11 - d) * Math.pow(s, -o.w[9]) * (Math.exp((1 - r) * o.w[10]) - 1) * hard * easy);
  }
  function nextStability(o, d, s, r, g) {
    return clampS(g === AGAIN ? forgetStability(o, d, s, r) : recallStability(o, d, s, r, g));
  }
  function retrievability(card, now, o) {
    o = o && o.factor ? o : opts(o);
    if (!card || !card.l || !card.s) return 0;
    var t = Math.max(0, days(card.l, now));
    return Math.pow(1 + o.factor * t / card.s, o.decay);
  }
  // py-fsrs _get_fuzzed_interval; rand() in [0, 1).
  function fuzzed(o, ivl, rand) {
    if (ivl < 2.5) return ivl;
    var delta = 1;
    FUZZ.forEach(function (f) { delta += f.factor * Math.max(Math.min(ivl, f.end) - f.start, 0); });
    var lo = Math.max(2, pyRound(ivl - delta)), hi = Math.min(pyRound(ivl + delta), o.maxInterval);
    lo = Math.min(lo, hi);
    return Math.min(Math.floor(rand() * (hi - lo + 1) + lo), o.maxInterval);
  }

  // Steps of the learning or relearning phase: the next interval in ms, or
  // null when the card graduates to review.
  function stepInterval(card, g, steps) {
    if (!steps.length || (card.p >= steps.length && g >= HARD)) return null;
    switch (g) {
      case AGAIN: card.p = 0; return steps[0];
      case HARD:
        if (card.p === 0 && steps.length === 1) return steps[0] * 1.5;
        if (card.p === 0) return (steps[0] + steps[1]) / 2;
        return steps[card.p];
      case GOOD:
        if (card.p + 1 === steps.length) return null;
        card.p += 1;
        return steps[card.p];
      default: return null;
    }
  }

  // review(card or null, grade 1–4, now ms, options) → the new card.
  function review(card, g, now, o) {
    o = opts(o);
    if (!(g >= 1 && g <= 4)) throw new Error("fsrs: unknown grade " + g);
    var c = card ? JSON.parse(JSON.stringify(card)) : { t: LEARNING, p: 0, s: null, d: null, due: now, l: null, r: 0, f: 0 };
    var since = c.l ? days(c.l, now) : null;
    var r = retrievability(c, now, o);
    var ms;
    if (c.t === LEARNING || c.t === RELEARNING) {
      if (c.s === null || c.d === null) {
        c.s = initialStability(o, g);
        c.d = initialDifficulty(o, g, true);
      } else if (since !== null && since < 1) {
        c.s = shortTermStability(o, c.s, g);
        c.d = nextDifficulty(o, c.d, g);
      } else {
        c.s = nextStability(o, c.d, c.s, r, g);
        c.d = nextDifficulty(o, c.d, g);
      }
      ms = stepInterval(c, g, c.t === LEARNING ? o.learning : o.relearning);
      if (ms === null) { c.t = REVIEW; c.p = null; }
    } else {
      c.s = since !== null && since < 1 ? shortTermStability(o, c.s, g) : nextStability(o, c.d, c.s, r, g);
      c.d = nextDifficulty(o, c.d, g);
      ms = null;
      if (g === AGAIN) {
        c.f += 1;
        if (o.relearning.length) { c.t = RELEARNING; c.p = 0; ms = o.relearning[0]; }
      }
    }
    if (ms === null) {
      var ivl = nextInterval(o, c.s);
      if (o.fuzz && c.t === REVIEW) ivl = fuzzed(o, ivl, o.fuzz);
      ms = ivl * DAY;
    }
    c.due = now + ms;
    c.l = now;
    c.r += 1;
    return c;
  }

  exports.PARAMS = PARAMS;
  exports.DAY = DAY;
  exports.MIN = MIN;
  exports.STABILITY_MIN = STABILITY_MIN;
  exports.LEARNING = LEARNING;
  exports.REVIEW = REVIEW;
  exports.RELEARNING = RELEARNING;
  exports.AGAIN = AGAIN;
  exports.HARD = HARD;
  exports.GOOD = GOOD;
  exports.EASY = EASY;
  exports.review = review;
  exports.retrievability = retrievability;
  exports.nextInterval = function (s, o) { return nextInterval(opts(o), s); };
  exports.fuzzed = function (ivl, rand, o) { return fuzzed(opts(o), ivl, rand); };
  exports.pyRound = pyRound;
})(typeof exports !== "undefined" ? exports : (window.EoFSRS = {}));
