// lesson_done(): stars by the first tries (a text to read, a video: one),
// the words of its cards into the collection, saved, synced.
var r, s, i, k, have;
s = 1;
if (global.qtotal > 0) {
  r = global.qfirst / global.qtotal;
  if (r >= 0.6) s = 2;
  if (r >= 0.9) s = 3;
}
global.got = s;
stars_set(global.les_slug, s);
for (i = 0; i < global.nex; i += 1) if (global.x_kind[i] == 0) {
  have = 0;
  for (k = 0; k < global.nword; k += 1) if (global.word_show[k] == global.x_show[i]) have = 1;
  if (!have) { global.word_show[global.nword] = global.x_show[i]; global.word_mean[global.nword] = global.x_opt[i, global.x_right[i]]; global.nword += 1; }
}
save_progress();
global.sync_due = 1;
global.scr = 3; global.focus = -1;
