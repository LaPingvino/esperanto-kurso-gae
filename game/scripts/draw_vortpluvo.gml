// draw_vortpluvo(): Vortpluvo — the course's words fall in the field; type
// one whole (cx or ch: ĉ, the screen's keyboard below) before it lands. A
// word the typed letters start has the accent's thick border; a word caught
// rises in the right tint with ✓, one landed stays on the ground a moment
// in the wrong tint with ✗. The lives are discs beside the score; under the
// ground, the typed letters (✗ when no falling word starts so) and the last
// word caught with its meaning. Drawn only: the step moves the words.
var x0, x1, y, t, k, x, w, f, b, e, c, by, lv, bw0, d, i;
pv_geom();
x0 = global.px0; x1 = global.px1;
// the keyboard first: its keys are the first the arrows reach
if (global.pv_st == 1) keys_screen(900, 0); else keys_screen(-1, 0);
lv = level_me();
y = global.hh + 12;
txt(x0, y, "Vortpluvo · " + lv, 17, 1, global.C_text);
t = string(global.pv_score) + " vortoj";
font(15, 0, 0); txt(x1 - 3 * 18 - 8 - string_width(t), y + 2, t, 15, 0, global.C_muted);
for (k = 0; k < 3; k += 1) { // the lives
  c = global.C_main; if (k >= global.pv_lives) c = global.C_sunk;
  brush_color = c; pen_color = global.C_main; draw_circle(x1 - 7 - (2 - k) * 18, y + 12, 7);
}
// the field and its ground
thick(x0, global.pg, x1, global.pg, 3, global.C_edge);
f = hfold(global.pv_buf); b = 0;
for (k = 0; k < global.pv_n; k += 1) {
  x = global.pv_x[k]; y = global.pv_y[k]; w = global.pv_ww[k];
  if (global.pv_buf != "" && string_copy(global.pv_f[k], 1, string_length(f)) == f) { b = 1; box(x, y, x + w, y + 40, global.C_panel, global.C_main, 3); c = global.C_main; }
  else { box(x, y, x + w, y + 40, global.C_panel, global.C_edge, 2); c = global.C_text; }
  txt(x + 14, y + 7, global.w_word[global.pv_w[k]], 20, 1, c);
  if (x < x0 - 1 || x + w > x1 + 1) overflow("Vortpluvo: a word past the field");
}
if (global.pv_fx > 0) { // caught: it rises
  d = 15 - global.pv_fx; x = global.pv_fxx; y = global.pv_fxy - 2 * d; w = global.pv_fxww;
  box(x, y, x + w, y + 40, global.C_okbg, global.C_okedge, 2);
  txt(x + 14, y + 7, global.pv_fxw, 20, 1, global.C_text);
  mark(x + w, y, 9, 1);
}
if (global.pv_lt > 0) { // landed
  x = global.pv_lx; y = global.pg - 40; w = global.pv_lww;
  box(x, y, x + w, y + 40, global.C_badbg, global.C_badedge, 2);
  txt(x + 14, y + 7, global.pv_lw, 20, 1, global.C_text);
  mark(x + w, y, 9, 0);
}
// under the ground: the typed letters, the last word caught
by = global.pb;
bw0 = min(240, (x1 - x0) / 2);
e = global.C_edge; c = global.C_text;
if (global.pv_buf != "" && !b) { e = global.C_badedge; c = global.C_bad; }
box(x0, by, x0 + bw0, by + 44, global.C_panel, e, 2);
if (global.pv_buf != "") {
  txt(x0 + 12, by + 10, fit(global.pv_buf, bw0 - 44, 18, 1), 18, 1, c);
  if (!b) mark(x0 + bw0 - 18, by + 22, 9, 0);
} else if (global.pv_st == 1) txt(x0 + 12, by + 12, fit("tajpu…", bw0 - 24, 16, 0), 16, 0, global.C_muted);
if (global.pv_last >= 0) {
  i = global.pv_last;
  txt(x0 + bw0 + 12, by + 12, fit(global.w_word[i] + " — " + global.w_mean[i], x1 - x0 - bw0 - 12, 15, 0), 15, 0, global.C_muted);
}
if (global.pv_st == 1) global.def_act = 931; // Enter: the typed letters cleared
if (global.pv_st == 0) { // before the start
  y = global.pt + 40;
  t = "Tajpu ĉiun vorton antaŭ ol ĝi falas.";
  fits(t, x1 - x0, 17, 0);
  y += tw(x0, y, t, x1 - x0, 17, 0, global.C_text) + 6;
  y += tw(x0, y, "cx aŭ ch: ĉ", x1 - x0, 15, 0, global.C_muted) + 18;
  button((x0 + x1) / 2 - bw("Komenci") / 2, y, (x0 + x1) / 2 + bw("Komenci") / 2, y + 48, "Komenci", 942, 1);
  global.def_act = 942;
}
if (global.pv_st == 2) { // the game done
  y = global.pt + 40;
  box(x0, y, x1, y + 72, global.C_okbg, global.C_okedge, 2);
  mark(x0 + 26, y + 36, 11, 1);
  t = "Finita! " + string(global.pv_score) + " vortoj · plej bone " + string(global.pv_best);
  txt(x0 + 46, y + 25, fit(t, x1 - x0 - 60 - bw("Denove"), 16, 1), 16, 1, global.C_text);
  button(x1 - 14 - bw("Denove"), y + 14, x1 - 14, y + 58, "Denove", 942, 1);
  global.def_act = 942;
}
