// rc_next(): the next question (a meaning) and its gate: the right word
// in one lane, two others in the rest; its time down the road from the
// learner's rating (1000 or less: 9 s; 2000: 5 s), a little less with each
// point, never under 3.5 s.
var q, k, o, t;
q = floor(random(global.rp_n));
if (q == global.rc_q) q = (q + 1) mod global.rp_n;
global.rc_q = q;
global.rc_right = floor(random(3));
global.rc_opt[0] = -1; global.rc_opt[1] = -1; global.rc_opt[2] = -1;
for (k = 0; k < 3; k += 1) {
  if (k == global.rc_right) { global.rc_opt[k] = q; continue; }
  o = floor(random(global.rp_n));
  while (o == q || o == global.rc_opt[0] || o == global.rc_opt[1]) o = (o + 1) mod global.rp_n;
  global.rc_opt[k] = o;
}
rc_geom();
global.rc_y = -56;
t = min(9, max(5, 9 - (global.rating - 1000) / 250)) - 0.15 * global.rc_score;
t = max(3.5, t);
global.rc_speed = (global.cy - global.ry0 + 56) / (t * 30); // the rooms run at 30 steps a second
global.rc_state = 1;
