// level_me(): the learner's level, for the bonus games' words: the
// account's (its rating, once settled: /ludo-progreso's R line), else the
// level the map is at (the first with a series not finished).
var s;
if (global.acclevel != "") return global.acclevel;
for (s = 0; s < global.ns; s += 1) if (series_done(s) < global.s_np[s]) return global.s_level[s];
return "A0";
