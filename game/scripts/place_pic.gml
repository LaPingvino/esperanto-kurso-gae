// place_pic(l): place l's picture in spr_place / spr_place_s (ASSETS.md): a
// Zagreb lesson its own (0–11, by the lesson number in its slug), any other
// place its kind's: 12 a reading, 13 exercises, 14 a video. Add 15 for the
// same picture faded (a place not open yet).
var s, n;
s = global.p_slug[argument0];
if (string_length(s) == 15 && string_copy(s, 1, 6) == "zagr-l" && string_copy(s, 9, 7) == "-teksto") {
  n = real(string_copy(s, 7, 2));
  if (n >= 1 && n <= 12) return n - 1;
}
if (global.p_kind[argument0] == "V") return 14;
if (global.p_kind[argument0] == "E") return 13;
return 12;
