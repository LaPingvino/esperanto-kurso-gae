// rc_pool_add(word, meaning): into Vetkuro's words, once, when it fits a
// lane (one word, as wide as a phone's gate holds) and has a meaning.
var k;
if (strip(argument1) == "" || string_pos(" ", strip(argument0)) > 0 || strip(argument0) == "") exit;
font(17, 1, 0);
if (string_width(argument0) > 96) exit; // a lane's gate on a phone: 97 px of text
for (k = 0; k < global.rp_n; k += 1) if (global.rp_word[k] == argument0 || global.rp_mean[k] == argument1) exit;
global.rp_word[global.rp_n] = argument0; global.rp_mean[global.rp_n] = argument1; global.rp_n += 1;
