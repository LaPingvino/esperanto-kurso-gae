// draw_all(): the whole screen; it also lays out what can be pressed
// (hotspots), so the mouse and keys use what was drawn last.
global.nhs = 0;
global.def_act = -1;
global.W = room_width;
global.H = room_height;
global.wide = (room_width > room_height);
if (global.wide) { global.m = 40; global.hh = 64; } else { global.m = 16; global.hh = 104; }
fill(0, 0, global.W, global.H, global.C_bg);
if (global.scr == 0) draw_map();
if (global.scr == 1) draw_reading();
if (global.scr == 2) draw_exercise();
if (global.scr == 3) draw_result();
if (global.scr == 4) draw_words();
draw_header(); // after the screen: the arrow keys reach the screen's own things first
if (global.focus >= global.nhs) global.focus = -1;
if (global.focus >= 0) {
  frame(global.hs_x1[global.focus] - 4, global.hs_y1[global.focus] - 4, global.hs_x2[global.focus] + 4, global.hs_y2[global.focus] + 4, global.C_focus);
}
