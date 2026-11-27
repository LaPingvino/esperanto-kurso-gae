// mm_ready(): Memoro, a board of pairs from the pool. Too few words (a kept
// copy from before them): wait for the site's list when it is on its way.
mm_pool();
if (global.mm_pn < 3) { if (global.wd_req < 0) global.wd_fail = 1; global.scr = 10; exit; }
mm_deal();
