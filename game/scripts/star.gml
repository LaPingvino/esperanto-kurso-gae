// star(cx, cy, r, colour): a five-pointed star.
var k, a, ox, oy, ix, iy, jx, jy;
brush_color = argument3; pen_color = argument3;
for (k = 0; k < 5; k += 1) {
  a = 90 + k * 72;
  ox = argument0 + lengthdir_x(argument2, a); oy = argument1 + lengthdir_y(argument2, a);
  ix = argument0 + lengthdir_x(argument2 * 0.45, a + 36); iy = argument1 + lengthdir_y(argument2 * 0.45, a + 36);
  jx = argument0 + lengthdir_x(argument2 * 0.45, a - 36); jy = argument1 + lengthdir_y(argument2 * 0.45, a - 36);
  draw_triangle(ox, oy, ix, iy, jx, jy);
  draw_triangle(argument0, argument1, ix, iy, jx, jy);
}
