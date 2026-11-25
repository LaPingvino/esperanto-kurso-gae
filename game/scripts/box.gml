// box(x1, y1, x2, y2, fill, edge, border): a rounded box with an edge
// border pixels thick (1 is rfill's own pen).
var b;
b = argument6;
if (b <= 1) { rfill(argument0, argument1, argument2, argument3, argument4, argument5); exit; }
rfill(argument0, argument1, argument2, argument3, argument5, argument5);
rfill(argument0 + b, argument1 + b, argument2 - b, argument3 - b, argument4, argument4);
