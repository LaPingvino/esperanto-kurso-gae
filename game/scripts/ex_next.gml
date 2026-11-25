// ex_next(): on to the next exercise, or the place is done.
global.qi += 1;
if (global.qi < global.qn) { ex_begin(); exit; }
lesson_done();
