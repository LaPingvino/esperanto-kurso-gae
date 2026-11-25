// draw_result(): the place done: stars, and the words its cards add to the collection.
var x, y, w, k, i, col, cw, rows, n;
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
n = 0;
for (i = 0; i < global.nex; i += 1) if (global.x_kind[i] == 0) n += 1;
if (n > 0) {
  txt(x, y, "Vortoj en via kolekto (" + string(n) + "):", 17, 1, global.C_text);
  y += 32;
  col = 1; if (global.wide) col = 3;
  cw = w / col;
  rows = floor((global.H - 150 - y) / 26);
  k = 0;
  for (i = 0; i < global.nex; i += 1) if (global.x_kind[i] == 0 && k < rows * col) {
    txt(x + (k mod col) * cw, y + (k div col) * 26, fit(global.x_show[i] + " — " + global.x_opt[i, global.x_right[i]], cw - 16, 15, 0), 15, 0, global.C_text);
    k += 1;
  }
}
// the bonus: the games, the race with this place's words
y = global.H - 128;
txt(global.m, y + 12, "Bonuso:", 15, 1, global.C_text);
font(15, 1, 0); x = global.m + string_width("Bonuso:") + 12;
button(x, y, x + bw("Vortludo"), y + 44, "Vortludo", 743, 0); x += bw("Vortludo") + 8;
button(x, y, x + bw("Vetkuro"), y + 44, "Vetkuro", 744, 0);
button(global.W - global.m - bw(global.t_continue_cta) - 24, global.H - 72, global.W - global.m, global.H - 20, global.t_continue_cta, 11, 1);
global.def_act = 11;
