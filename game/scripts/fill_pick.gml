// fill_pick(text): text into the current gap; when every gap has an
// answer, the whole exercise is judged at once, as the site judges it.
var g, all;
global.fa[global.cg] = argument0;
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
ex_done(all);
