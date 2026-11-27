// mm_deal(): a new Memoro board: up to six words of the pool, no two asked
// alike (one picture, one definition: the pair would be two), each as two
// cards — the word, and its picture (else its masked definition) — face
// down and shuffled. Card c: mm_cp[c] its pair, mm_ck[c] 0 the word, 1 the
// cue; mm_st[c] 0 down, 1 up, 2 found.
var i, j, k, t, n, tries, pic, q, ok;
n = 0;
for (tries = 0; tries < 200 && n < 6 && n < global.mm_pn; tries += 1) {
  i = global.mm_p[floor(random(global.mm_pn))];
  pic = -1; q = "";
  if (global.w_pic[i] != "") pic = bildo_frame(global.w_pic[i]);
  if (pic < 0) q = global.w_def[i];
  ok = 1;
  for (k = 0; k < n; k += 1) {
    if (global.mm_w[k] == i || global.w_word[global.mm_w[k]] == global.w_word[i]) ok = 0;
    if (pic >= 0 && global.mm_pic[k] == pic) ok = 0;
    if (pic < 0 && global.mm_pic[k] < 0 && global.mm_q[k] == q) ok = 0;
  }
  if (!ok) continue;
  global.mm_w[n] = i; global.mm_pic[n] = pic; global.mm_q[n] = q;
  n += 1;
}
global.mm_n = n; global.mm_nc = 2 * n;
for (k = 0; k < global.mm_nc; k += 1) { global.mm_cp[k] = k div 2; global.mm_ck[k] = k mod 2; global.mm_st[k] = 0; }
for (k = global.mm_nc - 1; k > 0; k -= 1) { j = floor(random(k + 1)); t = global.mm_cp[k]; global.mm_cp[k] = global.mm_cp[j]; global.mm_cp[j] = t; t = global.mm_ck[k]; global.mm_ck[k] = global.mm_ck[j]; global.mm_ck[j] = t; }
global.mm_a = -1; global.mm_b = -1; global.mm_wait = 0; global.mm_found = 0; global.mm_tries = 0;
global.focus = -1; keyboard_string = ""; global.typed = "";
global.scr = 12;
