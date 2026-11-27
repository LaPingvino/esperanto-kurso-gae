// pv_step(): every step on Vortpluvo: the words fall (a whole fall takes
// pv_fall steps, fewer with every word caught); one reaching the ground
// costs a life (it stays there a moment, ✗), the third lost ends the game.
// A new word every pv_t steps, sooner as the score grows, six at most.
var sp, k, f;
pv_geom();
if (global.pv_fx > 0) global.pv_fx -= 1;
if (global.pv_lt > 0) global.pv_lt -= 1;
if (global.pv_st != 1) exit;
sp = (global.pg - global.pt) / global.pv_fall;
for (k = global.pv_n - 1; k >= 0; k -= 1) {
  global.pv_y[k] += sp;
  if (global.pv_y[k] + 40 >= global.pg) {
    global.pv_lives -= 1; global.pv_lt = 40; global.flash = 12; sound_play(snd_wrong);
    global.pv_lw = global.w_word[global.pv_w[k]]; global.pv_lx = global.pv_x[k]; global.pv_lww = global.pv_ww[k];
    f = hfold(global.pv_buf);
    if (global.pv_buf != "" && string_copy(global.pv_f[k], 1, string_length(f)) == f) pv_type(""); // its start was typed: gone with it
    pv_drop_out(k);
  }
}
if (global.pv_lives <= 0) {
  global.pv_st = 2; global.pv_best = max(global.pv_best, global.pv_score); global.pv_n = 0;
  global.focus = -1; pv_type("");
  exit;
}
global.pv_t -= 1;
if (global.pv_t <= 0 && global.pv_n < 6) { pv_drop(); global.pv_t = max(36, 100 - 3 * global.pv_score); }
