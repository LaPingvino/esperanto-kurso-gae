// draw_choice(y): a word card (which meaning is this word's?) or multiple
// choice (the site's question and options), each option a tile as tall as
// its text; returns the body's bottom. A pick is "trying" until Kontroli;
// after it only the right option and a wrong pick stay (the others are out
// of play, and the verdict's dock needs their room on a phone). There is no
// scrolling: the type steps down (choice_size) until it all fits above the
// dock.
var x, w, y, k, st, i, qs, os, a;
i = global.idx;
x = global.bx0; w = global.bx1 - global.bx0;
y = argument0;
for (a = 0; a < 4; a += 1) { // the largest pair of sizes that fits
  qs = 22 - 2 * a; os = 18 - a;
  if (!global.wide) { qs -= 2; os -= 1; }
  if (choice_h(y, w, qs, os) <= global.H - dock_h(global.fb)) break;
}
if (global.kind == 0) {
  txt(x, y, global.t_what_means, 17, 0, global.C_muted);
  y += 32;
  fits(global.x_show[i], w, 36, 1);
  y += tw(x, y, global.x_show[i], w, 36, 1, global.C_text) + 6;
  if (global.x_seen[i] != "") { y += tw(x, y, "en la teksto: " + global.x_seen[i], w, 15, 0, global.C_muted); }
  y += 20;
} else {
  txt(x, y, global.t_choose_answer, 17, 0, global.C_muted);
  y += 32;
  fits(global.x_q[i], w, qs, 1);
  y += tw(x, y, global.x_q[i], w, qs, 1, global.C_text) + 20;
}
for (k = 0; k < global.x_no[i]; k += 1) {
  st = choice_state(k);
  if (global.fb == 0) y += tilebox(x, y, w, global.x_opt[i, k], 500 + k, st, os, k + 1) + 10;
  else if (st != 6) y += tilebox(x, y, w, global.x_opt[i, k], -1, st, os, k + 1) + 10;
}
return y;
