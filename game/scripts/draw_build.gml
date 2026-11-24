// draw_build(y): a sentence of the lesson's text, its words shuffled: put
// them in order. A placed word goes back when pressed.
var l, k, e, x, y, x0, x1, t, w, n, top;
l = global.les; k = global.idx; e = l * 100 + k;
x0 = global.m; x1 = global.W - global.m;
if (global.wide) { x0 = (global.W - 900) / 2; x1 = x0 + 900; }
y = argument0;
txt(x0, y, "Metu la vortojn en la ordon de la teksto:", 17, 0, global.C_muted);
y += 36;
top = y;
rfill(x0, top, x1, top + 132, global.C_panel, global.C_line);
x = x0 + 14; y = top + 14;
for (t = 0; t < global.bn; t += 1) {
  w = tile_w(global.b_tile[e, global.bp[t]], 18);
  if (x + w > x1 - 14) { x = x0 + 14; y += 58; }
  if (global.fb == 0) tile(x, y, global.b_tile[e, global.bp[t]], 400 + t, 1, 18);
  else if (global.fb == 1) tile(x, y, global.b_tile[e, global.bp[t]], -1, 3, 18);
  else tile(x, y, global.b_tile[e, global.bp[t]], -1, 4, 18);
  x += w + 10;
}
y = top + 156;
x = x0;
n = global.b_nt[l, k];
for (t = 0; t < n; t += 1) {
  w = tile_w(global.b_tile[e, t], 18);
  if (x + w > x1 && x > x0) { x = x0; y += 60; }
  if (global.bused[t] || global.fb != 0) tile(x, y, global.b_tile[e, t], -1, 6, 18);
  else tile(x, y, global.b_tile[e, t], 300 + t, 0, 18);
  x += w + 12;
}
y += 76;
if (global.fb == 0) button(x0, y, x0 + bw(global.t_dont_know), y + 44, global.t_dont_know, 7, 5);
