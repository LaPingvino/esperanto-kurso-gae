// button(x1, y1, x2, y2, label, action, style): 0 plain, 1 main, 2 on the
// header band, 3 right, 4 wrong, 5 quiet (no edge). action < 0: not pressable.
var f, e, t;
f = global.C_panel; e = global.C_line; t = global.C_text;
if (argument6 == 1) { f = global.C_main; e = global.C_main; t = global.C_on; }
if (argument6 == 2) { f = global.C_main2; e = global.C_main2; t = global.C_on; }
if (argument6 == 3) { f = global.C_okbg; e = global.C_ok; t = global.C_ok; }
if (argument6 == 4) { f = global.C_badbg; e = global.C_bad; t = global.C_bad; }
if (argument6 == 5) { f = global.C_bg; e = global.C_bg; t = global.C_link; }
rfill(argument0, argument1, argument2, argument3, f, e);
font(global.fs, 1, t);
draw_text(argument0 + ((argument2 - argument0) - string_width(argument4)) / 2, argument1 + ((argument3 - argument1) - string_height(argument4)) / 2, argument4);
if (argument5 >= 0) hs(argument0, argument1, argument2, argument3, argument5);
