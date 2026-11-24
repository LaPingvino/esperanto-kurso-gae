// frame(x1, y1, x2, y2, colour): an outline, two pixels.
pen_color = argument4;
draw_line(argument0, argument1, argument2, argument1);
draw_line(argument0, argument3, argument2, argument3);
draw_line(argument0, argument1, argument0, argument3);
draw_line(argument2, argument1, argument2, argument3);
draw_line(argument0 + 1, argument1 + 1, argument2 - 1, argument1 + 1);
draw_line(argument0 + 1, argument3 - 1, argument2 - 1, argument3 - 1);
draw_line(argument0 + 1, argument1 + 1, argument0 + 1, argument3 - 1);
draw_line(argument2 - 1, argument1 + 1, argument2 - 1, argument3 - 1);
