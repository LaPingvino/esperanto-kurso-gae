// lesson_open(p): place p — from the site when it plays in it (the cached
// copy at once when its version is the course's), else from the cache or
// the built-in part.
var t, i;
global.pl = argument0;
global.ser = global.p_series[argument0];
global.page = 0; global.focus = -1; global.les_fail = 0;
t = file_read_all(lesson_file(global.p_slug[argument0]));
if (t != "" && (!global.site || string_pos(chr(9) + global.course_ver + chr(9), t) > 0)) {
  if (lesson_parse(t)) { lesson_ready(); exit; }
}
if (global.site) { global.les_want = argument0; global.scr = 6; exit; }
lesson_offline_load(argument0);
