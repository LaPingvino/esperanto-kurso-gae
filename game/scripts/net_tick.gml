// net_tick(): every step — the course map once at the start, and a place's
// lesson when one is wanted; one request each at a time (http_get: the
// answers come in other-62, net_answer).
if (!global.site) exit;
if (global.course_req < 0 && !global.course_tried) {
  global.course_tried = 1;
  global.course_req = http_get("/ludo-api/kurso");
}
if (global.les_want >= 0 && global.les_req < 0 && !global.les_fail) {
  global.les_req = http_get("/ludo-api/leciono/" + global.p_slug[global.les_want] + "?lingvo=" + lang_cur());
}
