// draw_dock(): the stage's dock, always at the bottom (kurso-design): while
// answering, Kontroli (disabled until there is an answer) with "I don't
// know" (and typing on a phone); after Kontroli the verdict — Ĝuste! or Ne
// tute., with its ✓ or ✗ disc and the right answer — and Daŭrigi. As tall
// as its text; returns its top.
var y, x, w, s, h, b, bt, tx, tw0, top, ok, bh;
x = global.bx0; w = global.bx1 - global.bx0;
bh = 56;
if (global.fb == 0) {
  top = global.H - dock_h(0);
  fill(0, top, global.W, global.H, global.C_panel);
  fill(0, top, global.W, top + 1, global.C_line);
  y = top + 16;
  b = bw(global.t_check) + 24;
  if (ex_ready()) { button(x + w - b, y, x + w, y + bh, global.t_check, 19, 1); global.def_act = 19; }
  else button(x + w - b, y, x + w, y + bh, global.t_check, -1, 6);
  tx = x;
  if (global.kind == 1 || global.kind == 4 || global.kind == 5) {
    button(tx, y + 6, tx + bw(global.t_vocab_mode_type), y + bh - 6, global.t_vocab_mode_type, 8, 0);
    tx += bw(global.t_vocab_mode_type) + 8;
  }
  button(tx, y + 6, tx + bw(global.t_dont_know), y + bh - 6, global.t_dont_know, 7, 5);
  return top;
}
ok = (global.fb == 1);
s = dock_text();
b = bw(global.t_continue_cta) + 24;
tw0 = dock_tw();
fits(s, tw0, 17, !ok);
h = dock_h(global.fb);
top = global.H - h;
if (ok) { fill(0, top, global.W, global.H, global.C_okbg); fill(0, top, global.W, top + 3, global.C_okedge); }
else { fill(0, top, global.W, global.H, global.C_badbg); fill(0, top, global.W, top + 3, global.C_badedge); }
y = top + 20;
mark(x + 15, y + 15, 15, ok);
if (ok) {
  txt(x + 44, y, "Ĝuste!", 24, 1, global.C_ok);
  font(24, 1, 0);
  star(x + 44 + string_width("Ĝuste!") + 18, y + 12, 9, global.C_staredge); // the site's little sparkle
  star(x + 44 + string_width("Ĝuste!") + 18, y + 12, 6, global.C_star);
  y += 34;
} else {
  txt(x + 44, y, "Ne tute.", 24, 1, global.C_bad);
  y += 34;
  txt(x + 44, y, "La ĝusta respondo:", 15, 0, global.C_muted);
  y += 22;
}
y += tw(x + 44, y, s, tw0, 17, !ok, global.C_text);
bt = global.H - bh - 16;
button(x + w - b, bt, x + w, bt + bh, global.t_continue_cta, 6, 1);
global.def_act = 6;
return top;
