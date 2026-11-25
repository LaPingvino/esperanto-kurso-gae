// save_progress(): see load_progress. Lines: version (2), language, the
// number of lessons, each lesson's stars (0: not done yet), then the typed
// answers not yet sent to the site: their number, and for each its slug,
// its number of gaps and an answer per gap.
var i, g;
file_open_write("esperanto-kurso-ludo.txt");
file_write_real(2); file_writeln();
file_write_real(global.lang_i); file_writeln();
file_write_real(global.nl); file_writeln();
for (i = 0; i < global.nl; i += 1) { file_write_real(global.stars[i]); file_writeln(); }
file_write_real(global.ob_n); file_writeln();
for (i = 0; i < global.ob_n; i += 1) {
  file_write_string(global.ob_slug[i]); file_writeln();
  file_write_real(global.ob_na[i]); file_writeln();
  for (g = 0; g < global.ob_na[i]; g += 1) { file_write_string(global.ob_a[i, g]); file_writeln(); }
}
file_close();
