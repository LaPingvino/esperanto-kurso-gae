// vl_reset(): a Vortludo board empty for its word (vl_word, vl_n): no rows,
// no hint, no message, the keyboard's letters untried; the screen shown.
var k;
global.vl_row = 0; global.vl_cur = ""; global.vl_state = 0; global.vl_hint = 0; global.vl_msg = "";
global.vl_wait = 0; global.vl_req = -1; global.vl_shake = 0;
for (k = 0; k < 28; k += 1) global.vk_s[k] = -1; // each key's best state so far (-1 not tried)
keyboard_string = ""; global.typed = "";
global.scr = 8; global.focus = -1;
