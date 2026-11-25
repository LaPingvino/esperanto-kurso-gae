// fit(text, width, size, bold): the text cut with "…" to one line of
// width — for lists and labels that are not boxes (a box wraps instead).
var s;
s = argument0;
font(argument2, argument3, 0);
while (string_width(s) > argument1 && string_length(s) > 4) s = string_copy(s, 1, string_length(s) - 2) + "…";
return s;
