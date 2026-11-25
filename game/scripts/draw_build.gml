// draw_build(y): a sentence of the lesson's text, its words shuffled: put
// them in order. A placed word goes back when pressed and leaves a dashed
// slot in the bank, so nothing jumps; after Kontroli the line takes the
// verdict's colour and one badge at its end. Returns the body's bottom.
var i, x, y, x0, x1, t, w, n, top, rowh, st, h, bot;
i = global.idx;
x0 = global.bx0; x1 = global.bx1;
y = argument0;
txt(x0, y, "Metu la vortojn en la ordon de la teksto:", 17, 0, global.C_muted);
y += 34;
top = y;
// the line: measured first (its box is as tall as its rows), then drawn
x = x0 + 12; y = top + 12; rowh = 60;
for (t = 0; t < global.bn; t += 1) {
  w = min(tile_w(global.x_tile[i, global.bp[t]], 18, 0), x1 - x0 - 24);
  if (x + w > x1 - 12 && x > x0 + 12) { x = x0 + 12; y += rowh + 8; }
  x += w + 8;
}
bot = y + rowh + 12;
st = 1;
if (global.fb == 1) st = 2;
if (global.fb == 2) st = 3;
if (global.fb == 0) box(x0, top, x1, bot, global.C_panel, global.C_edge, 1);
else if (global.fb == 1) box(x0, top, x1, bot, global.C_okbg, global.C_okedge, 2);
else box(x0, top, x1, bot, global.C_badbg, global.C_badedge, 2);
x = x0 + 12; y = top + 12;
for (t = 0; t < global.bn; t += 1) {
  w = min(tile_w(global.x_tile[i, global.bp[t]], 18, 0), x1 - x0 - 24);
  if (x + w > x1 - 12 && x > x0 + 12) { x = x0 + 12; y += rowh + 8; }
  if (global.fb == 0) tilebox(x, y, w, global.x_tile[i, global.bp[t]], 400 + t, 1, 18, 0);
  else tilebox(x, y, w, global.x_tile[i, global.bp[t]], -1, 6, 18, 0);
  x += w + 8;
}
if (global.fb != 0) { mark(x1 - 2, top, 12, global.fb == 1); return bot + 16; } // one badge at the line's end
y = bot + 16;
x = x0; rowh = 0;
n = global.x_nt[i, 0];
for (t = 0; t < n; t += 1) {
  w = min(tile_w(global.x_tile[i, t], 18, 0), x1 - x0);
  if (x + w > x1 && x > x0) { x = x0; y += rowh + 10; rowh = 0; }
  if (global.bused[t]) h = tilebox(x, y, w, global.x_tile[i, t], -1, 5, 18, 0);
  else h = tilebox(x, y, w, global.x_tile[i, t], 300 + t, 0, 18, 0);
  rowh = max(rowh, h);
  x += w + 10;
}
return y + rowh + 14;
