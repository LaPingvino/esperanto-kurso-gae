// series_done(s): how many of series s's places are done.
var i, n;
n = 0;
for (i = global.s_first[argument0]; i < global.s_first[argument0] + global.s_np[argument0]; i += 1) if (stars_get(global.p_slug[i]) > 0) n += 1;
return n;
