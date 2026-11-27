// mm_step(): every step on Memoro: a pair that wasn't one turns back.
if (global.mm_wait > 0) { global.mm_wait -= 1; if (global.mm_wait == 0) mm_hide(); }
