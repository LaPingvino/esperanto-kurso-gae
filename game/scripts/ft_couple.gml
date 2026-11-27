// ft_couple(t): the depot's wagon t pressed: coupled behind the train; the
// last one coupled, the train is judged. A wrong train shown uncouples
// first (a press doesn't wait for it); a right one shown goes on.
if (global.ft_st == 1) ft_back();
if (global.ft_st == 2) { ft_go(); exit; }
if (global.ft_st != 0 || argument0 >= global.ft_nb) exit;
if (global.ft_used[argument0]) exit;
global.ft_used[argument0] = 1;
global.ft_bp[global.ft_n] = argument0;
global.ft_n += 1;
if (global.ft_n == global.ft_nb) ft_check();
