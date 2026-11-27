// draw_result(): the place done: stars, and the words its cards add to the collection.
var x, y, w, k, i, col, cw, rows, n, bx, bwall, two;
x = global.m; w = global.W - 2 * global.m;
if (global.wide) { w = min(w, 900); x = (global.W - w) / 2; }
y = global.hh + 28;
y += tw(x, y, global.les_title, w, 22, 1, global.C_text) + 14;
txtc(global.W / 2, y, "Finita!", 30, 1, global.C_link);
y += 74;
stars(global.W / 2, y, 22, global.got);
y += 40;
if (global.qtotal > 0) txtc(global.W / 2, y, string(global.qfirst) + " el " + string(global.qtotal) + " ĝuste je la unua provo", 16, 0, global.C_muted);
y += 44;
// the bonus row: one row after its label, else the label above, else (a phone) two rows
font(15, 1, 0); bx = global.m + string_width("Bonuso:") + 12;
bwall = bw("Vortludo") + bw("Vetkuro") + bw("Konstrui") + bw("Memoro") + bw("Frazotrajno") + 32;
two = (global.m + bwall > global.W - global.m);
n = 0;
for (i = 0; i < global.nex; i += 1) if (global.x_kind[i] == 0) n += 1;
if (n > 0) {
  txt(x, y, "Vortoj en via kolekto (" + string(n) + "):", 17, 1, global.C_text);
  y += 32;
  col = 1; if (global.wide) col = 3;
  cw = w / col;
  rows = floor((global.H - 150 - 26 * !global.wide - 52 * two - y) / 26); // a phone: room for the bonus's label (and its second row)
  k = 0;
  for (i = 0; i < global.nex; i += 1) if (global.x_kind[i] == 0 && k < rows * col) {
    txt(x + (k mod col) * cw, y + (k div col) * 26, fit(global.x_show[i] + " — " + global.x_opt[i, global.x_right[i]], cw - 16, 15, 0), 15, 0, global.C_text);
    k += 1;
  }
}
// the bonus: the games with this place's words (the race, Konstrui, Memoro) and sentences (Frazotrajno)
y = global.H - 128;
if (two) {
  txt(global.m, y - 52 - 26, "Bonuso:", 15, 1, global.C_text);
  x = global.m;
  button(x, y - 52, x + bw("Vortludo"), y - 8, "Vortludo", 743, 0); x += bw("Vortludo") + 8;
  button(x, y - 52, x + bw("Vetkuro"), y - 8, "Vetkuro", 744, 0);
  x = global.m;
} else {
  x = bx;
  if (x + bwall > global.W - global.m) { txt(global.m, y - 26, "Bonuso:", 15, 1, global.C_text); x = global.m; }
  else txt(global.m, y + 12, "Bonuso:", 15, 1, global.C_text);
  button(x, y, x + bw("Vortludo"), y + 44, "Vortludo", 743, 0); x += bw("Vortludo") + 8;
  button(x, y, x + bw("Vetkuro"), y + 44, "Vetkuro", 744, 0); x += bw("Vetkuro") + 8;
}
button(x, y, x + bw("Konstrui"), y + 44, "Konstrui", 746, 0); x += bw("Konstrui") + 8;
button(x, y, x + bw("Memoro"), y + 44, "Memoro", 748, 0); x += bw("Memoro") + 8;
button(x, y, x + bw("Frazotrajno"), y + 44, "Frazotrajno", 851, 0);
button(global.W - global.m - bw(global.t_continue_cta) - 24, global.H - 72, global.W - global.m, global.H - 20, global.t_continue_cta, 11, 1);
global.def_act = 11;
