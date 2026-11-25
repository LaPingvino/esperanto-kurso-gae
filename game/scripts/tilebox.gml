// tilebox(x, y, w, label, action, state, size, num): the one tile every
// activity is built from (kurso-design DESIGN.md), its label wrapped inside
// it and the tile as tall as that needs; returns the height it takes (with
// the ledge). num > 0: its key number at the left (and room at the right
// for the badge). action < 0: not pressable. States, each with a cue besides
// its colour:
//   0 new: surface, edge, a 4 px ledge (pressable)
//   1 trying (picked): accent border 3 px, accent ledge and text
//   2 right: right tint and edge, a ✓ badge
//   3 wrong: wrong tint and edge, a ✗ badge
//   4 done: sunk, dashed, muted text, no ledge, sits lower
//   5 an empty slot (a tile taken from the bank): dashed, no label
//   6 idle (out of play after the verdict): surface, edge, no ledge
var x, y, w, h, l, f, e, t, b, ledge, inner, padl, ty;
x = argument0; y = argument1; w = argument2;
h = tile_h(argument3, w, argument6, argument7);
l = 4;
padl = 16 + 28 * (argument7 != 0);
inner = w - padl - 16 - 24 * (argument7 > 0);
fits(argument3, inner, argument6, 0);
f = global.C_panel; e = global.C_edge; t = global.C_text; b = 2; ledge = global.C_edge;
if (argument5 == 1) { e = global.C_main; t = global.C_link; b = 3; ledge = global.C_main; }
if (argument5 == 2) { f = global.C_okbg; e = global.C_okedge; ledge = global.C_okedge; }
if (argument5 == 3) { f = global.C_badbg; e = global.C_badedge; ledge = global.C_badedge; }
if (argument5 == 4 || argument5 == 5) {
  if (argument5 == 4) { fill(x, y + l, x + w, y + l + h, global.C_sunk); }
  dashed(x, y + l, x + w, y + l + h, global.C_edge);
  if (argument5 == 4) {
    ty = y + l + (h - th(argument3, inner, argument6, 0)) / 2;
    tw(x + padl, ty, argument3, inner, argument6, 0, global.C_muted);
  }
  return h + l;
}
if (argument5 != 6) rfill(x, y + l, x + w, y + h + l, ledge, ledge);
box(x, y, x + w, y + h, f, e, b);
if (argument7 != 0) { // the key number (a key hint: --text-xs)
  rfill(x + 10, y + h / 2 - 12, x + 34, y + h / 2 + 12, global.C_sunk, global.C_sunk);
  txtc(x + 22, y + h / 2 - 9, string(abs(argument7)), 13, 1, global.C_muted);
}
ty = y + (h - th(argument3, inner, argument6, 0)) / 2;
tw(x + padl, ty, argument3, inner, argument6, 0, t);
if (argument5 == 2) mark(x + w - 21, y + h / 2, 11, 1);
if (argument5 == 3) mark(x + w - 21, y + h / 2, 11, 0);
if (argument4 >= 0) hs(x, y, x + w, y + h + l, argument4);
return h + l;
