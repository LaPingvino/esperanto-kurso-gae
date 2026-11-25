// draw_map(): a series' places on a path, twelve to a page; each finished
// place opens the next.
var s, f, n, l, k, x, y, x2, y2, c, r, open, lx, ly, nm, cur, st, k0, k1;
s = global.ser;
f = global.s_first[s]; n = global.s_np[s];
txt(global.m, global.hh + 18, global.s_label[s] + " — " + global.s_level[s], 18, 1, global.C_text);
r = 26; if (!global.wide) r = 22;
cur = -1;
for (l = f; l < f + n; l += 1) { if (cur < 0) { if (stars_get(global.p_slug[l]) == 0) { if (place_open_ok(l)) cur = l; } } }
if (global.mpage * 12 >= n) global.mpage = 0;
k0 = global.mpage * 12; k1 = min(n, k0 + 12);
for (k = k0; k < k1 - 1; k += 1) { // the path
  map_pos(k - k0); x = global.px; y = global.py;
  map_pos(k + 1 - k0); x2 = global.px; y2 = global.py;
  c = global.C_path;
  if (place_open_ok(f + k + 1)) c = global.C_main;
  for (l = -2; l <= 2; l += 1) { pen_color = c; draw_line(x + l, y, x2 + l, y2); draw_line(x, y + l, x2, y2 + l); }
}
for (k = k0; k < k1; k += 1) {
  l = f + k;
  map_pos(k - k0); x = global.px; y = global.py;
  open = place_open_ok(l);
  st = stars_get(global.p_slug[l]);
  nm = global.p_title[l];
  font(15, 1, 0);
  while (string_width(nm) > 170 && string_length(nm) > 6) nm = string_copy(nm, 1, string_length(nm) - 2) + "…";
  if (st > 0) { brush_color = global.C_main; pen_color = global.C_main; draw_circle(x, y, r); txtc(x, y - 12, string(k + 1), 18, 1, global.C_on); }
  else if (open) {
    brush_color = global.C_main; pen_color = global.C_main; draw_circle(x, y, r + 4);
    brush_color = global.C_panel; pen_color = global.C_panel; draw_circle(x, y, r - 1);
    txtc(x, y - 12, string(k + 1), 18, 1, global.C_main);
  } else {
    brush_color = global.C_line; pen_color = global.C_line; draw_circle(x, y, r);
    brush_color = global.C_lock; pen_color = global.C_lock; draw_circle(x, y, r - 2);
    txtc(x, y - 12, string(k + 1), 18, 1, global.C_muted);
  }
  if (global.wide) {
    lx = x; ly = y + r + 10;
    if (open) txtc(lx, ly, nm, 15, 1, global.C_text); else txtc(lx, ly, nm, 15, 0, global.C_muted);
    if (st > 0) stars(lx, ly + 34, 8, st);
    if (l == cur) { txtc(lx, ly + 24, "Komencu ĉi tie", 13, 0, global.C_main); global.def_act = 10000 + l; }
    if (open) hs(x - 90, y - r - 6, x + 90, ly + 46, 10000 + l);
  } else {
    lx = x + r + 14; ly = y - 11;
    if (open) txt(lx, ly, nm, 15, 1, global.C_text); else txt(lx, ly, nm, 15, 0, global.C_muted);
    font(15, 1, 0);
    if (st > 0) stars(lx + string_width(nm) + 34, y, 7, st);
    if (l == cur) { txt(lx + string_width(nm) + 12, ly + 1, "‹ komencu", 13, 0, global.C_main); global.def_act = 10000 + l; }
    if (open) hs(x - r - 6, y - r, global.W - global.m, y + r, 10000 + l);
  }
}
y = global.H - 56;
if (global.mpage > 0) button(global.m, y, global.m + bw(global.t_series_prev_fallback), y + 44, global.t_series_prev_fallback, 16, 0);
if (k1 < n) button(global.W - global.m - bw(global.t_series_next_fallback), y, global.W - global.m, y + 44, global.t_series_next_fallback, 15, 0);
if (global.wide) {
  if (!global.site) txt(global.m + 160, global.H - 44, "Via progreso restas en ĉi tiu retumilo.", 14, 0, global.C_muted);
  else if (global.sync_ok && global.ob_n == 0) txt(global.m + 160, global.H - 44, "Via progreso estas konservita ankaŭ en via konto ĉe la retejo.", 14, 0, global.C_muted);
  else txt(global.m + 160, global.H - 44, "Via progreso estas konservata; ĝi iros al via konto, kiam la retejo respondos.", 14, 0, global.C_muted);
}
