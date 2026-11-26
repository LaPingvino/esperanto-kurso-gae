// kb_tadd(label): a tile into Konstrui's bank, unless one shows it already.
var k;
for (k = 0; k < global.kb_nt; k += 1) if (global.kb_t[k] == argument0) return 0;
global.kb_t[global.kb_nt] = argument0; global.kb_nt += 1;
return 1;
