// ft_built(train): the words joined by spaces — of the train as coupled
// (train 1), else of the depot in its order.
var s, k;
s = "";
if (argument0) { for (k = 0; k < global.ft_n; k += 1) { if (k > 0) s += " "; s += global.ft_t[global.ft_bp[k]]; } }
else { for (k = 0; k < global.ft_nb; k += 1) { if (k > 0) s += " "; s += global.ft_t[k]; } }
return s;
