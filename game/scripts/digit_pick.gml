// digit_pick(n): the n-th choice on the screen (0-based) by its number key.
var e;
e = global.les * 100 + global.idx;
if (global.kind == 0 && argument0 < 4) act(500 + argument0);
if (global.kind == 1 && argument0 < global.f_nt[e, global.cg]) act(200 + argument0);
if (global.kind == 2 && argument0 < global.b_nt[global.les, global.idx] && !global.bused[argument0]) act(300 + argument0);
