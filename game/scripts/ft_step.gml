// ft_step(): every step on Frazotrajno: a train shown wrong uncouples, one
// shown right goes on.
if (global.ft_wait > 0) {
  global.ft_wait -= 1;
  if (global.ft_wait == 0) { if (global.ft_st == 1) ft_back(); else if (global.ft_st == 2) ft_go(); }
}
