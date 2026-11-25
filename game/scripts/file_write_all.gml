// file_write_all(name, text): the text as the file. A cache file: written
// last in its event, so a full storage (an error) loses nothing else.
var t, p;
file_open_write(argument0);
t = argument1;
while (string_length(t) > 0) {
  p = string_pos(chr(10), t);
  if (p == 0) { file_write_string(t); file_writeln(); break; }
  file_write_string(string_copy(t, 1, p - 1)); file_writeln();
  t = string_delete(t, 1, p);
}
file_close();
