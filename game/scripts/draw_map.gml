// draw_map(): the lessons as places on a path; each finished one opens the next.
var l, x, y, x2, y2, k, c, r, open, lx, ly, nm, cur;
txt(global.m, global.hh + 18, global.course_name + " — " + string(global.nl) + " lecionoj", 18, 1, global.C_text);
if (global.wide) txt(global.m, global.H - 40, "Ĉiu finita leciono malfermas la sekvan. Via progreso restas en ĉi tiu retumilo.", 14, 0, global.C_muted);
r = 26; if (!global.wide) r = 22;
cur = -1;
for (l = 0; l < global.nl; l += 1) if (cur < 0 && lesson_open_ok(l) && global.stars[l] == 0) cur = l;
for (l = 0; l < global.nl - 1; l += 1) { // the path
  map_pos(l); x = global.px; y = global.py;
  map_pos(l + 1); x2 = global.px; y2 = global.py;
  c = global.C_path;
  if (lesson_open_ok(l + 1)) c = global.C_main;
  for (k = -2; k <= 2; k += 1) { pen_color = c; draw_line(x + k, y, x2 + k, y2); draw_line(x, y + k, x2, y2 + k); }
}
for (l = 0; l < global.nl; l += 1) {
  map_pos(l); x = global.px; y = global.py;
  open = lesson_open_ok(l);
  nm = lesson_name(l);
  if (global.stars[l] > 0) { brush_color = global.C_main; pen_color = global.C_main; draw_circle(x, y, r); txtc(x, y - 12, string(l + 1), 18, 1, global.C_on); }
  else if (open) {
    brush_color = global.C_main; pen_color = global.C_main; draw_circle(x, y, r + 4);
    brush_color = global.C_panel; pen_color = global.C_panel; draw_circle(x, y, r - 1);
    txtc(x, y - 12, string(l + 1), 18, 1, global.C_main);
  } else { brush_color = global.C_line; pen_color = global.C_line; draw_circle(x, y, r); brush_color = global.C_lock; pen_color = global.C_lock; draw_circle(x, y, r - 2); txtc(x, y - 12, string(l + 1), 18, 1, global.C_muted); }
  if (global.wide) {
    lx = x; ly = y + r + 10;
    if (open) txtc(lx, ly, nm, 15, 1, global.C_text); else txtc(lx, ly, nm, 15, 0, global.C_muted);
    if (global.stars[l] > 0) stars(lx, ly + 34, 8, global.stars[l]);
    if (l == cur) { txtc(lx, ly + 24, "Komencu ĉi tie", 13, 0, global.C_main); global.def_act = 100 + l; }
    if (open) hs(x - 90, y - r - 6, x + 90, ly + 46, 100 + l);
  } else {
    lx = x + r + 14; ly = y - 11;
    if (open) txt(lx, ly, nm, 15, 1, global.C_text); else txt(lx, ly, nm, 15, 0, global.C_muted);
    font(15, 1, 0);
    if (global.stars[l] > 0) stars(lx + string_width(nm) + 34, y, 7, global.stars[l]);
    if (l == cur) { txt(lx + string_width(nm) + 12, ly + 1, "‹ komencu", 13, 0, global.C_main); global.def_act = 100 + l; }
    if (open) hs(x - r - 6, y - r, global.W - global.m, y + r, 100 + l);
  }
}
