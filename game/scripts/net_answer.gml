// net_answer(): other-62 — an answer from the site, to whichever request
// it belongs: the map, a lesson, or the sync's (sync_answer).
var t, ok, ss, ws, k, old;
if (http_id() == global.course_req) {
  global.course_req = -1;
  t = http_result();
  if (http_status() == 200 && t != global.course_txt) {
    // the open series and a place on its way, kept by their slugs: their
    // indexes may differ in the new map (the built-in part's are others)
    ss = ""; ws = "";
    if (global.ser >= 0) { if (global.ser < global.ns) ss = global.s_slug[global.ser]; }
    if (global.les_want >= 0) { if (global.les_want < global.np) ws = global.p_slug[global.les_want]; }
    if (course_parse(t)) {
      global.course_txt = t;
      if (global.ser >= 0) {
        old = global.ser; global.ser = -1;
        for (k = 0; k < global.ns; k += 1) if (global.s_slug[k] == ss) global.ser = k;
        if (global.ser < 0) { if (old < global.ns) global.ser = old; }
        if (global.ser < 0) { if (global.scr == 0) global.scr = 5; }
      }
      if (ws != "") {
        global.les_want = place_find(ws);
        if (global.les_want >= 0) global.ser = global.p_series[global.les_want];
        else if (global.scr == 6) global.scr = 5; // gone from the course
      }
      if (global.les_slug != "") { global.pl = place_find(global.les_slug); if (global.pl >= 0) global.ser = global.p_series[global.pl]; }
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
