// rc_step(): every step on Vetkuro: the gate comes down the road; at the
// car it is judged by the lane the car is in — the right word a point, a
// wrong one a crash (a life lost, the right word shown); three crashes end
// the race. The car slides to its lane.
var tx;
rc_geom();
tx = global.rx0 + global.lw * (global.rc_lane + 0.5);
global.rc_cx += (tx - global.rc_cx) * 0.45;
if (abs(tx - global.rc_cx) < 1) global.rc_cx = tx;
if (global.rc_state == 1) {
  global.rc_t += 1;
  if (string(AUTO) == "1") { if (global.rc_t mod 6 == 0 && global.rc_lane != global.rc_right) { if (global.rc_right > global.rc_lane) rc_steer(1); else rc_steer(-1); } } // the tests' driver
  global.rc_y += global.rc_speed; global.rc_scroll += global.rc_speed;
  if (global.ry0 + global.rc_y + 56 >= global.cy) {
    global.rc_n += 1; global.rc_hit = global.rc_lane;
    if (global.rc_lane == global.rc_right) { global.rc_score += 1; global.rc_wait = 20; sound_play(snd_right); }
    else { global.rc_lives -= 1; global.rc_crash += 1; global.rc_wait = 45; sound_play(snd_wrong); global.flash = 12; }
    global.rc_state = 2;
  }
  exit;
}
if (global.rc_state == 2) {
  global.rc_wait -= 1;
  if (global.rc_wait > 0) exit;
  if (global.rc_lives <= 0) { global.rc_state = 3; global.rc_best = max(global.rc_best, global.rc_score); global.focus = -1; }
  else rc_next();
}
