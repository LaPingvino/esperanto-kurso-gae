// thick(x1, y1, x2, y2, width, colour): a line of some width.
var k, h;
pen_color = argument5;
h = (argument4 - 1) / 2;
for (k = -h; k <= h; k += 1) { draw_line(argument0 + k, argument1, argument2 + k, argument3); draw_line(argument0, argument1 + k, argument2, argument3 + k); }
