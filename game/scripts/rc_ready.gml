// rc_ready(): Vetkuro, before the start: its words — a finished place's
// (the bonus after it, rc_place) when it has four or more, else the
// learner's level's as Vortludo's pool is made (the level, then below, then
// above, to twenty) — words short enough for a lane, with a meaning; the
// course's own, not those derived for Konstrui (w_drv).
var i, k, l, li;
global.rp_n = 0;
if (global.rc_place) {
  for (i = 0; i < global.nex; i += 1) {
    if (global.x_kind[i] == 4) rc_pool_add(global.x_ans[i, 0], global.x_q[i]);
    if (global.x_kind[i] == 0) rc_pool_add(global.x_show[i], global.x_opt[i, global.x_right[i]]);
  }
  if (global.rp_n < 4) global.rp_n = 0;
}
if (global.rp_n == 0) {
  li = lv_index(level_me());
  for (k = 0; k < 13; k += 1) {
    l = li; if (k > 0 && k <= li) l = li - k; if (k > li) l = k;
    if (k > 0 && global.rp_n >= 20) break;
    if (l > 6) continue;
    for (i = 0; i < global.nw; i += 1) if (lv_index(global.w_level[i]) == l && !global.w_drv[i]) rc_pool_add(global.w_word[i], global.w_mean[i]);
  }
}
if (global.rp_n < 3) { global.wd_fail = 1; global.scr = 10; exit; }
global.rc_state = 0; global.rc_lives = 3; global.rc_score = 0; global.rc_n = 0; global.rc_crash = 0;
global.rc_lane = 1; global.rc_q = -1; global.rc_t = 0; global.rc_y = -56; global.rc_hit = -1; global.rc_right = 0; global.rc_wait = 0; global.rc_speed = 0; global.rc_pressed = 0;
rc_geom(); global.rc_cx = global.rx0 + global.lw * 1.5;
global.scr = 9; global.focus = -1;
