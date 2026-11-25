// gap_answer(i): the right answer as the verdict shows it: a fill-in's
// sentence with its gaps filled, else the answer.
var i, t, s;
i = argument0;
if (global.x_kind[i] != 1) return global.x_ans[i, 0];
s = "";
for (t = 0; t < global.x_ntok[i]; t += 1) {
  if (global.x_tsp[i, t]) s += " ";
  if (global.x_tgap[i, t] < 0) s += global.x_tok[i, t];
  else s += global.x_ans[i, global.x_tgap[i, t] mod max(1, global.x_na[i])];
}
return s;
