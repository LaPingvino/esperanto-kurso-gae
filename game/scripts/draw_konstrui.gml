// draw_konstrui(): Konstrui — a word built Zagreb-style from its pieces.
// The question by the word itself (its picture, else its Esperanto
// definition, else its meaning, marked as the translation it is); the line
// the pieces go on (a placed one goes back when pressed, leaving a dashed
// slot in the bank, so nothing jumps) with the word they spell under it;
// the bank; the dock with Kontroli. After the verdict each placed piece
// shows right or wrong (kurso-design's tile states, each with its badge).
var x, y, x0, x1, t, w, n, top, rowh, st, h, bot, dock, lv;
x0 = global.m; x1 = global.W - global.m;
if (global.wide) { x0 = (global.W - 640) / 2; x1 = x0 + 640; }
global.bx0 = x0; global.bx1 = x1;
lv = level_me();
y = global.hh + 12;
t = "Konstrui · " + lv; if (global.kb_tries > 0) t += " · " + string(global.kb_ok) + " el " + string(global.kb_tries);
txt(x0, y, fit(t, x1 - x0, 17, 1), 17, 1, global.C_text);
y += 34;
if (global.kb_pic >= 0) {
  box(x0, y, x0 + 108, y + 108, global.C_pic, global.C_edge, 1);
  draw_sprite(spr_bildo, global.kb_pic, x0 + 6, y + 6);
  txt(x0 + 120, y + 84, fit("Bildo: OpenMoji, CC BY-SA 4.0", x1 - x0 - 120, 13, 0), 13, 0, global.C_muted);
  y += 120;
} else {
  t = "Difino:"; if (global.kb_tr) t = "Traduko (mankas difino):";
  txt(x0, y, t, 15, 1, global.C_muted);
  y += 24;
  fits(global.kb_q, x1 - x0, 17, 0);
  y += tw(x0, y, global.kb_q, x1 - x0, 17, 0, global.C_text) + 14;
}
txt(x0, y, fit("Konstruu la vorton el la pecoj:", x1 - x0, 16, 0), 16, 0, global.C_muted);
y += 30;
// the line: measured first (as tall as its rows), then drawn
top = y; rowh = 56;
x = x0 + 12; y = top + 12;
for (t = 0; t < global.kb_n; t += 1) {
  w = tile_w(global.kb_t[global.kb_bp[t]], 20, 0) + 24 * (global.fb != 0); // after the verdict: room for the badge
  if (x + w > x1 - 12 && x > x0 + 12) { x = x0 + 12; y += rowh + 8; }
  x += w + 8;
}
bot = y + rowh + 12;
if (global.fb == 0) box(x0, top, x1, bot, global.C_panel, global.C_edge, 1);
else if (global.fb == 1) box(x0, top, x1, bot, global.C_okbg, global.C_okedge, 2);
else box(x0, top, x1, bot, global.C_badbg, global.C_badedge, 2);
x = x0 + 12; y = top + 12;
for (t = 0; t < global.kb_n; t += 1) {
  w = tile_w(global.kb_t[global.kb_bp[t]], 20, 0) + 24 * (global.fb != 0);
  if (x + w > x1 - 12 && x > x0 + 12) { x = x0 + 12; y += rowh + 8; }
  if (global.fb == 0) tilebox(x, y, w, global.kb_t[global.kb_bp[t]], 760 + t, 1, 20, 0);
  else {
    st = 3; if (t < global.kb_nb) { if (global.kb_t[global.kb_bp[t]] == global.kb_b[t]) st = 2; }
    if (global.fb == 1) st = 2;
    tilebox(x, y, w, global.kb_t[global.kb_bp[t]], -1, st, 20, 0);
  }
  x += w + 8;
}
if (global.kb_n == 0) txt(x0 + 16, top + 28, fit("Premu pecon por meti ĝin ĉi tien.", x1 - x0 - 32, 15, 0), 15, 0, global.C_muted);
y = bot + 10;
if (global.kb_n > 0) { txt(x0, y, fit("= " + kb_built(), x1 - x0, 20, 1), 20, 1, global.C_link); }
y += 40;
// the bank
x = x0; rowh = 0;
for (t = 0; t < global.kb_nt; t += 1) {
  w = tile_w(global.kb_t[t], 20, 0);
  if (x + w > x1 && x > x0) { x = x0; y += rowh + 10; rowh = 0; }
  if (global.kb_used[t]) h = tilebox(x, y, w, global.kb_t[t], -1, 5, 20, 0);
  else if (global.fb != 0) h = tilebox(x, y, w, global.kb_t[t], -1, 6, 20, 0);
  else h = tilebox(x, y, w, global.kb_t[t], 780 + t, 0, 20, 0);
  rowh = max(rowh, h);
  x += w + 10;
}
y += rowh + 14;
dock = draw_dock();
if (y > dock) overflow("Konstrui runs under the dock");
