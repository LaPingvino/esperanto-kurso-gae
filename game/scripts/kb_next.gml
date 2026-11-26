// kb_next(): Konstrui's next word (another than the last) and its bank: the
// word's pieces (w_blk: "mal-|san|-ul-|-ej-|-o") with plausible others of
// the same families — another word's root, an affix of the same kind and
// a suffix, another ending and -j or -n — shuffled. Asked by the word
// itself: its picture, else its Esperanto definition, else (kb_tr) its
// meaning marked with its language.
var s, p, i, k, t, n, fam, root, w, tries;
w = global.kb_p[floor(random(global.kb_pn))];
for (tries = 0; tries < 8 && global.kb_pn > 1 && w == global.kb_w; tries += 1) w = global.kb_p[floor(random(global.kb_pn))];
global.kb_w = w; global.kb_word = global.w_word[w];
global.kb_nb = 0; global.kb_nt = 0; root = ""; fam = "";
s = global.w_blk[w] + "|";
while (s != "") {
  p = string_pos("|", s);
  t = string_copy(s, 1, p - 1); s = string_delete(s, 1, p);
  global.kb_b[global.kb_nb] = t; global.kb_nb += 1;
  global.kb_t[global.kb_nt] = t; global.kb_nt += 1;
  if (string_pos("-", t) == 0) root = t;
  if (fam == "" && string_char_at(t, string_length(t)) == "-") { fam = "s"; if (string_char_at(t, 1) != "-") fam = "p"; }
}
// another word's root
for (tries = 0; tries < 12; tries += 1) {
  s = global.w_blk[global.kb_p[floor(random(global.kb_pn))]] + "|";
  while (s != "") { p = string_pos("|", s); t = string_copy(s, 1, p - 1); s = string_delete(s, 1, p); if (string_pos("-", t) == 0) break; }
  if (t != root && string_pos("-", t) == 0) { kb_tadd(t); break; }
}
// an affix of the word's first affix's kind, and a suffix
n = 0;
for (tries = 0; tries < 30 && n < 2; tries += 1) {
  if (n == 0 && fam == "p") { t = "mal-|ge-|re-|ek-|dis-|mis-"; k = 6; }
  else { t = "-ul-|-ej-|-in-|-et-|-eg-|-il-|-ist-|-ar-|-an-|-aĵ-|-ec-|-ig-|-iĝ-|-ad-|-er-|-uj-|-em-|-ebl-"; k = 18; }
  i = floor(random(k));
  t += "|";
  while (i > 0) { t = string_delete(t, 1, string_pos("|", t)); i -= 1; }
  n += kb_tadd(string_copy(t, 1, string_pos("|", t) - 1));
}
// another ending, and -j or -n
for (tries = 0; tries < 12; tries += 1) if (kb_tadd("-" + string_char_at("oaei", floor(random(4)) + 1))) break;
if (random(2) < 1) kb_tadd("-j"); else kb_tadd("-n");
for (k = global.kb_nt - 1; k > 0; k -= 1) { i = floor(random(k + 1)); t = global.kb_t[k]; global.kb_t[k] = global.kb_t[i]; global.kb_t[i] = t; }
for (k = 0; k < global.kb_nt; k += 1) global.kb_used[k] = 0;
global.kb_n = 0;
// the question
global.kb_pic = -1; global.kb_q = ""; global.kb_tr = 0;
if (global.w_pic[w] != "") global.kb_pic = bildo_frame(global.w_pic[w]);
if (global.kb_pic < 0) global.kb_q = global.w_def[w];
if (global.kb_pic < 0 && global.kb_q == "") {
  global.kb_q = global.w_mean[w]; global.kb_tr = 1;
  if (!(string_char_at(global.kb_q, 1) == "(" && string_pos(") ", global.kb_q) > 2 && string_pos(") ", global.kb_q) <= 5)) global.kb_q = "(" + lang_cur() + ") " + global.kb_q;
}
global.fb = 0; global.focus = -1;
keyboard_string = ""; global.typed = "";
global.scr = 11;
