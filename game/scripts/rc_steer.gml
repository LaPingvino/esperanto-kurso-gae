// rc_steer(d): the car a lane left (-1) or right (1): the arrow keys, a
// swipe, a tap beside it (rc_tap) — all the same move.
if (global.rc_state != 1) exit;
global.rc_lane = max(0, min(2, global.rc_lane + argument0));
