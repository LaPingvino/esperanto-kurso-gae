// open_from_url(): /ludo/?leciono=SLUG (the site's "Play this lesson as a
// game") opens that lesson's text.
var u, p, s, l;
u = page_url();
p = string_pos("leciono=", u);
if (p == 0) exit;
s = string_copy(u, p + 8, string_length(u));
p = string_pos("&", s); if (p > 0) s = string_copy(s, 1, p - 1);
p = string_pos("#", s); if (p > 0) s = string_copy(s, 1, p - 1);
for (l = 0; l < global.nl; l += 1) if (global.l_slug[l] == s) { lesson_open(l); exit; }
