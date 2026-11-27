// draw_levels(): the course: its levels (KER A0 …, as text chips: the
// learner's current level filled, the others tinted), each with its series
// as the site groups them (/serioj); a series' tile shows its picture, its
// name wrapped inside it and its places done (a finished one: ✓). A row is
// as tall as its tallest tile. Paged when it doesn't fit (from series
// global.lv_skip on).
var x, y, x0, x1, s, lv, h, n, cols, cw, k, rs, re, cur, inner, bot, cl;
x0 = global.m; x1 = global.W - global.m;
y = global.hh + 18;
txt(x0, y, "La kurso — elektu serion", 20, 1, global.C_text);
y += 40;
cols = 4; if (!global.wide) cols = 2;
cw = (x1 - x0 - (cols - 1) * 12) / cols;
if (global.lv_skip == 0) { // the bonus games, on the first page: the same words, learnt otherwise
  font(15, 1, 0); cl = string_width("Ludoj") + 24;
  rfill(x0, y, x0 + cl, y + 28, global.C_tint, global.C_tint); txt(x0 + 12, y + 5, "Ludoj", 15, 1, global.C_link);
  y += 38;
  if (cols > 2) { // a wide screen: the six in a row
    k = (x1 - x0 - 5 * 12) / 6;
    for (s = 0; s < 6; s += 1) game_tile(x0 + s * (k + 12), y, k, s);
  } else { // a phone: three rows of two
    for (s = 0; s < 6; s += 2) { if (s > 0) y += 64 + 12; game_tile(x0, y, cw, s); game_tile(x0 + cw + 12, y, cw, s + 1); }
  }
  y += 64 + 12;
}
inner = cw - 24 - 40;
bot = global.H - 76;
cur = ""; // the current level: the first with a series not finished
for (s = 0; s < global.ns; s += 1) if (cur == "") { if (series_done(s) < global.s_np[s]) cur = global.s_level[s]; }
lv = "";
global.lv_next = -1;
s = global.lv_skip;
while (s < global.ns) {
  if (global.s_level[s] != lv) { // a level's chip, on a new row
    if (y + 40 + 60 > bot) { global.lv_next = s; break; }
    lv = global.s_level[s];
    font(15, 1, 0); cl = string_width(lv) + 24;
    if (lv == cur) { rfill(x0, y, x0 + cl, y + 28, global.C_main, global.C_main); txt(x0 + 12, y + 5, lv, 15, 1, global.C_on); }
    else { rfill(x0, y, x0 + cl, y + 28, global.C_tint, global.C_tint); txt(x0 + 12, y + 5, lv, 15, 1, global.C_link); }
    y += 38;
  }
  // this row: up to cols series of the same level, measured, then drawn
  rs = s; re = s; h = 64;
  while (re < global.ns && re - rs < cols) {
    if (global.s_level[re] != lv) break;
    h = max(h, th(global.s_label[re], inner, 15, 1) + 44);
    re += 1;
  }
  if (y + h > bot) { global.lv_next = s; break; }
  for (k = rs; k < re; k += 1) {
    x = x0 + (k - rs) * (cw + 12);
    n = series_done(k);
    box(x, y, x + cw, y + h, global.C_panel, global.C_edge, 1);
    if (n > 0) rfill(x + 10, y + h - 9, x + 10 + (cw - 20) * n / max(1, global.s_np[k]), y + h - 5, global.C_acc, global.C_acc);
    draw_sprite(spr_place_s, series_pic(k), x + cw - 38, y + 10); // the series' picture
    fits(global.s_label[k], inner, 15, 1);
    tw(x + 12, y + 9, global.s_label[k], inner, 15, 1, global.C_text);
    if (n >= global.s_np[k]) { mark(x + 20, y + h - 24, 8, 1); txt(x + 34, y + h - 33, string(n) + " / " + string(global.s_np[k]), 15, 0, global.C_muted); }
    else txt(x + 12, y + h - 33, string(n) + " / " + string(global.s_np[k]), 15, 0, global.C_muted);
    hs(x, y, x + cw, y + h, 20000 + k);
    if (global.def_act < 0) { if (n < global.s_np[k]) global.def_act = 20000 + k; } // Enter: the first series not finished
  }
  y += h + 12;
  s = re;
}
y = global.H - 62;
if (global.lv_skip > 0) button(x0, y, x0 + bw(global.t_series_prev_fallback), y + 44, global.t_series_prev_fallback, 17, 0);
if (global.lv_next >= 0) button(x1 - bw(global.t_series_next_fallback), y, x1, y + 44, global.t_series_next_fallback, 18, 0);
if (!global.site && global.wide) txt(x0 + 140, y + 12, "Sen la retejo: nur la enkonstruita parto.", 15, 0, global.C_muted);
