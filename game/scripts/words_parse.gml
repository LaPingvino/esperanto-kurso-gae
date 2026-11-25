// words_parse(text): the bonus games' words (GET /ludo-api/vortoj, see
// gamedata/words.go) into w_word, w_level, w_five, w_mean, w_def, w_pic, and
// the day's word per level (d_level, d_word); 1 when it is a whole one.
var l, n, nd;
if (string_pos(chr(10) + "FINO", argument0) == 0) return 0;
global.ptext = argument0; global.peof = 0;
split_line(next_line());
if (global.fld[0] != "VORTOJ") return 0;
global.wd_day = global.fld[2];
n = 0; nd = 0;
while (!global.peof) {
  l = next_line();
  if (l == "") continue;
  split_line(l);
  if (global.fld[0] == "W") {
    global.w_word[n] = global.fld[1]; global.w_level[n] = global.fld[2]; global.w_five[n] = (global.fld[3] == "1");
    global.w_mean[n] = global.fld[4]; global.w_def[n] = global.fld[5]; global.w_pic[n] = global.fld[6];
    n += 1;
  }
  if (global.fld[0] == "T") { global.d_level[nd] = global.fld[1]; global.d_word[nd] = global.fld[2]; nd += 1; }
}
global.nw = n; global.nd = nd;
return 1;
