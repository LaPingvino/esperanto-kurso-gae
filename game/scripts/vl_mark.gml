// vl_mark(row): each letter of that guess: 2 in its place, 1 in the word
// elsewhere (as many times as the word has it left), 0 not in it. (Ten states a row: up to nine letters.)
var r, k, j, g, left;
r = argument0; g = global.vl_g[r];
left = global.vl_word;
for (k = 0; k < global.vl_n; k += 1) {
  global.vl_s[r * 10 + k] = 0;
  if (string_char_at(g, k + 1) == string_char_at(global.vl_word, k + 1)) {
    global.vl_s[r * 10 + k] = 2;
    left = string_delete(left, k + 1, 1); left = string_insert("_", left, k + 1);
  }
}
for (k = 0; k < global.vl_n; k += 1) if (global.vl_s[r * 10 + k] == 0) {
  j = string_pos(string_char_at(g, k + 1), left);
  if (j > 0) { global.vl_s[r * 10 + k] = 1; left = string_delete(left, j, 1); left = string_insert("_", left, j); }
}
