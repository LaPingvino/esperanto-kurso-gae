// pv_ready(): Vortpluvo, before the start. Too few words (a kept copy from
// before them): wait for the site's list when it is on its way.
pv_pool();
if (global.pv_pn < 4) { if (global.wd_req < 0) global.wd_fail = 1; global.scr = 10; exit; }
pv_new();
