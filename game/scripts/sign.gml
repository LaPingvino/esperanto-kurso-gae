// sign(x, cy, r): the site's sign — round, as Esperanto's stations are
// (kurso-design): the flag's green with its white star.
brush_color = global.C_main; pen_color = global.C_main;
draw_circle(argument0 + argument2, argument1, argument2);
star(argument0 + argument2, argument1 + argument2 * 0.06, argument2 * 0.62, global.C_on);
