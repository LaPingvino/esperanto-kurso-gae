// vl_submit(): Provi — the row's letters (vl_n of them) as a guess, judged
// letter by letter (vl_mark); found, or six tries used, ends the round. Any
// letters count: the course's list is far shorter than the language.
var r, k, c, j;
if (global.vl_state != 0) exit;
if (string_length(global.vl_cur) != global.vl_n) { global.vl_msg = "Necesas " + vl_count(global.vl_n) + "."; exit; }
r = global.vl_row;
global.vl_g[r] = global.vl_cur;
vl_mark(r);
for (k = 0; k < global.vl_n; k += 1) {
  c = string_char_at(global.vl_cur, k + 1);
  j = string_pos(c, global.t_alpha) - 1;
  if (j >= 0) global.vk_s[j] = max(global.vk_s[j], global.vl_s[r * 10 + k]);
}
global.vl_row += 1; global.vl_cur = ""; keyboard_string = ""; global.typed = ""; global.vl_msg = "";
if (global.vl_g[r] == global.vl_word) { global.vl_state = 1; sound_play(snd_right); global.flash = 12; }
else if (global.vl_row >= 6) { global.vl_state = 2; sound_play(snd_wrong); }
global.focus = -1;
