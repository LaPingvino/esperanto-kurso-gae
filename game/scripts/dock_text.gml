// dock_text(): the right answer as the verdict's dock shows it.
var s;
s = "";
if (global.kind == 0) s = global.x_show[global.idx] + " = " + global.x_opt[global.idx, global.x_right[global.idx]];
if (global.kind == 3) s = global.x_opt[global.idx, global.x_right[global.idx]];
if (global.kind == 1 || global.kind == 4 || global.kind == 5) s = gap_answer(global.idx);
if (global.kind == 2) s = global.x_sent[global.idx] + ".";
return s;
