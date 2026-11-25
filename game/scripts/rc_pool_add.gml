// rc_pool_add(word, meaning): into Vetkuro's words, once, when it fits a
// lane (one word, as wide as a phone's gate holds) and has a meaning. Its
// question goes by the word itself, not by the learner's language: the
// word's picture where it has one (rp_pic, a frame of spr_bildo), else its
// Esperanto definition with the word masked, else (rp_tr) the meaning,
// marked with its language ("(en) car"). Two words asking the same (one
// picture, one text) can't both be in: the gate would have two right lanes.
var k, i, pic, q, tr;
if (strip(argument1) == "" || string_pos(" ", strip(argument0)) > 0 || strip(argument0) == "") exit;
font(17, 1, 0);
if (string_width(argument0) > 96) exit; // a lane's gate on a phone: 97 px of text
pic = -1; q = ""; tr = 0;
for (i = 0; i < global.nw; i += 1) if (global.w_word[i] == argument0) {
  if (global.w_pic[i] != "") pic = bildo_frame(global.w_pic[i]);
  q = global.w_def[i];
  break;
}
if (pic >= 0) q = "";
if (pic < 0 && q == "") {
  q = argument1; tr = 1;
  if (!(string_char_at(q, 1) == "(" && string_pos(") ", q) > 2 && string_pos(") ", q) <= 5)) q = "(" + lang_cur() + ") " + q; // Meaning marks a fallback language itself
}
for (k = 0; k < global.rp_n; k += 1) {
  if (global.rp_word[k] == argument0) exit;
  if (pic >= 0 && global.rp_pic[k] == pic) exit;
  if (pic < 0 && global.rp_q[k] == q) exit;
}
global.rp_word[global.rp_n] = argument0; global.rp_mean[global.rp_n] = argument1;
global.rp_pic[global.rp_n] = pic; global.rp_q[global.rp_n] = q; global.rp_tr[global.rp_n] = tr;
global.rp_n += 1;
