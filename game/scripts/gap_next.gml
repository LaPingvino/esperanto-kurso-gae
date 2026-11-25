// gap_next(): global.cg to the first empty gap (global.ngap: all filled).
var g;
for (g = 0; g < global.ngap; g += 1) if (global.fa[g] == "") { global.cg = g; exit; }
global.cg = global.ngap;
