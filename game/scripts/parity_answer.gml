// parity_answer(): other-62 in the parity check — a lesson in: judge all of it.
var t, i, g, n, s, w, k, ok, wrong, line, slug, place;
if (http_id() != global.par_req) exit;
t = http_result();
if (global.par_stage == 0) {
  global.par_stage = 1;
  if (http_status() != 200 || !course_parse(t)) parity_line("X" + chr(9) + "kurso" + chr(9) + string(http_status()));
  parity_next();
  exit;
}
place = global.p_slug[global.par_i];
global.par_i += 1;
if (http_status() != 200 || !lesson_parse(t)) { parity_line("X" + chr(9) + place + chr(9) + string(http_status())); parity_next(); exit; }
global.les_slug = place;
global.scr = 2; // as the exercises screen: act takes answers there only
for (i = 0; i < global.nex; i += 1) {
  global.q_idx[0] = i; global.q_again[0] = 0; global.qi = 0; global.qn = 1;
  k = global.x_kind[i];
  slug = global.x_slug[i];
  if (k == 1 || k == 4 || k == 5) {
    // the right answers through the same gap_pick the tiles and typing use
    ex_begin(); s = "";
    for (g = 0; g < global.ngap; g += 1) { if (k == 1) w = global.x_ans[i, g mod global.x_na[i]]; else w = global.x_ans[i, 0]; s += chr(9) + w; gap_pick(w, 0); }
    parity_line("R" + chr(9) + slug + chr(9) + string(global.fb == 1) + s);
    wrong = "";
    for (n = 0; n < global.x_nt[i, 0]; n += 1) { if (wrong == "") { if (!gap_ok(i, 0, global.x_tile[i, n])) wrong = global.x_tile[i, n]; } }
    if (wrong == "") wrong = "ne ĉi tio";
    ex_begin(); s = "";
    for (g = 0; g < global.ngap; g += 1) {
      if (g == 0) w = wrong; else w = global.x_ans[i, g mod global.x_na[i]];
      s += chr(9) + w; gap_pick(w, 0);
    }
    parity_line("W" + chr(9) + slug + chr(9) + string(global.fb == 1) + s);
    for (g = 0; g < global.ngap; g += 1) {
      n = 0;
      for (w = 0; w < global.x_nt[i, g]; w += 1) n += gap_ok(i, g, global.x_tile[i, g * 10 + w]);
      parity_line("T" + chr(9) + slug + chr(9) + string(g) + chr(9) + string(n));
    }
  }
  if (k == 3) {
    ex_begin(); act(500 + global.x_right[i]);
    parity_line("R" + chr(9) + slug + chr(9) + string(global.fb == 1) + chr(9) + global.x_opt[i, global.x_right[i]]);
    w = (global.x_right[i] + 1) mod global.x_no[i];
    ex_begin(); act(500 + w);
    parity_line("W" + chr(9) + slug + chr(9) + string(global.fb == 1) + chr(9) + global.x_opt[i, w]);
  }
  if (k == 0) {
    ex_begin(); act(500 + global.x_right[i]); ok = (global.fb == 1);
    ex_begin(); act(500 + (global.x_right[i] + 1) mod global.x_no[i]);
    parity_line("C" + chr(9) + place + ":" + string(i) + chr(9) + string(ok) + chr(9) + string(global.fb == 1));
  }
  if (k == 2) {
    ex_begin();
    s = global.x_sent[i] + " ";
    while (string_length(s) > 0) {
      g = string_pos(" ", s);
      w = string_copy(s, 1, g - 1);
      for (n = 0; n < global.x_nt[i, 0]; n += 1) { if (!global.bused[n]) { if (global.x_tile[i, n] == w) { build_place(n); break; } } }
      s = string_delete(s, 1, g);
    }
    parity_line("B" + chr(9) + place + ":" + string(i) + chr(9) + string(global.fb == 1));
    ex_begin();
    for (n = 0; n < global.x_nt[i, 0]; n += 1) build_place(n);
    parity_line("O" + chr(9) + place + ":" + string(i) + chr(9) + string(global.fb == 1));
  }
}
parity_next();
