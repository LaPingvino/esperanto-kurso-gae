// lesson_offline_load(p): place p's lesson from the cache or the built-in part.
var t, i;
t = file_read_all(lesson_file(global.p_slug[argument0]));
if (t != "") { if (lesson_parse(t)) { global.les_want = -1; lesson_ready(); exit; } }
for (i = 0; i < global.fb_n; i += 1) if (global.fb_slug[i] == global.p_slug[argument0]) {
  if (lesson_parse(global.fb_text[i])) { global.les_want = -1; lesson_ready(); exit; }
}
global.les_want = argument0; global.les_fail = 1; global.scr = 6;
