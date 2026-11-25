// ex_done(right): the verdict, with the site's own sound for it (eo.js:
// snd_right, snd_wrong; the player's sound button silences them); a wrong
// one comes back at the end of the row (as the site keeps a series open
// until each item is right).
if (argument0) {
  global.fb = 1;
  if (global.q_again[global.qi] == 0) global.qfirst += 1;
  sound_play(snd_right);
} else {
  global.fb = 2;
  global.q_idx[global.qn] = global.idx; global.q_again[global.qn] = 1;
  global.qn += 1;
  sound_play(snd_wrong);
}
global.focus = -1;
global.flash = 12;
