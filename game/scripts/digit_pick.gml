// digit_pick(n): the n-th choice on the screen (0-based) by its number key.
// (Nested ifs: GameMaker 5 evaluates both sides of &&.)
var k;
k = global.kind;
if (k == 0 || k == 3) { if (argument0 < global.x_no[global.idx]) act(500 + argument0); }
if (k == 1 || k == 4 || k == 5) { if (argument0 < global.x_nt[global.idx, global.cg]) act(200 + argument0); }
if (k == 2) { if (argument0 < global.x_nt[global.idx, 0]) { if (!global.bused[argument0]) act(300 + argument0); } }
