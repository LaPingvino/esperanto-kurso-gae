// vl_submit(): Provi — the row's letters (vl_n of them) as a guess. A word
// of the course's list goes in at once (vl_try); another is asked of the
// site (/ludo-api/vorto: a ReVo word, its inflection, or known roots and
// affixes with an ending), and one it knows no word shakes the row and
// costs no try (net_answer). Without the site, or with no answer
// (offline, an error, two seconds gone: vl_wait in the step), any letters
// count.
var i, w, q, k, c, j;
if (global.vl_state != 0 || global.vl_wait > 0) exit;
if (string_length(global.vl_cur) != global.vl_n) { global.vl_msg = "Necesas " + vl_count(global.vl_n) + "."; exit; }
w = global.vl_cur;
if (!global.site) { vl_try(w); exit; }
for (i = 0; i < global.nw; i += 1) if (global.w_word[i] == w) { vl_try(w); exit; }
// in the x-system: plain letters in the address
q = "";
for (k = 1; k <= string_length(w); k += 1) {
  c = string_char_at(w, k);
  j = string_pos(c, "ĉĝĥĵŝŭ");
  if (j > 0) c = string_char_at("cghjsu", j) + "x";
  q += c;
}
global.vl_ask = w; global.vl_wait = 1;
global.vl_req = http_get("/ludo-api/vorto?v=" + q);
