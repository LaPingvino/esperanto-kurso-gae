// typing(): every step. In an exercise with gaps, typed letters
// (keyboard_string) are the answer for the current gap, with the x-system;
// a lone digit picks that tile or choice (1–9); in Vortludo, the row being
// typed (vl_letters); in Konstrui a digit puts a piece on the line, in
// Frazotrajno a wagon behind the train; in Vortpluvo, the letters typed
// to catch a falling word (pv_letters; a space clears them).
// Elsewhere typing is dropped.
var s, d;
if (global.scr == 8) { // Vortludo: the row being typed
  if (global.vl_state != 0) { keyboard_string = ""; global.typed = ""; exit; }
  if (global.vl_wait > 0) { keyboard_string = global.vl_cur; exit; } // the row is being asked about
  if (keyboard_string == global.vl_cur) exit;
  s = vl_letters(keyboard_string);
  if (s != global.vl_cur) global.vl_msg = "";
  global.vl_cur = s; keyboard_string = s; global.typed = s;
  exit;
}
if (global.scr == 11) { // Konstrui: a lone digit puts that piece of the bank on the line (0: the tenth)
  s = keyboard_string; keyboard_string = ""; global.typed = "";
  if (global.fb == 0 && string_length(s) == 1 && string_digits(s) == s) { d = real(s); if (d == 0) d = 10; kb_place(d - 1); }
  exit;
}
if (global.scr == 13) { // Frazotrajno: a lone digit couples that wagon of the depot
  s = keyboard_string; keyboard_string = ""; global.typed = "";
  if (string_length(s) == 1 && string_digits(s) == s) { d = real(s); if (d >= 1) ft_couple(d - 1); }
  exit;
}
if (global.scr == 14) { // Vortpluvo: the letters typed
  if (global.pv_st != 1) { keyboard_string = ""; global.typed = ""; exit; }
  if (keyboard_string == global.pv_buf) exit;
  s = keyboard_string; if (string_pos(" ", s) > 0) s = "";
  pv_type(pv_letters(s));
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
