// draw_levels(): the course: its levels (KER A0 …), each with its series
// as the site groups them (/serioj); a series' tile shows its places done.
// Paged when it doesn't fit (from series global.lv_skip on).
var x, y, x0, x1, s, lv, h, lab, n, cols, cw, row;
x0 = global.m; x1 = global.W - global.m;
y = global.hh + 16;
txt(x0, y, "La kurso — elektu serion", 18, 1, global.C_text);
y += 36;
cols = 4; if (!global.wide) cols = 2;
cw = (x1 - x0 - (cols - 1) * 10) / cols;
h = 54; if (!global.wide) h = 50;
lv = ""; row = 0;
global.lv_next = -1;
for (s = global.lv_skip; s < global.ns; s += 1) {
  if (global.s_level[s] != lv) { // a level's heading, on a new row
    if (row > 0) { y += h + 10; row = 0; }
    if (y + 28 + h > global.H - 70) { global.lv_next = s; break; }
    lv = global.s_level[s];
    txt(x0, y, lv, 17, 1, global.C_main);
    y += 28;
  }
  if (row == cols) { y += h + 10; row = 0; }
  if (y + h > global.H - 70) { global.lv_next = s; break; }
  x = x0 + row * (cw + 10);
  n = series_done(s);
  lab = global.s_label[s];
  rfill(x, y, x + cw, y + h, global.C_panel, global.C_line);
  if (n > 0) fill(x + 2, y + h - 6, x + 2 + (cw - 4) * n / max(1, global.s_np[s]), y + h - 3, global.C_main);
  font(15, 1, 0);
  while (string_width(lab) > cw - 20 && string_length(lab) > 4) lab = string_copy(lab, 1, string_length(lab) - 2) + "…";
  txt(x + 10, y + 7, lab, 15, 1, global.C_text);
  txt(x + 10, y + 28, string(n) + " / " + string(global.s_np[s]), 13, 0, global.C_muted);
  hs(x, y, x + cw, y + h, 20000 + s);
  if (global.def_act < 0) { if (n < global.s_np[s]) global.def_act = 20000 + s; } // Enter: the first series not finished
  row += 1;
}
y = global.H - 60;
if (global.lv_skip > 0) button(x0, y, x0 + bw(global.t_series_prev_fallback), y + 44, global.t_series_prev_fallback, 17, 0);
if (global.lv_next >= 0) button(x1 - bw(global.t_series_next_fallback), y, x1, y + 44, global.t_series_next_fallback, 18, 0);
if (!global.site && global.wide) txt(x0 + 140, y + 12, "Sen la retejo: nur la enkonstruita parto.", 13, 0, global.C_muted);
