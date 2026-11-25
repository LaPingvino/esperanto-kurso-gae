// words_open(game): a bonus game (1 Vortludo, 2 Vetkuro) with the words in
// the meanings' language: the copy kept in the browser at once, and the
// site asked for the current one (the day's word changes daily); with
// neither, a loading screen (10) until they come.
var t;
global.game = argument0; global.focus = -1; global.wd_fail = 0;
if (global.wd_lang != lang_cur()) {
  global.nw = 0; global.nd = 0;
  t = file_read_all("ludo-vortoj-" + lang_cur() + ".txt");
  if (words_parse(t)) global.wd_lang = lang_cur();
}
if (global.site && !global.wd_asked) { global.wd_asked = 1; global.wd_req = http_get("/ludo-api/vortoj?lingvo=" + lang_cur()); }
if (global.wd_req >= 0 && global.nw == 0) { global.scr = 10; exit; }
game_start();
