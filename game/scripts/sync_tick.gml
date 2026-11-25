// sync_tick(): every step — the next request, when none is on its way.
var u, s, l, a;
if (!global.site || global.sync_req >= 0) exit;
if (global.sync_wait > 0) { global.sync_wait -= 1; exit; }
global.sync_t += 1;
if (global.sync_t >= 600) { global.sync_t = 0; global.sync_due = 1; } // 20 s at 30 steps a second
if (global.ob_n > 0) {
  u = "/ekzerco/" + global.ob_slug[0] + "/provo";
  a = global.ob_na[0];
  if (a == 1) global.sync_req = http_post(u, "answer", global.ob_a[0, 0]);
  if (a == 2) global.sync_req = http_post(u, "answer", global.ob_a[0, 0], "answer", global.ob_a[0, 1]);
  if (a == 3) global.sync_req = http_post(u, "answer", global.ob_a[0, 0], "answer", global.ob_a[0, 1], "answer", global.ob_a[0, 2]);
  if (a >= 4) global.sync_req = http_post(u, "answer", global.ob_a[0, 0], "answer", global.ob_a[0, 1], "answer", global.ob_a[0, 2], "answer", global.ob_a[0, 3]);
  global.sync_kind = 1;
  exit;
}
if (!global.sync_due) exit;
s = "";
for (l = 0; l < global.nl; l += 1) if (global.stars[l] > 0) { if (s != "") s += ","; s += global.l_slug[l] + "=" + string(global.stars[l]); }
global.sync_req = http_post("/ludo-progreso", "steloj", s);
global.sync_kind = 2;
global.sync_due = 0;
