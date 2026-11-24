// stars(cx, cy, r, n): three stars, n of them gold.
var k, c;
for (k = 0; k < 3; k += 1) {
  c = global.C_lock;
  if (k < argument3) c = global.C_star;
  star(argument0 + (k - 1) * argument2 * 2.3, argument1, argument2, c);
}
