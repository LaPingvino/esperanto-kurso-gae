// vl_chip(x, y, label, text, x1): a clue beside Vortludo's board: the label
// muted, the text in an accent-edged chip after it (32 high).
var x, t;
txt(argument0, argument1 + 5, argument2, 16, 0, global.C_muted);
font(16, 0, 0); x = argument0 + string_width(argument2) + 10;
font(18, 1, 0); t = argument3;
if (x + string_width(t) + 24 > argument4) overflow("a clue beside Vortludo's board: " + t);
box(x, argument1, x + string_width(t) + 24, argument1 + 32, global.C_panel, global.C_main, 2);
txt(x + 12, argument1 + 4, t, 18, 1, global.C_main);
