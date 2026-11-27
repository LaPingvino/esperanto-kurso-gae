// ft_round(): a new round of Frazotrajno: up to five sentences of the pool,
// none twice, the tally from nothing; the first one's train.
var i, k, n, tries, ok;
n = 0;
for (tries = 0; tries < 200 && n < 5 && n < global.ft_pn; tries += 1) {
  i = global.ft_p[floor(random(global.ft_pn))];
  ok = 1;
  for (k = 0; k < n; k += 1) if (global.ft_r[k] == i) ok = 0;
  if (ok) { global.ft_r[n] = i; n += 1; }
}
global.ft_nr = n; global.ft_i = 0; global.ft_ok = 0; global.ft_tries = 0;
ft_next();
global.scr = 13;
