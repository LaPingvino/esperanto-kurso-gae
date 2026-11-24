// load_progress(): the stars per lesson and the language, from the player's
// own file (the browser keeps it in this site's localStorage).
var v, i, n;
if (!file_exists("esperanto-kurso-ludo.txt")) exit;
file_open_read("esperanto-kurso-ludo.txt");
v = file_read_real(); file_readln();
if (v == 1) {
  global.lang_i = file_read_real(); file_readln();
  if (global.lang_i < 0 || global.lang_i >= global.nlang) global.lang_i = 0;
  n = file_read_real(); file_readln();
  for (i = 0; i < n && i < global.nl; i += 1) { global.stars[i] = file_read_real(); file_readln(); }
}
file_close();
