// gap_ok(i, gap, text): the site's check (checkAnswer) of one answer:
// ends trimmed, capitals alike (same). A fill-in: the gap's answer — one per
// gap, or the one answer for every gap — and no alternatives (the site
// checks none for fill-ins). A vocabulary word or a typed answer: the
// answer or one of its alternatives.
var i, k;
i = argument0;
if (global.x_na[i] <= 0) return 0;
if (global.x_kind[i] == 1) return same(argument2, global.x_ans[i, argument1 mod global.x_na[i]]);
if (same(argument2, global.x_ans[i, 0])) return 1;
for (k = 0; k < global.x_ny[i]; k += 1) if (same(argument2, global.x_alt[i, k])) return 1;
return 0;
