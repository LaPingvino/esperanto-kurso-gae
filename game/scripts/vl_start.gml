// vl_start(daily): a Vortludo round — the day's word for the learner's
// level (the site's, the same for everyone of that level), or (0) a word of
// the level's pool at random. vl_n letters (five; six to nine from B1,
// with the word's affix as a clue, vl_aff).
var i, k, lv, wd;
global.vl_daily = argument0;
vl_pool();
if (global.vpn == 0) { global.wd_fail = 1; global.scr = 10; exit; }
lv = level_me(); wd = "";
if (argument0 && global.wd_fresh) { for (k = 0; k < global.nd; k += 1) if (global.d_level[k] == lv) wd = global.d_word[k]; }
global.vl_w = global.vp[floor(random(global.vpn))];
if (wd != "") { for (i = 0; i < global.nw; i += 1) if (global.w_word[i] == wd) global.vl_w = i; }
else global.vl_daily = 0;
global.vl_word = global.w_word[global.vl_w]; global.vl_n = string_length(global.vl_word); global.vl_aff = global.w_aff[global.vl_w];
global.vl_mean = global.w_mean[global.vl_w]; global.vl_def = global.w_def[global.vl_w]; // kept: a newer list may come in mid-round
global.vl_pic = -1;
if (global.w_pic[global.vl_w] != "") global.vl_pic = bildo_frame(global.w_pic[global.vl_w]);
global.vl_row = 0; global.vl_cur = ""; global.vl_state = 0; global.vl_hint = 0; global.vl_msg = "";
global.vl_wait = 0; global.vl_req = -1; global.vl_shake = 0;
for (k = 0; k < 28; k += 1) global.vk_s[k] = -1; // each key's best state so far (-1 not tried)
keyboard_string = ""; global.typed = "";
global.scr = 8; global.focus = -1;
