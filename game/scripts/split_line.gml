// split_line(line): its tab-separated fields into global.fld[0 …], their
// number in global.nfld (gamedata/text.go's lines).
var s, p;
s = argument0;
global.nfld = 0;
while (1) {
  p = string_pos(chr(9), s);
  if (p == 0) { global.fld[global.nfld] = s; global.nfld += 1; break; }
  global.fld[global.nfld] = string_copy(s, 1, p - 1);
  global.nfld += 1;
  s = string_delete(s, 1, p);
}
for (p = global.nfld; p < global.nfld + 8; p += 1) global.fld[p] = ""; // fields asked for but not there: ""
