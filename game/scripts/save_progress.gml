// save_progress(): see load_progress (version 3).
var i, g;
file_open_write("esperanto-kurso-ludo.txt");
file_write_real(3); file_writeln();
file_write_string(global.mylang); file_writeln();
file_write_real(global.nst); file_writeln();
for (i = 0; i < global.nst; i += 1) { file_write_string(global.st_slug[i]); file_writeln(); file_write_real(global.st_n[i]); file_writeln(); }
file_write_real(global.ob_n); file_writeln();
for (i = 0; i < global.ob_n; i += 1) {
  file_write_string(global.ob_slug[i]); file_writeln();
  file_write_real(global.ob_na[i]); file_writeln();
  for (g = 0; g < global.ob_na[i]; g += 1) { file_write_string(global.ob_a[i, g]); file_writeln(); }
}
file_write_real(global.nword); file_writeln();
for (i = 0; i < global.nword; i += 1) { file_write_string(global.word_show[i]); file_writeln(); file_write_string(global.word_mean[i]); file_writeln(); }
file_close();
