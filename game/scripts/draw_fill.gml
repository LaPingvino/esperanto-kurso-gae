// draw_fill(y): an exercise with a gap: the site's fill-in (its sentence
// with its gaps), a vocabulary word (its meaning: the word?) or a question
// with a typed answer (the phrasebook) — the current gap's tiles; typing
// works too. A filled gap can be pressed to empty it until Kontroli. Every
// box is as tall as its text; returns the body's bottom.
var i, x, y, x0, x1, t, s, w, fs, lh, g, f, ed, tc, n, tx, ntok, tg, gh, rowh, st, b, h, inner;
i = global.idx;
x0 = global.bx0; x1 = global.bx1;
y = argument0;
fs = 22; if (!global.wide) fs = 20;
lh = fs + 26;
if (global.kind == 1) { txt(x0, y, global.t_choose_answer, 17, 0, global.C_muted); y += 34; }
else {
  if (global.kind == 4) txt(x0, y, global.t_vocab_prompt, 17, 0, global.C_muted);
  else txt(x0, y, global.t_phrase_prompt, 17, 0, global.C_muted);
  y += 32;
  fits(global.x_q[i], x1 - x0, 22, 1);
  y += tw(x0, y, global.x_q[i], x1 - x0, 22, 1, global.C_text) + 18;
}
tg = min(global.cg, global.ngap - 1); // the gap typing and the tiles go to
ntok = global.x_ntok[i];
if (global.kind != 1) ntok = 1; // one gap, alone
x = x0; rowh = lh;
for (t = 0; t < ntok; t += 1) {
  if (global.kind == 1) g = global.x_tgap[i, t]; else g = 0;
  if (g < 0) s = global.x_tok[i, t];
  else {
    s = global.fa[g];
    if (g == tg && global.fb == 0 && global.typed != "") s = global.typed;
  }
  font(fs, g >= 0, 0);
  w = string_width(s);
  if (g >= 0) w = max(w + 24, 64);
  if (global.kind == 1) { if (global.x_tsp[i, t]) x += string_width(" "); }
  if (w > x1 - x0) w = x1 - x0; // a long answer: its box as wide as the line, the text wrapped in it
  if (x + w > x1 && x > x0) { x = x0; y += rowh; rowh = lh; }
  if (g < 0) txt(x, y, s, fs, 0, global.C_text);
  else {
    inner = w - 24;
    h = max(fs + 16, th(s, inner, fs, 1) + 12);
    rowh = max(rowh, h + 26);
    f = global.C_panel; ed = global.C_edge; tc = global.C_text; b = 2; st = -1;
    if (global.fb == 0) {
      if (g == tg && (global.cg < global.ngap || global.typed != "")) { ed = global.C_main; b = 3; tc = global.C_link; } // the current gap
      else if (s == "") { f = global.C_sunk; ed = global.C_sunk; }
    } else {
      if (global.fok[g]) { f = global.C_okbg; ed = global.C_okedge; st = 1; } else { f = global.C_badbg; ed = global.C_badedge; st = 0; }
    }
    box(x, y - 6, x + w, y - 6 + h, f, ed, b);
    if (g == tg && global.fb == 0 && global.cg < global.ngap) fill(x + 8, y + h - 14, x + w - 8, y + h - 12, global.C_main); // the caret line
    fits(s, inner, fs, 1);
    tw(x + 12, y, s, inner, fs, 1, tc);
    if (st >= 0) mark(x + w - 2, y - 6, 9, st); // ✓ or ✗ at its corner
    if (global.fb == 0 && global.fa[g] != "") hs(x, y - 6, x + w, y - 6 + h, 600 + g);
  }
  x += w;
}
y += rowh + 8;
if (global.fb != 0) return y;
if (global.x_hint[i] != "" && global.kind != 1) { y += tw(x0, y - 10, global.x_hint[i], x1 - x0, 15, 0, global.C_muted) + 8; }
n = global.x_nt[i, tg];
x = x0; rowh = 0;
for (t = 0; t < n; t += 1) {
  tx = global.x_tile[i, tg * 10 + t];
  w = min(tile_w(tx, 18, -(t + 1)), x1 - x0);
  if (x + w > x1 && x > x0) { x = x0; y += rowh + 10; rowh = 0; }
  st = 0;
  if (global.fa[tg] == tx) { if (global.ftyped[tg] == 0) st = 1; } // the picked one: trying
  rowh = max(rowh, tilebox(x, y, w, tx, 200 + t, st, 18, -(t + 1)));
  x += w + 10;
}
y += rowh + 14;
if (global.wide) { txt(x0, y, "Aŭ tajpu la respondon kaj premu Enter (cx → ĉ, sx → ŝ …).", 15, 0, global.C_muted); y += 24; }
return y;
