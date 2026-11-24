// fill_ok(lesson, exercise, gap, text): the site's check of one gap
// (checkAnswer, "fillin"): the answer for that gap — one per gap, or the one
// answer for every gap — compared with same(). The site checks no
// alternatives for fill-ins, so neither does the game.
var e, n;
e = argument0 * 100 + argument1;
n = global.f_nans[argument0, argument1];
if (n <= 0) return 0;
return same(argument3, global.f_ans[e, argument2 mod n]);
