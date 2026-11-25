// dashed(x1, y1, x2, y2, colour): a dashed outline, two pixels: an empty
// slot, a tile that is done (kurso-design's tile states).
var x, y;
pen_color = argument4;
for (x = argument0 + 4; x < argument2 - 4; x += 10) {
  draw_line(x, argument1, min(x + 6, argument2 - 4), argument1); draw_line(x, argument1 + 1, min(x + 6, argument2 - 4), argument1 + 1);
  draw_line(x, argument3, min(x + 6, argument2 - 4), argument3); draw_line(x, argument3 - 1, min(x + 6, argument2 - 4), argument3 - 1);
}
for (y = argument1 + 4; y < argument3 - 4; y += 10) {
  draw_line(argument0, y, argument0, min(y + 6, argument3 - 4)); draw_line(argument0 + 1, y, argument0 + 1, min(y + 6, argument3 - 4));
  draw_line(argument2, y, argument2, min(y + 6, argument3 - 4)); draw_line(argument2 - 1, y, argument2 - 1, min(y + 6, argument3 - 4));
}
