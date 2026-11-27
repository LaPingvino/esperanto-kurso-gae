// draw_memoro(): Memoro — twelve cards face down (the star), three across
// on a phone, six on a wide screen; a card up shows its word, or its
// picture, else its masked Esperanto definition. A pair found stays up with
// ✓, two that aren't one show ✗ until they turn back (a cue besides the
// colour each). Every card is pressable, so the arrow keys keep their
// place; a board done says how many tries it took, and Denove deals anew.
var x0, x1, y, t, cols, gap, cw, ch, c, x, cy, p, f, e, b, s, sz, th0, lv, top, pics;
x0 = global.m; x1 = global.W - global.m;
cols = 3; gap = 10; ch = 150;
if (global.wide) { cols = 6; gap = 14; ch = 150; x0 = max(x0, (global.W - 1000) / 2); x1 = global.W - x0; }
cw = (x1 - x0 - (cols - 1) * gap) / cols;
lv = level_me();
y = global.hh + 12;
txt(x0, y, "Memoro · " + lv, 17, 1, global.C_text);
t = string(global.mm_found) + "/" + string(global.mm_n) + " paroj · " + string(global.mm_tries) + " provoj";
font(15, 0, 0); txt(x1 - string_width(t), y + 2, t, 15, 0, global.C_muted);
y += 36; top = y; pics = 0;
for (c = 0; c < global.mm_nc; c += 1) {
  x = x0 + (c mod cols) * (cw + gap); cy = top + (c div cols) * (ch + gap);
  p = global.mm_cp[c];
  if (global.mm_ck[c] == 1 && global.mm_pic[p] >= 0) pics = 1;
  if (global.mm_st[c] == 0) { // face down: the green card with its star
    rfill(x, cy + 4, x + cw, cy + ch + 4, global.C_edge, global.C_edge);
    box(x, cy, x + cw, cy + ch, global.C_main, global.C_main, 1);
    star(x + cw / 2, cy + ch / 2, 18, global.C_on);
  } else {
    f = global.C_panel; e = global.C_edge; b = 2;
    if (global.mm_ck[c] == 1 && global.mm_pic[p] >= 0) f = global.C_pic;
    if (global.mm_st[c] == 1 && global.mm_b < 0) { e = global.C_main; b = 3; } // the first card up
    if (global.mm_st[c] == 1 && global.mm_b >= 0) { f = global.C_badbg; e = global.C_badedge; }
    if (global.mm_st[c] == 2) { f = global.C_okbg; e = global.C_okedge; }
    box(x, cy, x + cw, cy + ch, f, e, b);
    if (global.mm_ck[c] == 0) { // the word
      s = global.w_word[global.mm_w[p]];
      sz = 17; font(sz, 1, 0); if (string_width(s) > cw - 16) sz = 15; font(sz, 1, 0); if (string_width(s) > cw - 16) sz = 13;
      fits(s, cw - 16, sz, 1);
      th0 = th(s, cw - 16, sz, 1);
      if (th0 > ch - 12) overflow("Memoro: a word over its card");
      font(sz, 1, 0);
      if (string_width(s) <= cw - 16) txtc(x + cw / 2, cy + (ch - th0) / 2, s, sz, 1, global.C_text); // one line: centred
      else tw(x + 8, cy + (ch - th0) / 2, s, cw - 16, sz, 1, global.C_text);
    } else if (global.mm_pic[p] >= 0) draw_sprite(spr_bildo, global.mm_pic[p], x + (cw - 96) / 2, cy + (ch - 96) / 2);
    else { // the definition, the word masked
      s = global.mm_q[p];
      fits(s, cw - 12, 13, 0);
      th0 = th(s, cw - 12, 13, 0);
      if (th0 > ch - 8) overflow("Memoro: a definition over its card");
      tw(x + 6, cy + max(4, (ch - th0) / 2), s, cw - 12, 13, 0, global.C_text);
    }
    if (global.mm_st[c] == 2) mark(x + cw - 13, cy + 13, 9, 1);
    if (global.mm_st[c] == 1 && global.mm_b >= 0) mark(x + cw - 13, cy + 13, 9, 0);
  }
  hs(x, cy, x + cw, cy + ch, 800 + c);
}
y = top + ceil(global.mm_nc / cols) * (ch + gap) + 2;
if (pics) { txt(x0, y, fit("Bildoj: OpenMoji, CC BY-SA 4.0", x1 - x0, 13, 0), 13, 0, global.C_muted); }
y += 26;
if (global.mm_n > 0 && global.mm_found >= global.mm_n) { // the board done
  box(x0, y, x1, y + 72, global.C_okbg, global.C_okedge, 2);
  mark(x0 + 26, y + 36, 11, 1);
  t = "Finita! " + string(global.mm_n) + " paroj per " + string(global.mm_tries) + " provoj.";
  txt(x0 + 46, y + 25, fit(t, x1 - x0 - 60 - bw("Denove"), 16, 1), 16, 1, global.C_text);
  button(x1 - 14 - bw("Denove"), y + 14, x1 - 14, y + 58, "Denove", 749, 1);
  global.def_act = 749;
  y += 72;
}
if (y > global.H - 8) overflow("Memoro runs off the screen");
