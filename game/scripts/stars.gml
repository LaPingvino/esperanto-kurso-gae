// stars(cx, cy, r, n): three stars, n of them earned: gold with the star
// edge round it (3:1 on the paper); the others sunk with the control edge.
var k, cx;
for (k = 0; k < 3; k += 1) {
  cx = argument0 + (k - 1) * argument2 * 2.3;
  if (k < argument3) { star(cx, argument1, argument2, global.C_staredge); star(cx, argument1, argument2 * 0.72, global.C_star); }
  else { star(cx, argument1, argument2, global.C_edge); star(cx, argument1, argument2 * 0.68, global.C_sunk); }
}
