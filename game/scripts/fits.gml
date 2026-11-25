// fits(text, width, size, bold): every word of the text is narrower than
// width (draw_text_ext wraps at spaces only, so a longer word would run out
// of its box). Says so in global.ovf when one isn't (overflow.gml).
var s, p, w;
font(argument2, argument3, 0);
s = argument0 + " ";
while (string_length(s) > 0) {
  p = string_pos(" ", s);
  w = string_copy(s, 1, p - 1);
  if (string_width(w) > argument1) { overflow(w); return 0; }
  s = string_delete(s, 1, p);
}
return 1;
