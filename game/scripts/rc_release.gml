// rc_release(): the finger (or the mouse button) up on Vetkuro: a swipe
// (30 px or more sideways) steers that way; a tap steers toward the lane
// tapped.
var dx, l;
if (!global.rc_pressed) exit;
global.rc_pressed = 0;
dx = mouse_x - global.rc_px;
if (dx >= 30) { rc_steer(1); exit; }
if (dx <= -30) { rc_steer(-1); exit; }
rc_geom();
l = floor((mouse_x - global.rx0) / global.lw);
if (l < global.rc_lane) rc_steer(-1);
if (l > global.rc_lane) rc_steer(1);
