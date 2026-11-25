// rc_gate(x, y, w, word, state): one lane's part of the gate: 0 coming,
// 2 the right word (judged), 3 the crash's word.
var f, e, t, h;
f = global.C_panel; e = global.C_edge; t = global.C_text;
if (argument4 == 2) { f = global.C_okbg; e = global.C_okedge; t = global.C_ok; }
if (argument4 == 3) { f = global.C_badbg; e = global.C_badedge; t = global.C_bad; }
box(argument0, argument1, argument0 + argument2, argument1 + 56, f, e, 2);
fits(argument3, argument2 - 16, 17, 1);
h = th(argument3, argument2 - 16, 17, 1);
if (h > 52) overflow(argument3);
font(17, 1, t);
draw_text_ext(argument0 + 8 + max(0, (argument2 - 16 - string_width(argument3)) / 2), argument1 + (56 - h) / 2, argument3, -1, argument2 - 16);
if (argument4 == 2) mark(argument0 + argument2 - 4, argument1 + 4, 9, 1);
if (argument4 == 3) mark(argument0 + argument2 - 4, argument1 + 4, 9, 0);
