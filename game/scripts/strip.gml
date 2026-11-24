// strip(s): s without spaces at either end.
var s;
s = argument0;
while (string_length(s) > 0 && string_char_at(s, 1) == " ") s = string_delete(s, 1, 1);
while (string_length(s) > 0 && string_char_at(s, string_length(s)) == " ") s = string_delete(s, string_length(s), 1);
return s;
