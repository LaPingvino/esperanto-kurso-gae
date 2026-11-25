// build_ok(): the placed words make the text's sentence.
var s, k;
s = "";
for (k = 0; k < global.bn; k += 1) { if (k > 0) s += " "; s += global.x_tile[global.idx, global.bp[k]]; }
return same(s, global.x_sent[global.idx]);
