// draw_fill(y): one of the site's fill-ins: the sentence with its gaps, the
// current gap's four answers as tiles; typing works too.
var l, k, e, x, y, x0, x1, t, s, w, gw, fs, lh, g, f, ed, tc, n, tx;
l = global.les; k = global.idx; e = l * 100 + k;
x0 = global.m; x1 = global.W - global.m;
if (global.wide) { x0 = (global.W - 900) / 2; x1 = x0 + 900; }
y = argument0;
txt(x0, y, global.t_choose_answer, 17, 0, global.C_muted);
y += 36;
fs = 24; if (!global.wide) fs = 21;
lh = fs + 26;
x = x0;
for (t = 0; t < global.f_ntok[l, k]; t += 1) {
  g = global.f_tgap[e, t];
  if (g < 0) s = global.f_tok[e, t];
  else {
    s = global.fa[g];
    if (g == global.cg && global.fb == 0) s = global.typed;
    if (s == "") s = "    ";
  }
  font(fs, 0, 0);
  w = string_width(s);
  if (g >= 0) w = max(w + 16, 48);
  if (global.f_tsp[e, t]) x += string_width(" ");
  if (x + w > x1 && x > x0) { x = x0; y += lh; }
  if (g < 0) txt(x, y, s, fs, 0, global.C_text);
  else {
    f = global.C_panel; ed = global.C_line; tc = global.C_text;
    if (g == global.cg && global.fb == 0) { f = global.C_gap; ed = global.C_main; }
    if (global.fb != 0 && global.fok[g]) { f = global.C_okbg; ed = global.C_ok; tc = global.C_ok; }
    if (global.fb != 0 && !global.fok[g]) { f = global.C_badbg; ed = global.C_bad; tc = global.C_bad; }
    rfill(x, y - 6, x + w, y + fs + 10, f, ed);
    if (g == global.cg && global.fb == 0) fill(x + 6, y + fs + 5, x + w - 6, y + fs + 7, global.C_main);
    txt(x + 8, y, s, fs, 1, tc);
  }
  x += w;
}
y += lh + 24;
if (global.fb != 0) exit;
n = global.f_nt[e, global.cg];
x = x0;
for (t = 0; t < n; t += 1) {
  tx = global.f_tile[e, global.cg * 10 + t];
  w = tile_w(string(t + 1) + "  " + tx, 18);
  if (x + w > x1 && x > x0) { x = x0; y += 60; }
  tile(x, y, string(t + 1) + "  " + tx, 200 + t, 0, 18);
  x += w + 12;
}
y += 76;
button(x0, y, x0 + bw(global.t_vocab_mode_type), y + 44, global.t_vocab_mode_type, 8, 0);
button(x0 + bw(global.t_vocab_mode_type) + 12, y, x0 + bw(global.t_vocab_mode_type) + 12 + bw(global.t_dont_know), y + 44, global.t_dont_know, 7, 5);
if (global.wide) txt(x0, y + 60, "Aŭ tajpu la respondon kaj premu Enter (cx → ĉ, sx → ŝ …).", 14, 0, global.C_muted);
