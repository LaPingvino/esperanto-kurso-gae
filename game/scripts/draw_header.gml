// draw_header(): the green band: the name, and the map's buttons (words,
// the definitions' language, the site) or the way back to the map.
var x, y, w, b, n, i;
fill(0, 0, global.W, global.hh, global.C_main);
if (global.wide) { txt(global.m, 16, "Esperanto-kurso", 24, 1, global.C_on); font(24, 1, 0); txt(global.m + string_width("Esperanto-kurso") + 12, 23, "la ludo", 16, 0, global.C_on); y = 12; }
else { txt(global.m, 12, "Esperanto-kurso", 20, 1, global.C_on); font(20, 1, 0); txt(global.m + string_width("Esperanto-kurso") + 10, 17, "la ludo", 14, 0, global.C_on); y = 54; }
x = global.W - global.m;
b = 40;
if (global.scr == 0) {
  n = 0;
  for (i = 0; i < global.nwords; i += 1) if (global.stars[global.w_lesson[i]] > 0) n += 1;
  w = bw("Retejo"); button(x - w, y, x, y + b, "Retejo", 4, 2); x -= w + 10;
  w = bw("Signifoj: " + string_upper(global.lang[global.lang_i])); button(x - w, y, x, y + b, "Signifoj: " + string_upper(global.lang[global.lang_i]), 3, 2); x -= w + 10;
  w = bw("Vortoj " + string(n)); button(x - w, y, x, y + b, "Vortoj " + string(n), 2, 2);
} else {
  w = bw("‹ Mapo"); button(x - w, y, x, y + b, "‹ Mapo", 1, 2);
}
