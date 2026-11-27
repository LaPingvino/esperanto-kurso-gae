// ft_uncouple(k): the train's k-th wagon pressed: back to the depot, the
// ones behind it closing up. On a train shown wrong or right: as ft_couple.
var i;
if (global.ft_st == 1) { ft_back(); exit; }
if (global.ft_st == 2) { ft_go(); exit; }
if (global.ft_st != 0 || argument0 >= global.ft_n) exit;
global.ft_used[global.ft_bp[argument0]] = 0;
for (i = argument0; i < global.ft_n - 1; i += 1) global.ft_bp[i] = global.ft_bp[i + 1];
global.ft_n -= 1;
