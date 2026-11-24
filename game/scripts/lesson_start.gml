// lesson_start(): the lesson's exercises in a row: up to five of its words
// first, then the site's fill-ins, then its sentences to build.
var l, k;
l = global.les;
global.qn = 0;
for (k = 0; k < global.l_nw[l] && k < 5; k += 1) { global.q_kind[global.qn] = 0; global.q_idx[global.qn] = global.l_word[l, k]; global.q_again[global.qn] = 0; global.qn += 1; }
for (k = 0; k < global.l_nf[l]; k += 1) { global.q_kind[global.qn] = 1; global.q_idx[global.qn] = k; global.q_again[global.qn] = 0; global.qn += 1; }
for (k = 0; k < global.l_nb[l]; k += 1) { global.q_kind[global.qn] = 2; global.q_idx[global.qn] = k; global.q_again[global.qn] = 0; global.qn += 1; }
global.qtotal = global.qn; // the first tries
global.qfirst = 0;         // right at the first try
global.qi = 0;
global.scr = 2;
ex_begin();
