// pv_letters(s): s as Vortpluvo's typed letters: the x- and h-systems
// converted (xsys), small letters, only the alphabet's 28 (so the x of a
// word caught at its u, aux, drops), sixteen at most.
var s, r, k, c;
s = string_lower(xsys(argument0));
r = "";
for (k = 1; k <= string_length(s); k += 1) {
  c = string_char_at(s, k);
  if (string_pos(c, global.t_alpha) > 0 && string_length(r) < 16) r += c;
}
return r;
