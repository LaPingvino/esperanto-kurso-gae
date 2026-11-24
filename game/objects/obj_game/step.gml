// A wide window plays in the wide room, a tall one in the tall room.
var ww, wh;
ww = window_get_width();
wh = window_get_height();
if (ww > 0 && wh > 0) {
  if (ww >= wh && room != rm_wide) { room_goto(rm_wide); exit; }
  if (ww < wh && room != rm_tall) { room_goto(rm_tall); exit; }
}
typing();
if (global.flash > 0) global.flash -= 1;
