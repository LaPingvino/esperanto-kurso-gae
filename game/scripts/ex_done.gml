// ex_done(right): judge the exercise; a wrong one comes back at the end of
// the row (as the site keeps a series open until each item is right).
if (argument0) {
  global.fb = 1;
  if (global.q_again[global.qi] == 0) global.qfirst += 1;
} else {
  global.fb = 2;
  global.q_kind[global.qn] = global.kind; global.q_idx[global.qn] = global.idx; global.q_again[global.qn] = 1;
  global.qn += 1;
}
global.focus = -1;
global.flash = 12;
