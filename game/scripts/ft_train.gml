// ft_train(x0, x1, top, draw): Frazotrajno's train laid out from top — the
// locomotive (the site's star on it), then the coupled wagons, wrapping onto
// another track when a row is full; while coupling, a dashed slot for each
// wagon still to come. Drawn (draw 1) or only measured; returns its bottom.
// A wagon is a word tile on wheels: coupled (the picked state), and on a
// train judged right ✓, or out of place ✗ (all from the first one out on).
var x0, x1, y, x, k, w, l, st, h, rowh, d, prev;
x0 = argument0; x1 = argument1; y = argument2;
rowh = 84;
if (argument3) {
  thick(x0, y + 75, x1, y + 75, 3, global.C_edge); // the rails
  rfill(x0, y + 24, x0 + 72, y + 60, global.C_main, global.C_main); // the locomotive
  rfill(x0 + 42, y + 2, x0 + 72, y + 32, global.C_main, global.C_main);
  rfill(x0 + 50, y + 9, x0 + 64, y + 23, global.C_tint, global.C_tint);
  rfill(x0 + 8, y + 8, x0 + 22, y + 26, global.C_main, global.C_main);
  star(x0 + 23, y + 42, 10, global.C_on);
  brush_color = global.C_text; pen_color = global.C_text;
  draw_circle(x0 + 14, y + 66, 6); draw_circle(x0 + 36, y + 66, 6); draw_circle(x0 + 58, y + 66, 6);
  if (global.ft_st == 2) { // off it goes: steam
    d = 45 - global.ft_wait;
    brush_color = global.C_line; pen_color = global.C_line;
    draw_circle(x0 + 15 + d / 4, y + 2 - d / 3, 5 + d / 9);
  }
}
x = x0 + 84; prev = x0 + 72;
for (k = 0; k < global.ft_nb; k += 1) {
  if (k >= global.ft_n && global.ft_st != 0) break;
  if (k < global.ft_n) { l = global.ft_t[global.ft_bp[k]]; w = min(tile_w(l, 18, 0) + 24, x1 - x0); } // room for the badge throughout: nothing moves at the verdict
  else w = 56;
  if (x + w > x1 && x > x0) { // another track
    x = x0; y += rowh; prev = -1;
    if (argument3) thick(x0, y + 75, x1, y + 75, 3, global.C_edge);
  }
  if (argument3) {
    if (k < global.ft_n) {
      st = 1;
      if (global.ft_st == 1) { st = 3; if (k < global.ft_good) st = 2; }
      if (global.ft_st >= 2) st = 2;
      if (prev >= 0) thick(prev, y + 44, x, y + 44, 4, global.C_edge); // the coupling
      h = tilebox(x, y, w, l, 860 + k, st, 18, 0);
      brush_color = global.C_text; pen_color = global.C_text;
      draw_circle(x + 16, y + 66, 6); draw_circle(x + w - 16, y + 66, 6);
    } else dashed(x, y + 4, x + w, y + 60, global.C_edge);
  }
  prev = x + w;
  x += w + 12;
}
return y + rowh;
