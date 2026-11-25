// mark(cx, cy, r, ok): the second cue of right and wrong (never colour
// alone): a disc in the right or wrong edge colour with a tick (ok 1) or a
// cross (ok 0) in the surface colour (3:1 or more, DESIGN.md). Drawn with
// lines: the engine's font has no ✓ or ✗.
var c, r, w;
r = argument2;
if (argument3) c = global.C_okedge; else c = global.C_badedge;
brush_color = c; pen_color = c;
draw_circle(argument0, argument1, r);
w = max(2, round(r / 4));
if (argument3) {
  thick(argument0 - r * 0.45, argument1 + r * 0.02, argument0 - r * 0.1, argument1 + r * 0.38, w, global.C_panel);
  thick(argument0 - r * 0.1, argument1 + r * 0.38, argument0 + r * 0.5, argument1 - r * 0.36, w, global.C_panel);
} else {
  thick(argument0 - r * 0.38, argument1 - r * 0.38, argument0 + r * 0.38, argument1 + r * 0.38, w, global.C_panel);
  thick(argument0 - r * 0.38, argument1 + r * 0.38, argument0 + r * 0.38, argument1 - r * 0.38, w, global.C_panel);
}
