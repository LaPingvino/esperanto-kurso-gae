// file_read_all(name): the whole file ("" when there is none).
var s;
if (!file_exists(argument0)) return "";
file_open_read(argument0);
s = "";
while (!file_eof()) { s += file_read_string() + chr(10); file_readln(); }
file_close();
return s;
