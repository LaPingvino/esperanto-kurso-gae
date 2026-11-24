// tile(x, y, label, action, style, size): a tile as wide as its label; returns its width.
var w, h;
font(argument5, 1, 0);
w = string_width(argument2) + 32;
h = argument5 + 30;
if (argument4 == 6) { rfill(argument0, argument1, argument0 + w, argument1 + h, global.C_bg, global.C_line); return w; } // used: an empty place
button(argument0, argument1, argument0 + w, argument1 + h, argument2, argument3, argument4);
return w;
