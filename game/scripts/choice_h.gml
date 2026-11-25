// choice_h(y, w, question size, option size): where draw_choice's body
// would end, measured without drawing.
var y, i, k, st;
i = global.idx;
y = argument0 + 32;
if (global.kind == 0) {
  y += th(global.x_show[i], argument1, 36, 1) + 6;
  if (global.x_seen[i] != "") y += th("en la teksto: " + global.x_seen[i], argument1, 15, 0);
  y += 20;
} else y += th(global.x_q[i], argument1, argument2, 1) + 20;
for (k = 0; k < global.x_no[i]; k += 1) {
  st = choice_state(k);
  if (global.fb == 0 || st != 6) y += tile_h(global.x_opt[i, k], argument1, argument3, k + 1) + 14;
}
return y;
