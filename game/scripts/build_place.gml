// build_place(t): tile t goes after the words placed so far; with the last
// one placed the sentence is judged.
if (global.bused[argument0]) exit;
global.bused[argument0] = 1;
global.bp[global.bn] = argument0;
global.bn += 1;
global.focus = -1;
if (global.bn < global.x_nt[global.idx, 0]) exit;
ex_done(build_ok());
