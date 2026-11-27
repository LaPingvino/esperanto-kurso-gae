// test-ripetoj: the cards' order and the rebuild from the server's trace
// (static/ripetoj.js over static/fsrs.js), run without a browser
// (node tools/test-ripetoj.mjs; go test runs it too, node_test.go).
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const load = (name) => {
  const E = {};
  vm.runInNewContext(fs.readFileSync(path.join(ROOT, "static", name), "utf8"), { exports: E });
  return E;
};
const F = load("fsrs.js");
const R = load("ripetoj.js");
const { DAY, MIN, AGAIN, HARD, GOOD, EASY } = F;

let n = 0;
const t = (name, fn) => {
  fn();
  n++;
};
const NOW = Date.UTC(2026, 10, 27, 15, 0, 0);
const today = Math.floor(NOW / DAY);
const card = (daysAgo, count, grade) => R.rebuildCard(F, today - daysAgo, count, grade, NOW);

t("rebuild: the last review is noon UTC of its day, never after now", () => {
  const c = card(3, 2, GOOD);
  assert.equal(c.l, (today - 3) * DAY + DAY / 2);
  const c0 = R.rebuildCard(F, today, 1, GOOD, (today * DAY) + 3600000);
  assert.ok(c0.l <= (today * DAY) + 3600000);
});
t("rebuild: due never before the last review; a valid card for clean()", () => {
  for (const g of [AGAIN, HARD, GOOD, EASY]) for (const k of [1, 2, 3, 5, 12, 63]) for (const ago of [0, 1, 7, 40, 400]) {
    const c = card(ago, k, g);
    assert.ok(c.due > c.l, `due ${c.due} ≤ last ${c.l} (grade ${g}, ${k} answers, ${ago} days ago)`);
    assert.ok(c.s > 0 && c.d >= 1 && c.d <= 10 && [1, 2, 3].includes(c.t));
    assert.equal(c.r, k);
  }
  const s = R.clean({ c: { a: card(2, 3, GOOD) } });
  assert.ok(s.c.a, "clean() drops a rebuilt card");
});
t("rebuild: a last Again is due within a day of the last answer (learning or relearning)", () => {
  for (const k of [1, 2, 4, 10]) {
    const c = card(5, k, AGAIN);
    assert.ok(c.due - c.l <= DAY, `${k} answers then Again: due ${(c.due - c.l) / DAY} days after`);
    assert.ok(c.t === F.LEARNING || c.t === F.RELEARNING);
  }
});
t("rebuild: one Good answer is learning, due 10 min after it, as a real first answer", () => {
  const c = card(1, 1, GOOD);
  assert.equal(c.t, F.LEARNING);
  assert.equal(c.due - c.l, 10 * MIN);
});
t("rebuild: from two Good/Easy answers on, a review card; more answers, a longer interval", () => {
  let prev = 0;
  for (const k of [2, 3, 4]) {
    const c = card(0, k, GOOD);
    assert.equal(c.t, F.REVIEW, `${k} answers`);
    const ivl = c.due - c.l;
    assert.ok(ivl > prev, `${k} answers: interval ${ivl / DAY} d, not longer than ${prev / DAY} d`);
    prev = ivl;
  }
  assert.equal(card(0, 2, EASY).t, F.REVIEW);
  assert.ok(card(0, 3, EASY).due > card(0, 3, GOOD).due, "Easy not later than Good");
  assert.ok(card(0, 3, HARD).due < card(0, 3, GOOD).due, "Hard not sooner than Good");
});
t("rebuild: a long count replays at most REPLAY answers, so the interval stays sensible", () => {
  const c = card(0, 63, GOOD);
  assert.deepEqual(c.due - c.l, card(0, R.REPLAY, GOOD).due - card(0, R.REPLAY, GOOD).l);
  assert.ok((c.due - c.l) / DAY < 365, `63 answers: ${(c.due - c.l) / DAY} days`);
  assert.ok((c.due - c.l) / DAY >= 30, `63 answers: only ${(c.due - c.l) / DAY} days`);
});
t("rebuild: a card answered long ago is due now; one answered today is not", () => {
  const old = card(400, 3, GOOD), fresh = card(0, 3, GOOD);
  assert.ok(old.due <= NOW, "a year-old card is not due");
  assert.ok(fresh.due > R.dayEnd(NOW), "a card answered today is due today");
  const s = R.clean({ c: { old, fresh } });
  assert.deepEqual([...R.due(s, null, NOW).rev], ["old"]);
});
t("addMissing: adds the cards the state lacks and keeps the ones it has", () => {
  const s = R.clean(null);
  s.c.a = F.review(null, GOOD, NOW - DAY);
  const keep = s.c.a;
  const added = R.addMissing(F, s, [["a", today, 5, EASY], ["b", today - 3, 2, GOOD], [null, 1, 1, 1], ["c", today - 1, 1, AGAIN]], NOW);
  assert.equal(added, 2);
  assert.equal(s.c.a, keep);
  assert.ok(s.c.b && s.c.c);
  assert.equal(R.addMissing(F, s, null, NOW), 0);
});
t("new cards: a series opens card by card; the window is the three open ones nearest the rating", () => {
  const chain = [{ id: "s1", r: 900 }, { id: "s2", r: 950 }, { id: "s3", r: 1000 }, { id: "s4", r: 1050 }];
  const single = [{ id: "x", r: 1800 }];
  const seen = (ids) => (id) => ids.includes(id);
  assert.deepEqual([...R.window([chain, single], seen([]), 900).map((c) => c.id)], ["s1", "x"]);
  assert.deepEqual([...R.window([chain, single], seen(["s1"]), 900).map((c) => c.id)], ["s2", "s3", "x"]);
  assert.deepEqual([...R.window([chain, single], seen(["s2"]), 1800).map((c) => c.id)], ["x", "s4", "s3"]);
  const pick = R.pickNew([chain], seen([]), 900, "2026-11-27:0");
  assert.equal(pick.id, "s1");
});
t("the day's limit of new cards, and +5 more", () => {
  const s = R.clean(null);
  assert.equal(R.newLeft(s, NOW, 10), 10);
  for (let i = 0; i < 4; i++) R.countNew(s, NOW);
  assert.equal(R.newLeft(s, NOW, 10), 6);
  R.moreNew(s, NOW, 5);
  assert.equal(R.newLeft(s, NOW, 10), 11);
  assert.equal(R.newLeft(s, NOW + 2 * DAY, 10), 10);
});

console.log(`test-ripetoj: ${n} testoj bone`);
