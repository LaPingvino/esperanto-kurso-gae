// button(x1, y1, x2, y2, label, action, style): kurso-design's button ladder.
// 0 ghost (surface, edge, accent text), 1 primary (accent fill), 2 the same
// ghost in the top bar, 5 quiet (accent text, no box), 6 disabled (sunk,
// muted text; Kontroli before there is an answer). action < 0: not
// pressable. A label wider than the button wraps inside it.
var f, e, t, b, w, h;
f = global.C_panel; e = global.C_edge; t = global.C_link; b = 2;
if (argument6 == 1) { f = global.C_main; e = global.C_main; t = global.C_on; b = 1; }
if (argument6 == 6) { f = global.C_sunk; e = global.C_sunk; t = global.C_muted; b = 1; }
if (argument6 != 5) box(argument0, argument1, argument2, argument3, f, e, b);
font(global.fs, 1, 0);
w = argument2 - argument0 - 16;
if (string_width(argument4) <= w) {
  txt(argument0 + ((argument2 - argument0) - string_width(argument4)) / 2, argument1 + ((argument3 - argument1) - string_height(argument4)) / 2, argument4, global.fs, 1, t);
} else {
  fits(argument4, w, global.fs, 1);
  h = th(argument4, w, global.fs, 1);
  if (h > argument3 - argument1) overflow(argument4);
  tw(argument0 + 8, argument1 + ((argument3 - argument1) - h) / 2, argument4, w, global.fs, 1, t);
}
if (argument5 >= 0) hs(argument0, argument1, argument2, argument3, argument5);
