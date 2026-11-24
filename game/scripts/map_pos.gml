// map_pos(l): where lesson l's place is on the map (global.px, global.py).
var c, r, top, bot;
if (global.wide) {
  top = global.hh + 60; bot = global.H - 70;
  r = argument0 div 6; c = argument0 mod 6;
  if (r == 1) c = 5 - c;
  global.px = global.m + 90 + c * (global.W - 2 * global.m - 180) / 5;
  global.py = top + (bot - top) * (0.2 + r * 0.5) + (c mod 2) * 34;
} else {
  top = global.hh + 86; bot = global.H - 34;
  global.px = global.m + 34 + (argument0 mod 2) * 30;
  global.py = top + argument0 * (bot - top) / max(1, global.nl - 1);
}
