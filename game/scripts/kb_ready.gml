// kb_ready(): Konstrui, a word built from its pieces (root, affixes,
// ending), the round's tally from nothing. No words with pieces yet (a kept
// copy from before them): wait for the site's list when it is on its way.
kb_pool();
if (global.kb_pn == 0) { if (global.wd_req < 0) global.wd_fail = 1; global.scr = 10; exit; }
global.kb_ok = 0; global.kb_tries = 0; global.kb_w = -1;
kb_next();
