// choice_ok(i, k): choice k of exercise i is the right one — a word card's
// meaning, or multiple choice as the site checks it (checkAnswer: the
// chosen option's text against the right option's, or an alternative).
var i, j;
i = argument0;
if (argument1 == global.x_right[i]) return 1;
if (global.x_kind[i] != 3) return 0;
if (same(global.x_opt[i, argument1], global.x_opt[i, global.x_right[i]])) return 1;
for (j = 0; j < global.x_ny[i]; j += 1) if (same(global.x_opt[i, argument1], global.x_alt[i, j])) return 1;
return 0;
