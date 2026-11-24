// build_place(t): tile t goes after the words placed so far; with the last
// one placed the sentence is judged.
var e, s, k;
if (global.bused[argument0]) exit;
global.bused[argument0] = 1;
global.bp[global.bn] = argument0;
global.bn += 1;
global.focus = -1;
if (global.bn < global.b_nt[global.les, global.idx]) exit;
ex_done(build_ok());
