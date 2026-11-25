// draw_verdict(): right, or wrong with the right answer; then on.
var y, x, w, s, h, b;
s = "";
if (global.kind == 0) s = global.x_show[global.idx] + " = " + global.x_opt[global.idx, global.x_right[global.idx]];
if (global.kind == 3) s = global.x_opt[global.idx, global.x_right[global.idx]];
if (global.kind == 1 || global.kind == 4 || global.kind == 5) s = gap_answer(global.idx);
if (global.kind == 2) s = global.x_sent[global.idx] + ".";
x = global.m; w = global.W - 2 * global.m;
h = 150; if (!global.wide) h = 190;
y = global.H - h;
if (global.fb == 1) {
  fill(0, y, global.W, global.H, global.C_okbg);
  fill(0, y, global.W, y + 3, global.C_ok);
  txt(x, y + 18, "Ĝuste!", 24, 1, global.C_ok);
  tw(x, y + 56, s, w, 17, 0, global.C_text);
} else {
  fill(0, y, global.W, global.H, global.C_badbg);
  fill(0, y, global.W, y + 3, global.C_bad);
  txt(x, y + 18, "Ne ĝuste. La ĝusta respondo:", 20, 1, global.C_bad);
  tw(x, y + 52, s, w, 17, 1, global.C_text);
}
b = bw(global.t_continue_cta) + 24;
button(global.W - global.m - b, global.H - 70, global.W - global.m, global.H - 18, global.t_continue_cta, 6, 1);
global.def_act = 6;
