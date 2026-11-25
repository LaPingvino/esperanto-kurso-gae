// draw_header(): the top bar (kurso-design's stage): the sign, and on an
// activity its progress through the place and close (×); on the levels the
// words, the meanings' language and the site; elsewhere the way back.
var x, y, w, b, l, x0, x1, by;
fill(0, 0, global.W, global.hh, global.C_panel);
fill(0, global.hh - 1, global.W, global.hh, global.C_line);
b = 44;
by = (min(global.hh, 60) - b) / 2; // the first row's buttons
if (global.wide) by = (global.hh - b) / 2;
x = global.W - global.m;
if (global.scr == 2 || global.scr == 1) { // an activity: sign, progress, n/N, ×
  sign(global.m, by + b / 2, 14);
  x0 = global.m + 40;
  if (global.wide) { txt(x0, by + 10, "Esperanto-kurso", 20, 1, global.C_link); font(20, 1, 0); x0 += string_width("Esperanto-kurso") + 24; }
  button(x - b, by, x, by + b, "", 1, 0); // close: a cross drawn (the engine's font has no ×)
  thick(x - b + 15, by + 15, x - 15, by + b - 15, 3, global.C_link);
  thick(x - b + 15, by + b - 15, x - 15, by + 15, 3, global.C_link);
  l = string(min(global.qi + 1, max(1, global.qn))) + "/" + string(max(1, global.qn));
  if (global.scr == 1) l = "";
  font(15, 1, 0); x1 = x - b - 12 - string_width(l);
  if (global.scr == 2) {
    txt(x1, by + 12, l, 15, 1, global.C_muted);
    x1 -= 12;
    if (global.wide) { x0 = max(x0, (global.W - 640) / 2); x1 = min(x1, (global.W + 640) / 2); }
    rfill(x0, by + b / 2 - 4, x1, by + b / 2 + 4, global.C_sunk, global.C_sunk);
    if (global.qi > 0) rfill(x0, by + b / 2 - 4, x0 + max(8, (x1 - x0) * global.qi / max(1, global.qn)), by + b / 2 + 4, global.C_acc, global.C_acc);
  }
  exit;
}
sign(global.m, by + b / 2, 14);
txt(global.m + 40, by + 10, "Esperanto-kurso", 20, 1, global.C_link);
font(20, 1, 0);
txt(global.m + 40 + string_width("Esperanto-kurso") + 10, by + 14, "la ludo", 15, 0, global.C_muted);
if (global.scr == 5) {
  if (!global.wide) { y = 60; x = global.W - global.m; } else y = by;
  w = bw("Retejo"); button(x - w, y, x, y + b, "Retejo", 4, 2); x -= w + 10;
  l = "Signifoj: " + string_upper(lang_cur());
  w = bw(l); button(x - w, y, x, y + b, l, 3, 2); x -= w + 10;
  w = bw("Vortoj " + string(global.nword)); button(x - w, y, x, y + b, "Vortoj " + string(global.nword), 2, 2);
} else if (global.scr == 0) {
  w = bw("‹ Niveloj"); button(x - w, by, x, by + b, "‹ Niveloj", 1, 2);
} else if (global.scr == 4) {
  w = bw("‹ Reen"); button(x - w, by, x, by + b, "‹ Reen", 1, 2);
} else {
  w = bw("‹ Mapo"); button(x - w, by, x, by + b, "‹ Mapo", 1, 2);
}
