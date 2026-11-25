// load_progress(): the player's own file (in the browser: this site's
// localStorage). Version 3: the language, the stars by place slug, the
// typed answers not yet sent, the words collected. Versions 1 and 2 (the
// Zagreb-only game: stars by lesson number, the language by its place in
// that game's list) are read into it.
var v, i, n, g, old;
if (!file_exists("esperanto-kurso-ludo.txt")) exit;
file_open_read("esperanto-kurso-ludo.txt");
v = file_read_real(); file_readln();
if (v == 1 || v == 2) {
  old = "en cs da de el es fr hu id it nl pl pt ro ru sv sw tr uk vi"; // that game's languages, in order
  i = file_read_real(); file_readln();
  if (i >= 0 && i < 20) global.mylang = string_copy(old, i * 3 + 1, 2);
  n = file_read_real(); file_readln();
  for (i = 0; i < n; i += 1) {
    g = file_read_real(); file_readln();
    if (i < 9) stars_set("zagr-l0" + string(i + 1) + "-teksto", g); else stars_set("zagr-l" + string(i + 1) + "-teksto", g);
  }
}
if (v == 3) {
  global.mylang = file_read_string(); file_readln();
  n = file_read_real(); file_readln();
  for (i = 0; i < n; i += 1) { global.st_slug[i] = file_read_string(); file_readln(); global.st_n[i] = file_read_real(); file_readln(); }
  global.nst = n;
}
if (v == 2 || v == 3) {
  global.ob_n = file_read_real(); file_readln();
  for (i = 0; i < global.ob_n; i += 1) {
    global.ob_slug[i] = file_read_string(); file_readln();
    global.ob_na[i] = file_read_real(); file_readln();
    for (g = 0; g < global.ob_na[i]; g += 1) { global.ob_a[i, g] = file_read_string(); file_readln(); }
  }
}
if (v == 3) {
  global.nword = file_read_real(); file_readln();
  for (i = 0; i < global.nword; i += 1) { global.word_show[i] = file_read_string(); file_readln(); global.word_mean[i] = file_read_string(); file_readln(); }
}
file_close();
