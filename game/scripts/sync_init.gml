// sync_init(): the game's link with the site's account, when it plays in a
// page (served by the site at /ludo/; elsewhere it keeps its progress only
// in its own file). Each visitor of the site has an account (anonymous
// until they sign in), and the page's requests carry it (same-origin
// cookies). Two kinds of request, one at a time, in the background
// (http_post's answer comes in other-62, sync_answer):
//   1. typed answers, queued in the progress file and sent one by one to the
//      site's own POST /ekzerco/{slug}/provo — kept until the site has them;
//   2. the stars, POST /ludo-progreso: sent, merged there by the larger per
//      lesson, and the account's state comes back (sync_merge).
// No answer (offline) or a server error: tried again later, waiting longer
// each time (1 s, 2 s … at most a minute). Every 20 s on its own, and after
// each finished lesson, the stars are synced again, so progress made on the
// site in another tab shows up.
var l, k;
global.site = (page_url() != "");
global.sync_req = -1;   // the request on its way (-1: none)
global.sync_kind = 0;   // 1 an answer, 2 the stars
global.sync_wait = 0;   // steps to wait before trying again
global.sync_back = 30;  // the next wait after a failure
global.sync_due = 1;    // the stars want syncing
global.sync_t = 0;
global.sync_ok = 0;     // the last sync went through
for (l = 0; l < global.nl; l += 1) for (k = 0; k < global.l_nf[l]; k += 1) global.f_site[l, k] = 0;
