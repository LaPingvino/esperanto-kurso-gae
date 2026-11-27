// mm_pool(): Memoro's words into mm_p[] (mm_pn of them, indexes of w_*).
// After a place (mm_place), that place's words when four or more can be
// pairs; else the learner's level's and, while fewer than twelve, the
// levels below — never above, a word not yet taught (gamedata.MemoPool).
var i, k, l, li, w;
global.mm_pn = 0;
if (global.mm_place) {
  for (k = 0; k < global.nex; k += 1) {
    w = "";
    if (global.x_kind[k] == 0) w = global.x_show[k];
    if (global.x_kind[k] == 4) w = global.x_ans[k, 0];
    if (w == "") continue;
    for (i = 0; i < global.nw; i += 1) if (global.w_word[i] == w) {
      if (mm_ok(i)) {
        for (l = 0; l < global.mm_pn; l += 1) if (global.mm_p[l] == i) break;
        if (l == global.mm_pn) { global.mm_p[global.mm_pn] = i; global.mm_pn += 1; } // once
      }
      break;
    }
  }
  if (global.mm_pn >= 4) exit;
  global.mm_pn = 0;
}
li = lv_index(level_me());
for (l = li; l >= 0; l -= 1) {
  if (l < li && global.mm_pn >= 12) break;
  for (i = 0; i < global.nw; i += 1) if (lv_index(global.w_level[i]) == l && mm_ok(i)) { global.mm_p[global.mm_pn] = i; global.mm_pn += 1; }
}
