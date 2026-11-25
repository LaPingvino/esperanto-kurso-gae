// dock_h(fb): the dock's height — answering (fb 0), or the verdict (1 right,
// 2 wrong) with its text wrapped (draw_dock draws it so).
var h;
if (argument0 == 0) return 56 + 32;
h = 20 + 34 + th(dock_text(), dock_tw(), 17, argument0 == 2) + 18;
if (argument0 == 2) h += 22;
if (global.wide) return max(h, 56 + 40);
return h + 56 + 16;
