// vl_pool_add(level index): that level's Vortludo words into vp[]: the
// five-letter ones, or (vl_long) the long ones with an affix.
var i, ok;
for (i = 0; i < global.nw; i += 1) {
  ok = global.w_five[i]; if (global.vl_long) ok = (global.w_aff[i] != "");
  if (ok && lv_index(global.w_level[i]) == argument0) { global.vp[global.vpn] = i; global.vpn += 1; }
}
