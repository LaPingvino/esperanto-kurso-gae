// lesson_start(): the place's exercises in a row, in the order the lesson
// gives them (word cards, the site's exercises, sentences to build).
var k;
global.qn = 0;
for (k = 0; k < global.nex; k += 1) { global.q_idx[global.qn] = k; global.q_again[global.qn] = 0; global.qn += 1; }
global.qtotal = global.qn; global.qfirst = 0; global.qi = 0;
if (global.qn == 0) { lesson_done(); exit; } // a text to read only
global.scr = 2;
ex_begin();
