// ft_check(): the whole train judged: right when its words, joined, are the
// text's sentence (alike words may swap places); else ft_good is how many
// wagons from the locomotive on are in place: those stay, the rest go back
// (ft_back) after a second and a half or at the next press. Nothing goes to
// the site: only a course exercise's typed answer moves the ratings.
var k;
global.ft_tries += 1;
if (ft_built(1) == global.ft_s) { global.ft_st = 2; global.ft_ok += 1; global.ft_wait = 45; exit; }
global.ft_good = 0;
for (k = 0; k < global.ft_n; k += 1) { if (global.ft_t[global.ft_bp[k]] != global.ft_b[k]) break; global.ft_good += 1; }
global.ft_st = 1; global.ft_wait = 45;
