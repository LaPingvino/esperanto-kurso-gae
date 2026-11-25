// draw_exercise(): the activity stage (kurso-design): the top bar
// (draw_header), the body — the kind (icon and word), the question, the
// answer — at most 640 wide, and the dock at the bottom with the one big
// button. The body is laid out from the heights its texts take.
var y, x0, x1, top;
x0 = global.m; x1 = global.W - global.m;
if (global.wide) { x0 = (global.W - 640) / 2; x1 = x0 + 640; }
global.bx0 = x0; global.bx1 = x1;
y = global.hh + 18;
draw_sprite(spr_kind, global.kind, x0, y); // the exercise's kind (ex_begin.gml's 0–5)
txt(x0 + 30, y + 2, kind_word(global.kind), 15, 1, global.C_link);
y += 36;
if (global.kind == 0 || global.kind == 3) y = draw_choice(y);
if (global.kind == 1 || global.kind == 4 || global.kind == 5) y = draw_fill(y);
if (global.kind == 2) y = draw_build(y);
top = draw_dock();
if (y > top) overflow("the body runs under the dock");
