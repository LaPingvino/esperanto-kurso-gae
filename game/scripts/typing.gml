// typing(): every step. In an exercise with gaps, typed letters
// (keyboard_string) are the answer for the current gap, with the x-system;
// a lone digit picks that tile or choice (1–9); in Vortludo, the row being
// typed (vl_letters). Elsewhere typing is dropped.
var s, d;
if (global.scr == 8) { // Vortludo: the row being typed
  if (global.vl_state != 0) { keyboard_string = ""; global.typed = ""; exit; }
  if (keyboard_string == global.vl_cur) exit;
  s = vl_letters(keyboard_string);
  if (s != global.vl_cur) global.vl_msg = "";
  global.vl_cur = s; keyboard_string = s; global.typed = s;
  exit;
}
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
if (global.ngap <= 0) { keyboard_string = ""; global.typed = ""; exit; }
keyboard_string = s;
global.typed = s;
