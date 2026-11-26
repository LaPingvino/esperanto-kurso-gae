// dock_text(): the right answer as the verdict's dock shows it.
var s;
s = "";
if (global.scr == 11) return string_replace_all(string_replace_all(string_replace_all(global.w_blk[global.kb_w], "-|-", "·"), "-", ""), "|", "·") + " = " + global.kb_word; // Konstrui: the word in its pieces
if (global.kind == 0) s = global.x_show[global.idx] + " = " + global.x_opt[global.idx, global.x_right[global.idx]];
if (global.kind == 3) s = global.x_opt[global.idx, global.x_right[global.idx]];
if (global.kind == 1 || global.kind == 4 || global.kind == 5) s = gap_answer(global.idx);
if (global.kind == 2) s = global.x_sent[global.idx] + ".";
return s;
