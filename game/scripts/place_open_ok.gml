// place_open_ok(p): a series' first place, and each one after a finished one, is open.
if (argument0 == global.s_first[global.p_series[argument0]]) return 1;
return stars_get(global.p_slug[argument0 - 1]) > 0;
