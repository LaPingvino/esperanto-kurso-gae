// fill_answer(lesson, exercise): the sentence with the right answers in its gaps.
var e, t, s, n;
e = argument0 * 100 + argument1;
n = global.f_nans[argument0, argument1];
s = "";
for (t = 0; t < global.f_ntok[argument0, argument1]; t += 1) {
  if (global.f_tsp[e, t]) s += " ";
  if (global.f_tgap[e, t] < 0) s += global.f_tok[e, t];
  else s += global.f_ans[e, global.f_tgap[e, t] mod n];
}
return s;
