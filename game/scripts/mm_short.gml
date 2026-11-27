// mm_short(text): no word of it (between spaces) longer than 12 letters — a
// card's text wraps at spaces only.
var s, p;
s = argument0 + " ";
while (s != "") {
  p = string_pos(" ", s);
  if (p > 13) return 0;
  s = string_delete(s, 1, p);
}
return 1;
