// mm_flip(c): card c pressed. Two cards up that were no pair turn back
// first (a press doesn't wait for them); a found card or the one already up
// does nothing. The second card up is a try: the same pair stays found,
// else both show wrong until the next press or a second and a half.
var c;
c = argument0;
if (c >= global.mm_nc || global.mm_found >= global.mm_n) exit;
if (global.mm_b >= 0) mm_hide();
if (global.mm_st[c] != 0) exit;
global.mm_st[c] = 1;
if (global.mm_a < 0) { global.mm_a = c; exit; }
global.mm_b = c; global.mm_tries += 1;
if (global.mm_cp[global.mm_a] == global.mm_cp[c]) {
  global.mm_st[global.mm_a] = 2; global.mm_st[c] = 2;
  global.mm_a = -1; global.mm_b = -1; global.mm_found += 1;
  if (global.mm_found >= global.mm_n) global.focus = -1; // the board done: Enter plays again
} else global.mm_wait = 45;
