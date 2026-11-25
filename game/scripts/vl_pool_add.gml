// vl_pool_add(level index): that level's five-letter words into vp[].
var i;
for (i = 0; i < global.nw; i += 1) if (global.w_five[i]) { if (lv_index(global.w_level[i]) == argument0) { global.vp[global.vpn] = i; global.vpn += 1; } }
