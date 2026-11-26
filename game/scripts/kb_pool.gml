// kb_pool(): Konstrui's words into kb_p[] (kb_pn of them, indexes of w_*):
// words whose split is beyond doubt (w_blk, legvortoj.Blocks). After a
// place (kb_place), that place's words when three or more of them are such;
// else the learner's level's and, while fewer than ten, the levels below —
// never above, a word not yet taught (gamedata.BuildPool).
var i, k, l, li, w;
global.kb_pn = 0;
if (global.kb_place) {
  for (k = 0; k < global.nex; k += 1) {
    w = "";
    if (global.x_kind[k] == 0) w = global.x_show[k];
    if (global.x_kind[k] == 4) w = global.x_ans[k, 0];
    if (w == "") continue;
    for (i = 0; i < global.nw; i += 1) if (global.w_word[i] == w && global.w_blk[i] != "") {
      for (l = 0; l < global.kb_pn; l += 1) if (global.kb_p[l] == i) break;
      if (l == global.kb_pn) { global.kb_p[global.kb_pn] = i; global.kb_pn += 1; } // once
      break;
    }
  }
  if (global.kb_pn >= 3) exit;
  global.kb_pn = 0;
}
li = lv_index(level_me());
for (l = li; l >= 0; l -= 1) {
  if (l < li && global.kb_pn >= 10) break;
  for (i = 0; i < global.nw; i += 1) if (global.w_blk[i] != "" && lv_index(global.w_level[i]) == l) { global.kb_p[global.kb_pn] = i; global.kb_pn += 1; }
}
