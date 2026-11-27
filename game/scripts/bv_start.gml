// bv_start(): a Bonvorto round — Vortludo's board on a pair of La Bona
// Lingvo (B lines, gamedata/bona.go): the word with a root of its own given
// (vl_neo), the built word of the same meaning to find, its shape with the
// roots hidden beside the board (vl_shape: …·ar·o). Any pair, whatever the
// level; never the last one again when there are others.
var i;
global.vl_bona = 1; global.vl_daily = 0; global.vl_long = 1;
if (global.nbw == 0) { global.wd_fail = 1; global.scr = 10; exit; }
i = floor(random(global.nbw));
if (global.nbw > 1 && i == global.bv_last) i = (i + 1) mod global.nbw;
global.bv_last = i;
global.vl_word = global.b_word[i]; global.vl_n = string_length(global.vl_word); global.vl_aff = "";
global.vl_neo = global.b_neo[i]; global.vl_shape = global.b_shape[i]; global.vl_div = global.b_div[i];
global.vl_mean = global.b_mean[i]; global.vl_def = ""; global.vl_pic = -1; // the hint after three misses: the meaning
vl_reset();
