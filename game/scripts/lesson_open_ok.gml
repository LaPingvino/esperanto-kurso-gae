// lesson_open_ok(l): the first lesson, and each one after a finished one, is open.
if (argument0 == 0) return 1;
return global.stars[argument0 - 1] > 0;
