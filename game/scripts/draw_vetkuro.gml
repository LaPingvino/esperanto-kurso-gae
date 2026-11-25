// draw_vetkuro(): Vetkuro — the question card (the meaning; points and
// lives), the road of three lanes with its gate of words coming down, the
// car; before the start and after the end, a card over the road. A judged
// gate shows the right word with ✓ and a crash's word with ✗ (a cue
// besides the colour), and the card says which was right.
var x0, x1, y, k, gx, gy, st, t, l, cx, cw, ch, py, d;
rc_geom();
x0 = global.rx0; x1 = global.rx1;
y = global.hh + 12;
box(x0, y, x1, y + global.qh, global.C_panel, global.C_edge, 1);
for (k = 0; k < 3; k += 1) { // the lives: filled discs left, hollow ones lost
  if (k < global.rc_lives) { brush_color = global.C_main; pen_color = global.C_main; }
  else { brush_color = global.C_panel; pen_color = global.C_edge; }
  draw_circle(x1 - 20 - k * 22, y + 20, 7);
}
t = "Poentoj: " + string(global.rc_score);
font(15, 1, 0); txt(x1 - 80 - string_width(t), y + 11, t, 15, 1, global.C_text);
if (global.rc_state == 1 || global.rc_state == 2) {
  l = "Kiu vorto?";
  if (global.rc_state == 2) { if (global.rc_hit == global.rc_right) l = "Ĝuste!"; else l = "Ne tute: " + global.rp_word[global.rc_q]; }
  if (global.rc_state == 2 && global.rc_hit != global.rc_right) txt(x0 + 14, y + 11, fit(l, x1 - x0 - 200, 15, 1), 15, 1, global.C_bad);
  else txt(x0 + 14, y + 11, fit(l, x1 - x0 - 200, 15, 1), 15, 1, global.C_link);
  t = global.rp_mean[global.rc_q];
  while (th(t, x1 - x0 - 28, 19, 1) > global.qh - 46 && string_length(t) > 12) t = string_copy(t, 1, string_length(t) - 8) + "…";
  fits(t, x1 - x0 - 28, 19, 1);
  tw(x0 + 14, y + 38, t, x1 - x0 - 28, 19, 1, global.C_text);
} else txt(x0 + 14, y + 11, "Vetkuro", 15, 1, global.C_link);
// the road, its lane marks running with the race
fill(x0, global.ry0, x1, global.H, global.C_sunk);
fill(x0, global.ry0, x0 + 3, global.H, global.C_edge); fill(x1 - 3, global.ry0, x1, global.H, global.C_edge);
d = global.rc_scroll - floor(global.rc_scroll / 48) * 48;
for (k = 1; k < 3; k += 1) for (py = global.ry0 - 48 + d; py < global.H; py += 48) fill(x0 + k * global.lw - 2, max(global.ry0, py), x0 + k * global.lw + 2, max(global.ry0, py + 24), global.C_edge);
// the gate
if ((global.rc_state == 1 || global.rc_state == 2) && global.ry0 + global.rc_y + 56 > global.ry0) {
  gy = global.ry0 + global.rc_y;
  for (k = 0; k < 3; k += 1) {
    gx = x0 + k * global.lw + 8;
    st = 0;
    if (global.rc_state == 2 && k == global.rc_right) st = 2;
    if (global.rc_state == 2 && k == global.rc_hit && k != global.rc_right) st = 3;
    rc_gate(gx, max(global.ry0 - 56, gy), global.lw - 16, global.rp_word[global.rc_opt[k]], st);
  }
}
// the car: the accent's body, a tinted windscreen, the star on its roof
cx = global.rc_cx; cw = 46; ch = 72;
fill(cx - cw / 2 - 4, global.cy + 10, cx - cw / 2, global.cy + 26, global.C_text); fill(cx + cw / 2, global.cy + 10, cx + cw / 2 + 4, global.cy + 26, global.C_text);
fill(cx - cw / 2 - 4, global.cy + 48, cx - cw / 2, global.cy + 64, global.C_text); fill(cx + cw / 2, global.cy + 48, cx + cw / 2 + 4, global.cy + 64, global.C_text);
rfill(cx - cw / 2, global.cy, cx + cw / 2, global.cy + ch, global.C_main, global.C_main);
rfill(cx - cw / 2 + 7, global.cy + 10, cx + cw / 2 - 7, global.cy + 24, global.C_tint, global.C_tint);
star(cx, global.cy + 46, 10, global.C_on);
if (global.rc_state == 2 && global.rc_hit != global.rc_right && global.flash > 0) mark(cx + cw / 2 + 4, global.cy - 4, 11, 0);
// before the start, after the end
if (global.rc_state == 0 || global.rc_state == 3) {
  y = global.ry0 + 24;
  box(x0 + 16, y, x1 - 16, y + 250, global.C_panel, global.C_edge, 1);
  if (global.rc_state == 0) {
    txt(x0 + 32, y + 16, "Vetkuro", 22, 1, global.C_text);
    tw(x0 + 32, y + 54, "Stiru la aŭton al la vorto, kiu signifas la demandon: per la sagoklavoj, aŭ frapetu aŭ ŝovu sur la vojo. Tri eraroj finas la vetkuron.", x1 - x0 - 64, 16, 0, global.C_text);
    button(x1 - 32 - bw("Komenci"), y + 190, x1 - 32, y + 234, "Komenci", 742, 1);
  } else {
    txt(x0 + 32, y + 16, "Finita!", 22, 1, global.C_text);
    tw(x0 + 32, y + 54, string(global.rc_score) + " poentoj el " + string(global.rc_n) + " demandoj. Plej bone: " + string(global.rc_best) + ".", x1 - x0 - 64, 16, 0, global.C_text);
    button(x1 - 32 - bw("Denove"), y + 190, x1 - 32, y + 234, "Denove", 742, 1);
  }
  global.def_act = 742;
}
