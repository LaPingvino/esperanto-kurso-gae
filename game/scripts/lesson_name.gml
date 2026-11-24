// lesson_name(l): the title without "Leciono N: ".
var t, p;
t = global.l_title[argument0];
p = string_pos(": ", t);
if (p > 0) return string_copy(t, p + 2, string_length(t));
return t;
