// ft_pool(): Frazotrajno's sentences into ft_p[] (ft_pn of them, indexes of
// f_*). After a place (ft_place), that place's own when it has two or more;
// else the learner's level's and, while fewer than ten, the levels below —
// never above, words not yet taught (gamedata.TrainPool).
var i, l, li;
global.ft_pn = 0;
if (global.ft_place) {
  for (i = 0; i < global.nf; i += 1) if (global.f_place[i] == global.les_slug) { global.ft_p[global.ft_pn] = i; global.ft_pn += 1; }
  if (global.ft_pn >= 2) exit;
  global.ft_pn = 0;
}
li = lv_index(level_me());
for (l = li; l >= 0; l -= 1) {
  if (l < li && global.ft_pn >= 10) break;
  for (i = 0; i < global.nf; i += 1) if (lv_index(global.f_level[i]) == l) { global.ft_p[global.ft_pn] = i; global.ft_pn += 1; }
}
