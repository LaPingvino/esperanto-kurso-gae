// ft_back(): a train shown wrong: the wagons from the first one out of
// place on go back to the depot; the right start stays coupled.
var k;
for (k = global.ft_good; k < global.ft_n; k += 1) global.ft_used[global.ft_bp[k]] = 0;
global.ft_n = global.ft_good;
global.ft_st = 0; global.ft_wait = 0;
