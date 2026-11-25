// sync_merge(text): the account's state (/ludo-progreso) into the game's,
// merged by the larger, so nothing is lost: "L lang" the account's language
// for meanings (the default until the player picks one); "S slug n" the
// game's stars kept on the account; "D slug" a place whose exercises were
// all answered rightly on the site — done, one star. ("F slug" lines, the
// exercises themselves, the site works out into D lines.)
var t, p, changed;
changed = 0;
global.ptext = argument0; global.peof = 0;
while (!global.peof) {
  t = next_line();
  if (string_length(t) < 3) continue;
  p = string_copy(t, 1, 2);
  t = string_delete(t, 1, 2);
  if (p == "L ") global.acclang = t;
  if (p == "D ") { if (stars_set(t, 1)) changed = 1; }
  if (p == "S ") {
    p = string_pos(" ", t);
    if (p > 0) { if (stars_set(string_copy(t, 1, p - 1), min(3, real(string_digits(string_delete(t, 1, p)))))) changed = 1; }
  }
}
if (changed) { save_progress(); global.sync_due = 1; }
