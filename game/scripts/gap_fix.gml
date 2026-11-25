// gap_fix(i, gap, text): a typed answer as the learner meant it. Typing
// converts the x- and h-systems as it goes, and the h-system's letters occur
// in real words too (flughaveno → fluĝaveno): when the text isn't right as
// shown but is one of the gap's answers in another reading (hfold), it is
// that answer — so the site (checkAnswer) gets the answer itself, judged
// alike. Else the text unchanged.
var i, k;
i = argument0;
if (gap_ok(i, argument1, argument2) || global.x_na[i] <= 0) return argument2;
if (global.x_kind[i] == 1) {
  if (hfold(argument2) == hfold(global.x_ans[i, argument1 mod global.x_na[i]])) return global.x_ans[i, argument1 mod global.x_na[i]];
  return argument2;
}
if (hfold(argument2) == hfold(global.x_ans[i, 0])) return global.x_ans[i, 0];
for (k = 0; k < global.x_ny[i]; k += 1) if (hfold(argument2) == hfold(global.x_alt[i, k])) return global.x_alt[i, k];
return argument2;
