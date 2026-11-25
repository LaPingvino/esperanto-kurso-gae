// sync_answer(): other-62 — the site answered (or didn't: status 0).
var st;
if (http_id() != global.sync_req) exit;
global.sync_req = -1;
st = http_status();
if (global.sync_kind == 1) {
  // The site has it (2xx), or will never take it (4xx: no such exercise):
  // off the queue either way. No answer or a server error: later.
  if ((st >= 200 && st < 300) || (st >= 400 && st < 500 && st != 408 && st != 429)) {
    ob_pop(); save_progress();
    global.sync_back = 30; global.sync_due = 1;
  } else sync_retry();
  exit;
}
if (st == 200) { sync_merge(http_result()); global.sync_back = 30; global.sync_ok = 1; }
else { global.sync_due = 1; sync_retry(); }
