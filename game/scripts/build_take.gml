// build_take(k): the k-th placed word goes back among the tiles.
var i;
if (argument0 >= global.bn) exit;
global.bused[global.bp[argument0]] = 0;
for (i = argument0; i < global.bn - 1; i += 1) global.bp[i] = global.bp[i + 1];
global.bn -= 1;
global.focus = -1;
