// ex_next(): on to the next exercise, or the lesson is done.
var r, s;
global.qi += 1;
if (global.qi < global.qn) { ex_begin(); exit; }
r = global.qfirst / max(1, global.qtotal);
s = 1;
if (r >= 0.6) s = 2;
if (r >= 0.9) s = 3;
global.got = s;
if (s > global.stars[global.les]) global.stars[global.les] = s;
save_progress();
global.scr = 3;
global.focus = -1;
