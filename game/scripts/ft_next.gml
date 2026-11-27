// ft_next(): the round's current sentence as wagons: its words (ft_b, split
// at spaces) shuffled into the depot (ft_t), never in the text's order; the
// train (ft_bp: depot indexes, ft_n of them) empty. ft_st: 0 coupling, 1
// wrong shown, 2 right shown, 3 the round done.
var s, p, k, j, t;
global.ft_s = global.f_text[global.ft_r[global.ft_i]];
global.ft_nb = 0;
s = global.ft_s + " ";
while (s != "") {
  p = string_pos(" ", s);
  if (p > 1) { global.ft_b[global.ft_nb] = string_copy(s, 1, p - 1); global.ft_nb += 1; }
  s = string_delete(s, 1, p);
}
for (k = 0; k < global.ft_nb; k += 1) { global.ft_t[k] = global.ft_b[k]; global.ft_used[k] = 0; }
for (k = global.ft_nb - 1; k > 0; k -= 1) { j = floor(random(k + 1)); t = global.ft_t[k]; global.ft_t[k] = global.ft_t[j]; global.ft_t[j] = t; }
if (ft_built(0) == global.ft_s) { t = global.ft_t[0]; for (k = 0; k < global.ft_nb - 1; k += 1) global.ft_t[k] = global.ft_t[k + 1]; global.ft_t[global.ft_nb - 1] = t; }
global.ft_n = 0; global.ft_st = 0; global.ft_good = 0; global.ft_wait = 0;
global.focus = -1; keyboard_string = ""; global.typed = "";
