// lesson_open(l): lesson l's text, cut into its paragraphs.
var t, p;
global.les = argument0;
global.np = 0;
t = global.l_text[argument0] + "#";
while (string_length(t) > 0) {
  p = string_pos("#", t);
  if (p > 1) { global.para[global.np] = string_copy(t, 1, p - 1); global.np += 1; }
  t = string_delete(t, 1, p);
}
global.page = 0;
global.scr = 1;
global.focus = -1;
