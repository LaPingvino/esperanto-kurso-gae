// sync_merge(text): the account's state (/ludo-progreso) into the game's.
// "S slug n": the game's stars kept on the account; "F slug": an exercise
// answered rightly on the site. Merged by the larger, so nothing is lost:
// a lesson takes the account's stars when they are more, and counts as done
// (one star) when every one of its fill-ins was answered rightly on the site.
var t, line, p, kind, slug, n, l, k, all, changed;
changed = 0;
t = argument0;
while (string_length(t) > 0) {
  p = string_pos(chr(10), t);
  if (p == 0) p = string_length(t) + 1;
  line = string_copy(t, 1, p - 1);
  t = string_delete(t, 1, p);
  kind = string_copy(line, 1, 2);
  line = string_delete(line, 1, 2);
  if (kind == "S ") {
    p = string_pos(" ", line);
    slug = string_copy(line, 1, p - 1);
    n = real(string_digits(string_delete(line, 1, p)));
    for (l = 0; l < global.nl; l += 1) if (global.l_slug[l] == slug && n > global.stars[l] && n <= 3) { global.stars[l] = n; changed = 1; }
  }
  if (kind == "F ") {
    for (l = 0; l < global.nl; l += 1) for (k = 0; k < global.l_nf[l]; k += 1) if (global.f_slug[l, k] == line) global.f_site[l, k] = 1;
  }
}
for (l = 0; l < global.nl; l += 1) {
  if (global.l_nf[l] == 0 || global.stars[l] > 0) continue;
  all = 1;
  for (k = 0; k < global.l_nf[l]; k += 1) if (!global.f_site[l, k]) all = 0;
  if (all) { global.stars[l] = 1; changed = 1; }
}
if (changed) { save_progress(); global.sync_due = 1; }
