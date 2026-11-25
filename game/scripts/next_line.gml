// next_line(): the next line of global.ptext (consumed); "" at the end
// (global.peof then 1). The text parsers read through it.
var p, l;
if (string_length(global.ptext) == 0) { global.peof = 1; return ""; }
p = string_pos(chr(10), global.ptext);
if (p == 0) { l = global.ptext; global.ptext = ""; return l; }
l = string_copy(global.ptext, 1, p - 1);
global.ptext = string_delete(global.ptext, 1, p);
return l;
