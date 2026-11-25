// draw_exercise(): the current exercise, and after an answer the verdict.
var y;
y = global.hh + 14;
draw_progress(y);
y += 50;
if (global.kind == 0 || global.kind == 3) draw_choice(y);
if (global.kind == 1 || global.kind == 4 || global.kind == 5) draw_fill(y);
if (global.kind == 2) draw_build(y);
if (global.fb != 0) draw_verdict();
