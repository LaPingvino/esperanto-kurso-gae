// fill_pick(text, typed): text into the current gap (typed: 1 when the
// player typed it, 0 a tile); when every gap has an answer, the whole
// exercise is judged at once, as the site judges it. An exercise typed in
// every gap goes to the site as an attempt (ob_add, sync_tick): only typed answers
// move the site's ratings — a tile is easier than typing.
var g, all, typed;
global.fa[global.cg] = argument0;
global.ftyped[global.cg] = argument1;
global.cg += 1;
global.typed = "";
keyboard_string = "";
global.focus = -1;
if (global.cg < global.ngap) exit;
all = 1;
for (g = 0; g < global.ngap; g += 1) {
  global.fok[g] = fill_ok(global.les, global.idx, g, global.fa[g]);
  if (!global.fok[g]) all = 0;
}
typed = 1;
for (g = 0; g < global.ngap; g += 1) if (!global.ftyped[g]) typed = 0;
if (typed) ob_add();
ex_done(all);
