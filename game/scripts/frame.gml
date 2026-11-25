// frame(x1, y1, x2, y2, colour): the keyboard's focus ring, three pixels.
var k;
pen_color = argument4;
for (k = 0; k < 3; k += 1) {
  draw_line(argument0 + k, argument1 + k, argument2 - k, argument1 + k);
  draw_line(argument0 + k, argument3 - k, argument2 - k, argument3 - k);
  draw_line(argument0 + k, argument1 + k, argument0 + k, argument3 - k);
  draw_line(argument2 - k, argument1 + k, argument2 - k, argument3 - k);
}
