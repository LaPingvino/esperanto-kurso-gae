// ft_go(): a train shown right goes on: the round's next sentence, or the
// round done (Enter then plays again).
global.ft_wait = 0;
global.ft_i += 1;
if (global.ft_i < global.ft_nr) { ft_next(); exit; }
global.ft_i = global.ft_nr; global.ft_st = 3; global.focus = -1;
