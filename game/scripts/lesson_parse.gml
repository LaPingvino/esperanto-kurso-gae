// lesson_parse(text): a place (GET /ludo-api/leciono/…) into the current
// lesson's arrays (x_*: one exercise each, every array set for every kind,
// as GameMaker 5 evaluates both sides of &&); 1 when it is a whole one.
var l, i, k, h, g;
if (string_pos(chr(10) + "FINO", argument0) == 0) return 0;
global.ptext = argument0; global.peof = 0;
split_line(next_line());
if (global.fld[0] != "LECIONO") return 0;
global.les_slug = global.fld[2]; global.les_kind = global.fld[3];
global.les_title = ""; global.les_video = ""; global.npara = 0; global.nex = 0;
i = -1;
while (!global.peof) {
  l = next_line();
  if (l == "") continue;
  split_line(l);
  h = global.fld[0];
  if (h == "T") global.les_title = global.fld[1];
  if (h == "X") { global.para[global.npara] = global.fld[1]; global.npara += 1; }
  if (h == "U") global.les_video = global.fld[1];
  if (h == "C" || h == "F" || h == "M" || h == "Q" || h == "H" || h == "B") {
    i = global.nex; global.nex += 1;
    global.x_slug[i] = ""; global.x_q[i] = ""; global.x_hint[i] = ""; global.x_na[i] = 0; global.x_ny[i] = 0;
    global.x_ng[i] = 0; global.x_ntok[i] = 0; global.x_no[i] = 0; global.x_right[i] = 0;
    global.x_show[i] = ""; global.x_seen[i] = ""; global.x_sent[i] = ""; global.x_nk[i] = 0;
    for (g = 0; g < 10; g += 1) global.x_nt[i, g] = 0;
    if (h == "C") { global.x_kind[i] = 0; global.x_show[i] = global.fld[1]; global.x_seen[i] = global.fld[2]; global.x_right[i] = real(global.fld[3]);
      for (k = 4; k < global.nfld; k += 1) { global.x_opt[i, global.x_no[i]] = global.fld[k]; global.x_no[i] += 1; } }
    if (h == "F") { global.x_kind[i] = 1; global.x_slug[i] = global.fld[1]; global.x_q[i] = global.fld[2]; global.x_hint[i] = global.fld[3]; }
    if (h == "M") { global.x_kind[i] = 3; global.x_slug[i] = global.fld[1]; global.x_q[i] = global.fld[2]; global.x_right[i] = real(global.fld[3]); }
    if (h == "Q") { global.x_kind[i] = 4; global.x_slug[i] = global.fld[1]; global.x_q[i] = global.fld[2]; global.x_hint[i] = global.fld[3]; global.x_ng[i] = 1; }
    if (h == "H") { global.x_kind[i] = 5; global.x_slug[i] = global.fld[1]; global.x_q[i] = global.fld[2]; global.x_hint[i] = global.fld[3]; global.x_ng[i] = 1; }
    if (h == "B") { global.x_kind[i] = 2; global.x_sent[i] = global.fld[1]; }
  }
  if (i >= 0) {
    if (h == "O") {
      k = global.x_ntok[i];
      global.x_tok[i, k] = global.fld[1]; global.x_tgap[i, k] = real(global.fld[2]); global.x_tsp[i, k] = real(global.fld[3]);
      global.x_ntok[i] += 1;
      if (global.x_tgap[i, k] >= 0) global.x_ng[i] = max(global.x_ng[i], global.x_tgap[i, k] + 1);
    }
    if (h == "A") { global.x_ans[i, global.x_na[i]] = global.fld[1]; global.x_na[i] += 1; }
    if (h == "Y") { global.x_alt[i, global.x_ny[i]] = global.fld[1]; global.x_ny[i] += 1; }
    if (h == "K") {
      g = global.x_nk[i]; global.x_nk[i] += 1; // the g-th tile set (a fill-in: one per gap)
      if (global.x_kind[i] == 3) { for (k = 1; k < global.nfld; k += 1) { global.x_opt[i, global.x_no[i]] = global.fld[k]; global.x_no[i] += 1; } }
      else for (k = 1; k < global.nfld && k <= 9; k += 1) { global.x_tile[i, g * 10 + k - 1] = global.fld[k]; global.x_nt[i, g] = k; }
    }
  }
}
return 1;
