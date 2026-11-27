// A wide window plays in the wide room, a tall one in the tall room.
var ww, wh;
ww = window_get_width();
wh = window_get_height();
if (ww > 0 && wh > 0) {
  if (ww >= wh && room != rm_wide) { room_goto(rm_wide); exit; }
  if (ww < wh && room != rm_tall) { room_goto(rm_tall); exit; }
}
typing();
if (global.scr == 9) rc_step();
if (global.scr == 12) mm_step();
if (global.scr == 13) ft_step();
if (global.scr == 14) pv_step();
if (string(PARITY) != "1") { net_tick(); sync_tick(); }
if (global.flash > 0) global.flash -= 1;
if (global.vl_shake > 0) global.vl_shake -= 1;
if (global.vl_wait > 0) { // a guess asked of the site: two seconds, then taken
  global.vl_wait += 1;
  if (global.scr != 8 || global.vl_state != 0) { global.vl_wait = 0; global.vl_req = -1; }
  else if (global.vl_wait > 120) vl_try(global.vl_ask);
}
