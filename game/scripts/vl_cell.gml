// vl_cell(x, y, size, letter, state): one cell of the grid. States: -2 a
// row to come, -1 the row being typed, 0 not in the word, 1 elsewhere in it
// (a dot under the letter), 2 in its place (accent fill).
var x, y, c, l, st, f, e, t, z;
x = argument0; y = argument1; c = argument2; l = string_upper(argument3); st = argument4;
f = global.C_panel; e = global.C_line; t = global.C_text;
if (st == -1) { e = global.C_edge; if (l != "") e = global.C_text; }
if (st == 0) { f = global.C_sunk; e = global.C_sunk; t = global.C_muted; }
if (st == 1) { f = global.C_notebg; e = global.C_note; t = global.C_note; }
if (st == 2) { f = global.C_main; e = global.C_main; t = global.C_on; }
box(x, y, x + c, y + c, f, e, 2);
z = min(24, floor(c / 2)); // smaller cells (a long word on a phone): a smaller letter
if (l != "") txtc(x + c / 2, y + c / 2 - z * 5 / 6, l, z, 1, t);
if (st == 1) { brush_color = global.C_note; pen_color = global.C_note; draw_circle(x + c / 2, y + c - 5, 3); }
