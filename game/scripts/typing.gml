// typing(): every step. In a fill-in, typed letters (keyboard_string) are
// the answer for the current gap, with the x-system; a lone digit picks that
// tile or meaning (1–9). Elsewhere typing is dropped.
var s, d;
if (global.scr != 2 || global.fb != 0) { keyboard_string = ""; global.typed = ""; exit; }
if (keyboard_string == global.typed) exit;
s = xsys(keyboard_string);
if (strip(s) == "") s = "";
if (string_length(s) == 1 && string_digits(s) == s) {
  d = real(s);
  keyboard_string = ""; global.typed = "";
  if (d >= 1) digit_pick(d - 1);
  exit;
}
if (global.kind != 1) { keyboard_string = ""; global.typed = ""; exit; }
keyboard_string = s;
global.typed = s;
