// open_from_url(): /ludo/?leciono=SLUG (the site's "Play this as a game")
// opens that place; a place the map doesn't have yet (the built-in part,
// before the site's map is in) is looked up again when it comes.
var u, p, s;
u = page_url();
p = string_pos("leciono=", u);
if (p == 0) exit;
s = string_copy(u, p + 8, string_length(u));
p = string_pos("&", s); if (p > 0) s = string_copy(s, 1, p - 1);
p = string_pos("#", s); if (p > 0) s = string_copy(s, 1, p - 1);
global.want_slug = s;
open_wanted();
