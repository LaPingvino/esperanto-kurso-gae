// open_wanted(): the place the page asked for, once the map has it.
var p;
if (global.want_slug == "") exit;
p = place_find(global.want_slug);
if (p < 0) exit;
global.want_slug = "";
lesson_open(p);
