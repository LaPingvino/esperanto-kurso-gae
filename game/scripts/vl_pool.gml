// vl_pool(): the Vortludo words for the learner's level into vp[] (vpn of
// them, indexes of w_*): the five-letter words in the A levels, from B1 the
// long ones with an affix (vl_long); that level's, and while there are
// fewer than ten, the levels below it, then above it (gamedata.Pool).
var li, l;
li = lv_index(level_me());
global.vl_long = (li >= 3); // gamedata.LongFrom
global.vpn = 0;
vl_pool_add(li);
for (l = li - 1; l >= 0; l -= 1) if (global.vpn < 10) vl_pool_add(l);
for (l = li + 1; l <= 6; l += 1) if (global.vpn < 10) vl_pool_add(l);
