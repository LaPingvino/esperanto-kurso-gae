// draw_map(): a series' places on a line, twelve to a page, as
// kurso-design's course map: done (filled, its number), here (ringed, "Vi
// estas ĉi tie"), ahead (hollow). Each finished place opens the next.
var s, f, n, l, k, x, y, x2, y2, c, r, open, lx, ly, nm, cur, st, k0, k1, pic, e, cl;
if (global.wide) draw_sprite(spr_compass, 0, global.W - global.m - 60, global.hh + 14);
s = global.ser;
f = global.s_first[s]; n = global.s_np[s];
font(15, 1, 0); cl = string_width(global.s_level[s]) + 24;
rfill(global.m, global.hh + 18, global.m + cl, global.hh + 46, global.C_tint, global.C_tint);
txt(global.m + 12, global.hh + 23, global.s_level[s], 15, 1, global.C_link);
txt(global.m + cl + 12, global.hh + 20, fit(global.s_label[s], global.W - 3 * global.m - cl - 80 * global.wide, 18, 1), 18, 1, global.C_text);
r = 26; if (!global.wide) r = 22;
cur = -1;
for (l = f; l < f + n; l += 1) { if (cur < 0) { if (stars_get(global.p_slug[l]) == 0) { if (place_open_ok(l)) cur = l; } } }
if (global.mpage * 12 >= n) global.mpage = 0;
k0 = global.mpage * 12; k1 = min(n, k0 + 12);
for (k = k0; k < k1 - 1; k += 1) { // the line: the accent up to the open places, the edge colour on
  map_pos(k - k0); x = global.px; y = global.py;
  map_pos(k + 1 - k0); x2 = global.px; y2 = global.py;
  if (place_open_ok(f + k + 1)) thick(x, y, x2, y2, 6, global.C_acc);
  else thick(x, y, x2, y2, 2, global.C_edge);
}
for (k = k0; k < k1; k += 1) {
  l = f + k;
  map_pos(k - k0); x = global.px; y = global.py;
  open = place_open_ok(l);
  st = stars_get(global.p_slug[l]);
  nm = global.p_title[l];
  pic = place_pic(l) + 15 * (!open); // its picture, faded until it opens
  if (global.wide) {
    if (k - k0 == 6) draw_sprite(spr_place, pic, x + r + 10, y - 22); // beside it: the line comes down from above
    else draw_sprite(spr_place, pic, x - 22, y - r - 52);
  }
  else draw_sprite(spr_place_s, pic, global.W - global.m - 28, y - 14); // at the row's end
  if (st > 0) { // done
    brush_color = global.C_main; pen_color = global.C_main; draw_circle(x, y, r);
    txtc(x, y - 12, string(k + 1), 18, 1, global.C_on);
  } else if (open) { // here, or open
    brush_color = global.C_main; pen_color = global.C_main; draw_circle(x, y, r + 5);
    brush_color = global.C_panel; pen_color = global.C_panel; draw_circle(x, y, r);
    txtc(x, y - 12, string(k + 1), 18, 1, global.C_link);
  } else { // ahead: hollow
    brush_color = global.C_edge; pen_color = global.C_edge; draw_circle(x, y, r);
    brush_color = global.C_panel; pen_color = global.C_panel; draw_circle(x, y, r - 2);
    txtc(x, y - 12, string(k + 1), 18, 0, global.C_muted);
  }
  if (global.wide) {
    lx = x; ly = y + r + 10;
    font(15, 1, 0);
    nm = fit(nm, 176, 15, open);
    if (open) txtc(lx, ly, nm, 15, 1, global.C_text); else txtc(lx, ly, nm, 15, 0, global.C_muted);
    if (st > 0) stars(lx, ly + 36, 8, st);
    if (l == cur) { txtc(lx, ly + 24, "Vi estas ĉi tie", 13, 1, global.C_link); global.def_act = 10000 + l; }
    if (open) hs(x - 90, y - r - 6, x + 90, ly + 46, 10000 + l);
  } else {
    lx = x + r + 14; ly = y - 11;
    e = 0; // what follows the name on its row, kept clear of the picture at the row's end
    if (st > 0) e = 62;
    else if (l == cur) { font(13, 1, 0); e = 12 + string_width("‹ vi estas ĉi tie"); }
    nm = fit(nm, global.W - global.m - 42 - lx - e, 15, open);
    if (open) txt(lx, ly, nm, 15, 1, global.C_text); else txt(lx, ly, nm, 15, 0, global.C_muted);
    font(15, open, 0);
    if (st > 0) stars(lx + string_width(nm) + 34, y, 7, st);
    if (l == cur) { txt(lx + string_width(nm) + 12, ly + 2, "‹ vi estas ĉi tie", 13, 1, global.C_link); global.def_act = 10000 + l; }
    if (open) hs(x - r - 6, y - r, global.W - global.m, y + r, 10000 + l);
  }
}
y = global.H - 58;
if (global.mpage > 0) button(global.m, y, global.m + bw(global.t_series_prev_fallback), y + 44, global.t_series_prev_fallback, 16, 0);
if (k1 < n) button(global.W - global.m - bw(global.t_series_next_fallback), y, global.W - global.m, y + 44, global.t_series_next_fallback, 15, 0);
if (global.wide) {
  if (!global.site) txt(global.m + 160, global.H - 46, "Via progreso restas en ĉi tiu retumilo.", 15, 0, global.C_muted);
  else if (global.sync_ok && global.ob_n == 0) txt(global.m + 160, global.H - 46, "Via progreso estas konservita ankaŭ en via konto ĉe la retejo.", 15, 0, global.C_muted);
  else txt(global.m + 160, global.H - 46, "Via progreso estas konservata; ĝi iros al via konto, kiam la retejo respondos.", 15, 0, global.C_muted);
}
