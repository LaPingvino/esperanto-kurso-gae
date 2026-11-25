// draw_header(): the green band: the name, and on the levels the words,
// the meanings' language and the site; elsewhere the way back.
var x, y, w, b, l;
fill(0, 0, global.W, global.hh, global.C_main);
if (global.wide) { txt(global.m, 16, "Esperanto-kurso", 24, 1, global.C_on); font(24, 1, 0); txt(global.m + string_width("Esperanto-kurso") + 12, 23, "la ludo", 16, 0, global.C_on); y = 12; }
else { txt(global.m, 12, "Esperanto-kurso", 20, 1, global.C_on); font(20, 1, 0); txt(global.m + string_width("Esperanto-kurso") + 10, 17, "la ludo", 14, 0, global.C_on); y = 54; }
x = global.W - global.m;
b = 40;
if (global.scr == 5) {
  w = bw("Retejo"); button(x - w, y, x, y + b, "Retejo", 4, 2); x -= w + 10;
  l = "Signifoj: " + string_upper(lang_cur());
  w = bw(l); button(x - w, y, x, y + b, l, 3, 2); x -= w + 10;
  w = bw("Vortoj " + string(global.nword)); button(x - w, y, x, y + b, "Vortoj " + string(global.nword), 2, 2);
} else if (global.scr == 0) {
  w = bw("‹ Niveloj"); button(x - w, y, x, y + b, "‹ Niveloj", 1, 2);
} else {
  w = bw("‹ Mapo"); button(x - w, y, x, y + b, "‹ Mapo", 1, 2);
}
