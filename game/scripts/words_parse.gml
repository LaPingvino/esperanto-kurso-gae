// words_parse(text): the bonus games' words (GET /ludo-api/vortoj, see
// gamedata/words.go) into w_word, w_level, w_five, w_mean, w_def, w_pic, w_aff, w_blk, w_drv (a word for Konstrui alone), and
// the day's word per level (d_level, d_word), Frazotrajno's sentences (f_level, f_place, f_text),
// Bonvorto's pairs (b_neo, b_word, b_shape, b_div, b_mean); 1 when it is a whole one.
var l, n, nd, nf, nb;
if (string_pos(chr(10) + "FINO", argument0) == 0) return 0;
global.ptext = argument0; global.peof = 0;
split_line(next_line());
if (global.fld[0] != "VORTOJ") return 0;
global.wd_day = global.fld[2];
n = 0; nd = 0; nf = 0; nb = 0;
while (!global.peof) {
  l = next_line();
  if (l == "") continue;
  split_line(l);
  if (global.fld[0] == "W") {
    global.w_word[n] = global.fld[1]; global.w_level[n] = global.fld[2]; global.w_five[n] = (global.fld[3] == "1");
    global.w_mean[n] = global.fld[4]; global.w_def[n] = global.fld[5]; global.w_pic[n] = global.fld[6]; global.w_aff[n] = global.fld[7]; global.w_blk[n] = global.fld[8]; global.w_drv[n] = (global.fld[9] == "1");
    n += 1;
  }
  if (global.fld[0] == "T") { global.d_level[nd] = global.fld[1]; global.d_word[nd] = global.fld[2]; nd += 1; }
  if (global.fld[0] == "B") { global.b_neo[nb] = global.fld[1]; global.b_word[nb] = global.fld[2]; global.b_shape[nb] = global.fld[3]; global.b_div[nb] = global.fld[4]; global.b_mean[nb] = global.fld[5]; nb += 1; }
  if (global.fld[0] == "F") { global.f_level[nf] = global.fld[1]; global.f_place[nf] = global.fld[2]; global.f_text[nf] = global.fld[3]; nf += 1; }
}
global.nw = n; global.nd = nd; global.nf = nf; global.nbw = nb;
return 1;
