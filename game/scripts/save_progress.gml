// save_progress(): see load_progress. Lines: version, language, the number
// of lessons, then each lesson's stars (0: not done yet).
var i;
file_open_write("esperanto-kurso-ludo.txt");
file_write_real(1); file_writeln();
file_write_real(global.lang_i); file_writeln();
file_write_real(global.nl); file_writeln();
for (i = 0; i < global.nl; i += 1) { file_write_real(global.stars[i]); file_writeln(); }
file_close();
