// test-fsrs: the FSRS-6 scheduler (static/fsrs.js), run without a browser
// (node tools/test-fsrs.mjs; go test runs it too, node_test.go). The
// expected values are py-fsrs 6.3.3's own (tests/test_basic.py), not numbers
// taken from this port. Copied from angla.org's tools/test-fsrs.mjs.
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const src = fs.readFileSync(path.join(ROOT, "static", "fsrs.js"), "utf8");
const F = {};
vm.runInNewContext(src, { exports: F });
const { AGAIN, HARD, GOOD, EASY, DAY, MIN } = F;

let n = 0;
const t = (name, fn) => {
  fn();
  n++;
};
const near = (got, want, tol, what) => assert.ok(Math.abs(got - want) <= tol, `${what}: ${got}, want ${want}`);
const START = Date.UTC(2022, 10, 29, 12, 30, 0);
const ivlDays = (c) => Math.floor((c.due - c.l) / DAY);

t("py-fsrs test_review_card: the intervals of 6 Good, 2 Again, 5 Good", () => {
  const ratings = [GOOD, GOOD, GOOD, GOOD, GOOD, GOOD, AGAIN, AGAIN, GOOD, GOOD, GOOD, GOOD, GOOD];
  let card = null, now = START;
  const hist = [];
  for (const g of ratings) {
    card = F.review(card, g, now);
    hist.push(ivlDays(card));
    now = card.due;
  }
  assert.deepEqual(hist, [0, 2, 11, 46, 163, 498, 0, 0, 2, 4, 7, 12, 21]);
});
t("py-fsrs test_memo_state: stability 53.62691, difficulty 6.3574867", () => {
  let card = null, now = START;
  const ratings = [AGAIN, GOOD, GOOD, GOOD, GOOD, GOOD];
  const gaps = [0, 0, 1, 3, 8, 21];
  ratings.forEach((g, i) => {
    now += gaps[i] * DAY;
    card = F.review(card, g, now);
  });
  near(card.s, 53.62691, 1e-4, "stability");
  near(card.d, 6.3574867, 1e-4, "difficulty");
});
t("py-fsrs test_repeated_correct_reviews: ten Easy give difficulty 1", () => {
  let card = null;
  for (let i = 0; i < 10; i++) card = F.review(card, EASY, START + i);
  assert.equal(card.d, 1);
});
t("py-fsrs learning steps: Good 10 min, Again 1 min, Hard 5.5 min, Easy a day", () => {
  let c = F.review(null, GOOD, START);
  assert.equal(c.t, F.LEARNING);
  assert.equal(c.p, 1);
  assert.equal(c.due - START, 10 * MIN);
  c = F.review(c, GOOD, c.due);
  assert.equal(c.t, F.REVIEW);
  assert.equal(c.p, null);
  assert.ok(c.due - START >= DAY);
  const a = F.review(null, AGAIN, START);
  assert.equal(a.p, 0);
  assert.equal(a.due - START, 1 * MIN);
  const h = F.review(null, HARD, START);
  assert.equal(h.p, 0);
  assert.equal(h.due - START, 5.5 * MIN);
  const e = F.review(null, EASY, START);
  assert.equal(e.t, F.REVIEW);
  assert.ok(e.due - START >= DAY);
});
t("py-fsrs test_review_state: Again on a review card relearns in 10 min", () => {
  let c = F.review(null, GOOD, START);
  c = F.review(c, GOOD, c.due);
  c = F.review(c, GOOD, c.due);
  assert.equal(c.t, F.REVIEW);
  const before = c.due;
  c = F.review(c, AGAIN, c.due);
  assert.equal(c.t, F.RELEARNING);
  assert.equal(c.due - before, 10 * MIN);
  assert.equal(c.f, 1);
  const late = F.review(c, GOOD, c.due + DAY);
  assert.equal(late.t, F.REVIEW);
});
t("py-fsrs test_same_day_hard_review_does_not_decrease_stability", () => {
  const o = { learning: [] };
  const c = F.review(null, GOOD, START, o);
  const h = F.review(c, HARD, START + MIN, o);
  assert.equal(h.s, c.s);
});
t("py-fsrs test_stability_lower_bound: a thousand late Again", () => {
  let c = null;
  for (let i = 0; i < 1000; i++) {
    c = F.review(c, AGAIN, c ? c.due + DAY : START);
    assert.ok(c.s >= F.STABILITY_MIN);
  }
});
t("py-fsrs test_maximum_interval", () => {
  const o = { maxInterval: 100 };
  let c = null, now = START;
  for (const g of [EASY, GOOD, EASY, GOOD]) {
    c = F.review(c, g, now, o);
    assert.ok(ivlDays(c) <= 100);
    now = c.due;
  }
});
t("py-fsrs test_no_learning_steps: the first review goes straight to review", () => {
  const c = F.review(null, AGAIN, START, { learning: [] });
  assert.equal(c.t, F.REVIEW);
  assert.ok(c.due - START >= DAY);
});
t("retrievability: 0 for a new card, 0.9 after one stability's days", () => {
  assert.equal(F.retrievability(null, START), 0);
  const c = F.review(null, EASY, START);
  near(F.retrievability({ ...c, s: 10 }, START + 10 * DAY), 0.9, 1e-9, "R(S)");
});
t("fuzz: py-fsrs's ranges, never under 2 days, intervals under 2.5 untouched", () => {
  assert.equal(F.fuzzed(2, () => 0.99), 2);
  for (const r of [0, 0.5, 0.999]) {
    const v = F.fuzzed(10, () => r);
    assert.ok(v >= 8 && v <= 12, `fuzz 10 → ${v}`);
  }
  assert.equal(F.fuzzed(100, () => 0), 100 - Math.round(1 + 0.15 * 4.5 + 0.1 * 13 + 0.05 * 80));
});
t("pyRound: halves to even, as Python", () => {
  assert.deepEqual([0.5, 1.5, 2.5, 2.6, 3.4].map(F.pyRound), [0, 2, 2, 3, 3]);
});

console.log(`test-fsrs: ${n} testoj bone`);
