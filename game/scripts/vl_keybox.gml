// vl_keybox(x, y, w, h, label, state, action): a key of the screen's
// keyboard, in its letter's best state so far (vl_cell's; -1 not tried).
var x, y, f, e, t, st;
x = argument0; y = argument1; st = argument5;
f = global.C_panel; e = global.C_edge; t = global.C_text;
if (st == 0) { f = global.C_sunk; e = global.C_sunk; t = global.C_muted; }
if (st == 1) { f = global.C_notebg; e = global.C_note; t = global.C_note; }
if (st == 2) { f = global.C_main; e = global.C_main; t = global.C_on; }
box(x, y, x + argument2, y + argument3, f, e, 1);
font(18, 1, t);
if (string_width(argument4) > argument2 - 4) font(14, 1, t);
draw_text(x + (argument2 - string_width(argument4)) / 2, y + (argument3 - string_height(argument4)) / 2, argument4);
if (st == 1) { brush_color = global.C_note; pen_color = global.C_note; draw_circle(x + argument2 / 2, y + argument3 - 6, 2); }
if (argument6 >= 0) hs(x, y, x + argument2, y + argument3, argument6);
