// draw_result(): the lesson done: stars, and the words it adds to the collection.
var x, y, w, k, wi, col, cw, rows;
x = global.m; w = global.W - 2 * global.m;
if (global.wide) { w = min(w, 900); x = (global.W - w) / 2; }
y = global.hh + 28;
txtc(global.W / 2, y, global.l_title[global.les], 22, 1, global.C_text);
y += 42;
txtc(global.W / 2, y, "Finita!", 30, 1, global.C_main);
y += 74;
stars(global.W / 2, y, 22, global.got);
y += 40;
txtc(global.W / 2, y, string(global.qfirst) + " el " + string(global.qtotal) + " ĝuste je la unua provo", 16, 0, global.C_muted);
y += 44;
txt(x, y, "Novaj vortoj en via kolekto (" + string(global.l_nw[global.les]) + "):", 17, 1, global.C_text);
y += 32;
col = 1; if (global.wide) col = 3;
cw = w / col;
rows = floor((global.H - 100 - y) / 26);
for (k = 0; k < global.l_nw[global.les] && k < rows * col; k += 1) {
  wi = global.l_word[global.les, k];
  txt(x + (k mod col) * cw, y + (k div col) * 26, global.w_base[wi] + " — " + global.w_def[wi, global.lang_i], 15, 0, global.C_text);
}
button(global.W - global.m - bw(global.t_continue_cta) - 24, global.H - 72, global.W - global.m, global.H - 20, global.t_continue_cta, 11, 1);
global.def_act = 11;
