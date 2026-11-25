// load_progress(): the stars per lesson and the language, from the player's
// own file (the browser keeps it in this site's localStorage).
var v, i, n, g;
if (!file_exists("esperanto-kurso-ludo.txt")) exit;
file_open_read("esperanto-kurso-ludo.txt");
v = file_read_real(); file_readln();
if (v == 1 || v == 2) {
  global.lang_i = file_read_real(); file_readln();
  if (global.lang_i < 0 || global.lang_i >= global.nlang) global.lang_i = 0;
  n = file_read_real(); file_readln();
  for (i = 0; i < n && i < global.nl; i += 1) { global.stars[i] = file_read_real(); file_readln(); }
}
if (v == 2) {
  global.ob_n = file_read_real(); file_readln();
  for (i = 0; i < global.ob_n; i += 1) {
    global.ob_slug[i] = file_read_string(); file_readln();
    global.ob_na[i] = file_read_real(); file_readln();
    for (g = 0; g < global.ob_na[i]; g += 1) { global.ob_a[i, g] = file_read_string(); file_readln(); }
  }
}
file_close();
