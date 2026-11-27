// draw_vortludo(): Vortludo (a Wordle on the course's words): six rows of
// the word's cells (five; six to nine from B1, their size fitted to the
// screen), with a long word's affix beside the board (never in its place:
// it may sit anywhere in the word), the screen's keyboard of the 28 letters, and a
// panel (below the grid on a phone, beside it when wide) with the message,
// the hint after three misses (the word's picture, its Esperanto
// definition, else its meaning) and the round's buttons. Each letter's
// state has a cue besides its colour: in its place, the accent fill (white
// letter); elsewhere in the word, the note tint with a dot under it; not in
// it, sunk with the muted letter (kurso-design's checked pairs, 7:1 and up).
var sx, x0, x1, y, c, g, gw, gh, gx, gy, r, k, l, st, kt, n, x, px0, py0, px1, py1, by, t, lv, h, a, pend;
x0 = global.m; x1 = global.W - global.m;
lv = level_me();
y = global.hh + 12;
t = "la vorto de la tago"; if (!global.vl_daily) t = "libera ludo";
txt(x0, y, fit("Vortludo · " + t + " · " + lv, x1 - x0, 17, 1), 17, 1, global.C_text);
y += 34;
c = 52; g = 6; if (global.wide) c = 50;
n = global.vl_n;
if (global.wide) c = min(c, floor((global.W / 2 - 30 - x0 - (n - 1) * g) / n));
else c = min(c, floor((x1 - x0 - (n - 1) * g) / n));
gw = n * c + (n - 1) * g; gh = 6 * c + 5 * g;
gx = (global.W - gw) / 2; if (global.wide) gx = global.W / 2 - gw - 30;
gy = y;
for (r = 0; r < 6; r += 1) for (k = 0; k < n; k += 1) {
  l = ""; st = -2;
  if (r < global.vl_row) { l = string_char_at(global.vl_g[r], k + 1); st = global.vl_s[r * 10 + k]; }
  else if (r == global.vl_row && global.vl_state == 0) { l = string_char_at(global.vl_cur, k + 1); st = -1; }
  sx = 0; if (r == global.vl_row && global.vl_shake > 0) sx = round(sin(global.vl_shake * 1.3) * global.vl_shake * 0.6); // no word: the row shakes
  vl_cell(gx + k * (c + g) + sx, gy + r * (c + g), c, l, st);
}
// the keyboard (a–h, ĥ–p, then Viŝi and r–z), its letters in their states
kt = keys_screen(700 - 701 * (global.vl_state != 0), 1);
// the panel
px0 = x0; py0 = gy + gh + 12; px1 = x1; py1 = kt - 12;
if (global.wide) { px0 = global.W / 2 + 10; py0 = gy; px1 = min(x1, px0 + 520); py1 = gy + gh; }
by = py1 - 44;
y = py0;
if (global.vl_aff != "") { // the clue: the affix alone, beside the board
  t = "Afikso en la vorto:";
  txt(px0, y + 5, t, 16, 0, global.C_muted);
  font(16, 0, 0); x = px0 + string_width(t) + 10;
  font(18, 1, 0); box(x, y, x + string_width(global.vl_aff) + 24, y + 32, global.C_panel, global.C_main, 2);
  txt(x + 12, y + 4, global.vl_aff, 18, 1, global.C_main);
  y += 42;
}
if (global.vl_state == 0) {
  if (global.vl_msg != "") { txt(px0, y, fit(global.vl_msg, px1 - px0, 16, 1), 16, 1, global.C_bad); y += 26; }
  else if (global.vl_row >= 3 && global.vl_hint == 0) { txt(px0, y, fit("Tri provoj: ĉu iom da helpo?", px1 - px0, 16, 0), 16, 0, global.C_muted); y += 26; }
  else if (global.vl_row == 0) { txt(px0, y, fit("Tajpu vorton de " + vl_count(global.vl_n) + " (cx aŭ ch: ĉ).", px1 - px0, 16, 0), 16, 0, global.C_muted); y += 26; }
} else {
  if (global.vl_state == 1) { mark(px0 + 12, y + 12, 12, 1); txt(px0 + 32, y, "Ĝuste!", 20, 1, global.C_ok); }
  else { mark(px0 + 12, y + 12, 12, 0); txt(px0 + 32, y, "Ne tute.", 20, 1, global.C_bad); }
  y += 30;
  txt(px0, y, fit(global.vl_word + " — " + global.vl_mean, px1 - px0, 17, 1), 17, 1, global.C_text);
  y += 30;
}
if (global.vl_hint == 1) {
  box(px0, y, px0 + 108, y + 108, global.C_pic, global.C_edge, 1);
  draw_sprite(spr_bildo, global.vl_pic, px0 + 6, y + 6);
  txt(px0 + 120, y + 84, fit("Bildo: OpenMoji, CC BY-SA 4.0", px1 - px0 - 120, 13, 0), 13, 0, global.C_muted);
  y += 118;
}
if (global.vl_hint == 2 || global.vl_hint == 3) {
  t = global.vl_def; if (global.vl_hint == 3) t = global.vl_mean;
  h = by - 10 - y;
  while (th(t, px1 - px0, 16, 0) > h && string_length(t) > 12) t = string_copy(t, 1, string_length(t) - 12) + "…";
  fits(t, px1 - px0, 16, 0);
  y += tw(px0, y, t, px1 - px0, 16, 0, global.C_text) + 10;
}
if (y > by) overflow("Vortludo's panel");
// the buttons: the hints, Provi; after the round, another word
x = px0;
if (global.vl_state == 0) {
  pend = (global.vl_row >= 3);
  if (pend && global.vl_pic >= 0 && global.vl_hint != 1) { button(x, by, x + bw("Bildo"), by + 44, "Bildo", 732, 0); x += bw("Bildo") + 8; }
  if (pend && global.vl_def != "" && global.vl_hint != 2) { button(x, by, x + bw("Difino"), by + 44, "Difino", 733, 0); x += bw("Difino") + 8; }
  if (pend && global.vl_pic < 0 && global.vl_def == "" && global.vl_hint != 3) { button(x, by, x + bw("Signifo"), by + 44, "Signifo", 734, 0); x += bw("Signifo") + 8; }
  if (string_length(global.vl_cur) == global.vl_n) button(px1 - bw("Provi"), by, px1, by + 44, "Provi", 731, 1);
  else button(px1 - bw("Provi"), by, px1, by + 44, "Provi", -1, 6);
  if (x > px1 - bw("Provi")) overflow("Vortludo's buttons");
  global.def_act = 731;
} else {
  button(px1 - bw("Nova vorto"), by, px1, by + 44, "Nova vorto", 735, 1);
  if (!global.vl_daily) { t = "La vorto de la tago"; if (px1 - px0 < bw(t) + bw("Nova vorto") + 8) t = "Hodiaŭa"; button(px0, by, px0 + bw(t), by + 44, t, 736, 0); }
  global.def_act = 735;
}
