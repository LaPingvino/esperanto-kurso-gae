// pv_pool(): Vortpluvo's words into pv_p[] (pv_pn of them, indexes of w_*).
// After a place (pv_place), that place's words when four or more can fall;
// else the learner's level's and, while fewer than twenty, the levels
// below — never above, a word not yet taught (gamedata.RainPool).
var i, k, l, li, w;
global.pv_pn = 0;
if (global.pv_place) {
  for (k = 0; k < global.nex; k += 1) {
    w = "";
    if (global.x_kind[k] == 0) w = global.x_show[k];
    if (global.x_kind[k] == 4) w = global.x_ans[k, 0];
    if (w == "") continue;
    for (i = 0; i < global.nw; i += 1) if (global.w_word[i] == w) {
      if (pv_ok(i)) {
        for (l = 0; l < global.pv_pn; l += 1) if (global.pv_p[l] == i) break;
        if (l == global.pv_pn) { global.pv_p[global.pv_pn] = i; global.pv_pn += 1; } // once
      }
      break;
    }
  }
  if (global.pv_pn >= 4) exit;
  global.pv_pn = 0;
}
li = lv_index(level_me());
for (l = li; l >= 0; l -= 1) {
  if (l < li && global.pv_pn >= 20) break;
  for (i = 0; i < global.nw; i += 1) if (lv_index(global.w_level[i]) == l && pv_ok(i)) { global.pv_p[global.pv_pn] = i; global.pv_pn += 1; }
}
