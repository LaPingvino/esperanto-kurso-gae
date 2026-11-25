// build_place(t): tile t goes after the words placed so far (Kontroli
// judges the sentence once every word is placed).
if (global.bused[argument0]) exit;
global.bused[argument0] = 1;
global.bp[global.bn] = argument0;
global.bn += 1;
global.focus = -1;
