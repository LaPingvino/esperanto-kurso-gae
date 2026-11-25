// net_answer(): other-62 — an answer from the site, to whichever request
// it belongs: the map, a lesson, or the sync's (sync_answer).
var t, ok;
if (http_id() == global.course_req) {
  global.course_req = -1;
  t = http_result();
  if (http_status() == 200 && t != global.course_txt) {
    if (course_parse(t)) {
      global.course_txt = t;
      if (global.ser >= global.ns) { global.ser = -1; if (global.scr == 0) global.scr = 5; }
      if (global.les_slug != "") global.pl = place_find(global.les_slug);
      open_wanted();
      file_write_all("ludo-kurso.txt", t); // the cache: last
    }
  }
  exit;
}
if (http_id() == global.les_req) {
  global.les_req = -1;
  t = http_result();
  ok = 0;
  if (http_status() == 200) ok = lesson_parse(t);
  if (ok) {
    global.les_want = -1;
    lesson_ready();
    file_write_all(lesson_file(global.les_slug), t); // the cache: last
  } else lesson_offline();
  exit;
}
sync_answer();
