// draw_all(): the whole screen; it also lays out what can be pressed
// (hotspots), so the mouse and keys use what was drawn last.
global.nhs = 0;
global.def_act = -1;
global.W = room_width;
global.H = room_height;
global.wide = (room_width > room_height);
// The top bar: one row (a phone's levels screen: two, for its buttons).
if (global.wide) { global.m = 40; global.hh = 64; } else { global.m = 16; global.hh = 60; if (global.scr == 5) global.hh = 108; }
fill(0, 0, global.W, global.H, global.C_bg);
if (global.scr == 5) draw_levels();
if (global.scr == 0) draw_map();
if (global.scr == 6) draw_loading();
if (global.scr == 1) draw_reading();
if (global.scr == 7) draw_video();
if (global.scr == 2) draw_exercise();
if (global.scr == 3) draw_result();
if (global.scr == 4) draw_words();
if (global.scr == 8) draw_vortludo();
if (global.scr == 9) draw_vetkuro();
if (global.scr == 10) draw_words_wait();
if (global.scr == 11) draw_konstrui();
if (global.scr == 12) draw_memoro();
draw_header(); // after the screen: the arrow keys reach the screen's own things first
if (global.focus >= global.nhs) global.focus = -1;
if (global.focus >= 0) {
  frame(global.hs_x1[global.focus] - 5, global.hs_y1[global.focus] - 5, global.hs_x2[global.focus] + 5, global.hs_y2[global.focus] + 5, global.C_focus);
}
