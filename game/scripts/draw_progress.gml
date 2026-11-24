// draw_progress(y): the lesson's exercises done so far, as a bar.
var x, w;
x = global.m; w = global.W - 2 * global.m;
txt(x, argument0, global.l_title[global.les], 14, 0, global.C_muted);
fill(x, argument0 + 24, x + w, argument0 + 31, global.C_lock);
fill(x, argument0 + 24, x + w * global.qi / max(1, global.qn), argument0 + 31, global.C_main);
