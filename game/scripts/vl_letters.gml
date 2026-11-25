// vl_letters(s): s as Vortludo letters: the x- and h-systems converted,
// small letters, only the alphabet's 28 letters, at most five.
var s, r, k, c;
s = string_lower(xsys(argument0));
r = "";
for (k = 1; k <= string_length(s); k += 1) {
  c = string_char_at(s, k);
  if (string_pos(c, global.t_alpha) > 0 && string_length(r) < 5) r += c;
}
return r;
