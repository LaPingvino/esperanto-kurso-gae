// pv_drop(): a word of the pool starts falling, at a place across the
// field. Never one whose spelling starts another's falling (or the same):
// a word typed whole is then caught at once, never a longer one's start.
var i, f, g, k, ok, tries, w;
pv_geom();
ok = 0;
for (tries = 0; tries < 20 && !ok; tries += 1) {
  i = global.pv_p[floor(random(global.pv_pn))];
  f = hfold(global.w_word[i]); ok = 1;
  for (k = 0; k < global.pv_n; k += 1) {
    g = global.pv_f[k];
    if (string_copy(g, 1, string_length(f)) == f || string_copy(f, 1, string_length(g)) == g) ok = 0;
  }
}
if (!ok) exit;
font(20, 1, 0); w = string_width(global.w_word[i]) + 28;
k = global.pv_n;
global.pv_w[k] = i; global.pv_f[k] = f; global.pv_ww[k] = w;
global.pv_x[k] = global.px0 + floor(random(max(1, global.px1 - global.px0 - w)));
global.pv_y[k] = global.pt;
global.pv_n += 1; global.pv_k += 1;
