// mm_hide(): the two cards up that were no pair face down again.
if (global.mm_a >= 0) global.mm_st[global.mm_a] = 0;
if (global.mm_b >= 0) global.mm_st[global.mm_b] = 0;
global.mm_a = -1; global.mm_b = -1; global.mm_wait = 0;
