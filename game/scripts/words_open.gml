// words_open(game): a bonus game (1 Vortludo, 2 Vetkuro, 3 Konstrui, 4 Memoro, 5 Frazotrajno) with the words in
// the meanings' language: the copy kept in the browser at once, and the
// site asked for the current one once a session (the day's word changes
// daily); with neither, a loading screen (10) until they come. The day's
// word waits for the site's list; without it, Vortludo plays free.
var t;
global.game = argument0; global.focus = -1; global.wd_fail = 0;
if (global.wd_lang != lang_cur()) {
  global.nw = 0; global.nd = 0; global.nf = 0; global.wd_fresh = 0;
  t = file_read_all("ludo-vortoj-" + lang_cur() + ".txt");
  if (words_parse(t)) global.wd_lang = lang_cur();
}
if (global.site && !global.wd_asked && !global.wd_fresh) { global.wd_asked = 1; global.wd_req = http_get("/ludo-api/vortoj?lingvo=" + lang_cur()); }
// the day's word only from today's list: a kept copy may be yesterday's
if (global.wd_req >= 0 && (global.nw == 0 || (argument0 == 1 && global.vl_daily))) { global.scr = 10; exit; }
if (argument0 == 1 && global.vl_daily && !global.wd_fresh) global.vl_daily = 0; // no fresh list: free play
game_start();
