// draw_choice(y): a word card (which meaning is this word's?) or multiple
// choice (the site's question and options).
var x, w, y, k, st, h, i;
i = global.idx;
x = global.m; w = global.W - 2 * global.m;
if (global.wide) { w = min(w, 720); x = (global.W - w) / 2; }
y = argument0;
if (global.kind == 0) {
  txt(x, y, global.t_what_means, 17, 0, global.C_muted);
  y += 34;
  txtc(global.W / 2, y, global.x_show[i], 40, 1, global.C_text);
  y += 58;
  if (global.x_seen[i] != "") txtc(global.W / 2, y, "en la teksto: " + global.x_seen[i], 15, 0, global.C_muted);
  y += 36;
} else {
  txt(x, y, global.t_choose_answer, 17, 0, global.C_muted);
  y += 34;
  y += tw(x, y, global.x_q[i], w, 22, 1, global.C_text) + 24;
}
h = 54; if (!global.wide) h = 50;
for (k = 0; k < global.x_no[i]; k += 1) {
  st = 0;
  if (global.fb != 0) { if (k == global.x_right[i]) st = 3; }
  if (global.fb == 2) { if (k == global.pick) st = 4; }
  button(x, y, x + w, y + h, string(k + 1) + ".  " + global.x_opt[i, k], 500 + k, st);
  y += h + 10;
}
