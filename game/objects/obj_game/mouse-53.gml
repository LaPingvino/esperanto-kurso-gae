// A click or a tap anywhere: the thing under it, as the last frame drew them.
var i;
draw_all(); // the hotspots as they stand now (drawing outside the draw event draws nothing)
for (i = global.nhs - 1; i >= 0; i -= 1) {
  if (mouse_x >= global.hs_x1[i] && mouse_x <= global.hs_x2[i] && mouse_y >= global.hs_y1[i] && mouse_y <= global.hs_y2[i]) {
    global.focus = -1; // a pointer: no keyboard ring
    act(global.hs_act[i]);
    exit;
  }
}
if (global.scr == 9) { global.rc_pressed = 1; global.rc_px = mouse_x; } // Vetkuro: a tap or a swipe, told at the release
