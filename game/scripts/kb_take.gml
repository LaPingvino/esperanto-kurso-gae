// kb_take(k): Konstrui's k-th placed piece goes back to the bank.
var i;
if (global.fb != 0 || argument0 >= global.kb_n) exit;
global.kb_used[global.kb_bp[argument0]] = 0;
for (i = argument0; i < global.kb_n - 1; i += 1) global.kb_bp[i] = global.kb_bp[i + 1];
global.kb_n -= 1;
global.focus = -1;
