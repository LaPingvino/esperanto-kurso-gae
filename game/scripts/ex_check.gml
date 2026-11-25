// ex_check(): Kontroli — the answer judged as the site judges it. Gaps: each
// by gap_ok; typed in every gap, it goes to the site as an attempt (ob_add,
// sync_tick): only typed answers move the site's ratings, choosing a tile is
// easier than typing.
var g, all, typed;
if (!ex_ready()) exit;
if (global.kind == 0 || global.kind == 3) { ex_done(choice_ok(global.idx, global.pick)); exit; }
if (global.kind == 2) { ex_done(build_ok()); exit; }
all = 1; typed = 1;
for (g = 0; g < global.ngap; g += 1) {
  global.fok[g] = gap_ok(global.idx, g, global.fa[g]);
  if (!global.fok[g]) all = 0;
  if (!global.ftyped[g]) typed = 0;
}
if (typed) ob_add();
ex_done(all);
