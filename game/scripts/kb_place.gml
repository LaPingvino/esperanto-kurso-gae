// kb_place(t): Konstrui's tile t goes after the pieces placed so far.
if (global.fb != 0 || argument0 >= global.kb_nt) exit;
if (global.kb_used[argument0]) exit;
global.kb_used[argument0] = 1;
global.kb_bp[global.kb_n] = argument0;
global.kb_n += 1;
global.focus = -1;
