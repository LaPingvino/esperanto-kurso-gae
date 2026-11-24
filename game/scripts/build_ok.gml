// build_ok(): the placed words make the lesson's sentence.
var e, s, k;
e = global.les * 100 + global.idx;
s = "";
for (k = 0; k < global.bn; k += 1) { if (k > 0) s += " "; s += global.b_tile[e, global.bp[k]]; }
return same(s, global.b_sent[global.les, global.idx]);
