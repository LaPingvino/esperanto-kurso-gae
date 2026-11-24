// draw_card(y): a word of the lesson; which meaning is its?
var x, w, y, k, o, st, l, h;
x = global.m; w = global.W - 2 * global.m;
if (global.wide) { w = min(w, 720); x = (global.W - w) / 2; }
y = argument0;
txt(x, y, global.t_what_means, 17, 0, global.C_muted);
y += 34;
txtc(global.W / 2, y, global.w_show[global.idx], 40, 1, global.C_text);
y += 58;
if (global.w_seen[global.idx] != "") txtc(global.W / 2, y, "en la teksto: " + global.w_seen[global.idx], 15, 0, global.C_muted);
y += 36;
h = 54; if (!global.wide) h = 50;
for (k = 0; k < 4; k += 1) {
  o = global.w_opt[global.idx, k];
  st = 0;
  if (global.fb != 0 && o == global.idx) st = 3;
  if (global.fb == 2 && k == global.pick) st = 4;
  button(x, y, x + w, y + h, string(k + 1) + ".  " + global.w_def[o, global.lang_i], 500 + k, st);
  y += h + 10;
}
