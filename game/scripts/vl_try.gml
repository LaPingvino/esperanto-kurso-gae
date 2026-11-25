// vl_try(w): the guess w (vl_n letters, a word or taken as one) tried: judged
// letter by letter (vl_mark); found, or six tries used, ends the round.
var r, k, c, j;
global.vl_wait = 0; global.vl_req = -1;
r = global.vl_row;
global.vl_g[r] = argument0;
vl_mark(r);
for (k = 0; k < global.vl_n; k += 1) {
  c = string_char_at(argument0, k + 1);
  j = string_pos(c, global.t_alpha) - 1;
  if (j >= 0) global.vk_s[j] = max(global.vk_s[j], global.vl_s[r * 10 + k]);
}
global.vl_row += 1; global.vl_cur = ""; keyboard_string = ""; global.typed = ""; global.vl_msg = "";
if (global.vl_g[r] == global.vl_word) { global.vl_state = 1; sound_play(snd_right); global.flash = 12; }
else if (global.vl_row >= 6) { global.vl_state = 2; sound_play(snd_wrong); }
global.focus = -1;
