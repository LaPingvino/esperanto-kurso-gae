// parity_dump(): every exercise answered as the game judges it, into
// parity.txt, for the Go test that judges the same answers with the site's
// own checkAnswer (internal/handler/game_parity_test.go). Lines, tab-separated:
//   F slug verdict answers…   the site's right answers, gap by gap
//   W slug verdict answers…   a wrong tile in the first gap
//   T slug gap n              how many of the gap's tiles the game accepts
//   B lesson:k verdict        a sentence placed in its order (R: in the tiles' shown order)
//   C word verdict-right verdict-other
var l, k, e, g, n, s, ok, t, cnt, i, wrong, nt;
file_open_write("parity.txt");
for (l = 0; l < global.nl; l += 1) {
  for (k = 0; k < global.l_nf[l]; k += 1) {
    e = l * 100 + k;
    n = global.f_nans[l, k];
    global.les = l;
    // the right answers, through the same fill_pick the tiles and typing use
    px_begin(1, k); s = "";
    for (g = 0; g < global.ngap; g += 1) { s += chr(9) + global.f_ans[e, g mod n]; fill_pick(global.f_ans[e, g mod n], 0); }
    file_write_string("F" + chr(9) + global.f_slug[l, k] + chr(9) + string(global.fb == 1) + s); file_writeln();
    // a wrong tile in the first gap, the right answers in the others
    wrong = "";
    for (t = 0; t < global.f_nt[e, 0]; t += 1) if (wrong == "" && !fill_ok(l, k, 0, global.f_tile[e, t])) wrong = global.f_tile[e, t];
    px_begin(1, k); s = "";
    for (g = 0; g < global.ngap; g += 1) {
      if (g == 0) { s += chr(9) + wrong; fill_pick(wrong, 0); }
      else { s += chr(9) + global.f_ans[e, g mod n]; fill_pick(global.f_ans[e, g mod n], 0); }
    }
    file_write_string("W" + chr(9) + global.f_slug[l, k] + chr(9) + string(global.fb == 1) + s); file_writeln();
    for (g = 0; g < global.f_ngap[l, k]; g += 1) {
      cnt = 0;
      for (t = 0; t < global.f_nt[e, g]; t += 1) cnt += fill_ok(l, k, g, global.f_tile[e, g * 10 + t]);
      file_write_string("T" + chr(9) + global.f_slug[l, k] + chr(9) + string(g) + chr(9) + string(cnt)); file_writeln();
    }
  }
  for (k = 0; k < global.l_nb[l]; k += 1) {
    e = l * 100 + k;
    nt = global.b_nt[l, k];
    global.les = l; px_begin(2, k);
    // the sentence's words in order, each the first unused tile that matches
    s = global.b_sent[l, k] + " ";
    while (string_length(s) > 0) {
      i = string_pos(" ", s);
      for (t = 0; t < nt; t += 1) if (!global.bused[t] && global.b_tile[e, t] == string_copy(s, 1, i - 1)) { build_place(t); break; }
      s = string_delete(s, 1, i);
    }
    file_write_string("B" + chr(9) + string(l) + ":" + string(k) + chr(9) + string(global.fb == 1) + chr(9) + string(global.bn)); file_writeln();
    px_begin(2, k);
    for (t = 0; t < nt; t += 1) build_place(t); // the tiles in the order shown: never the sentence's (cmd/export-game)
    file_write_string("R" + chr(9) + string(l) + ":" + string(k) + chr(9) + string(global.fb == 1)); file_writeln();
  }
}
for (i = 0; i < global.nwords; i += 1) {
  px_begin(0, i);
  ok = -1; wrong = -1;
  for (k = 0; k < 4; k += 1) { if (global.w_opt[i, k] == i) ok = k; else if (wrong < 0) wrong = k; }
  act(500 + ok); s = string(global.fb == 1);
  px_begin(0, i); act(500 + wrong);
  file_write_string("C" + chr(9) + global.w_root[i] + chr(9) + s + chr(9) + string(global.fb == 1)); file_writeln();
}
file_close();
global.scr = 0; global.les = -1; global.qn = 0;
show_debug_message("parity.txt written");
