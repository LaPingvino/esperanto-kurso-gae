// kb_built(): the word Konstrui's placed pieces spell (their hyphens off).
var s, k;
s = "";
for (k = 0; k < global.kb_n; k += 1) s += string_replace_all(global.kb_t[global.kb_bp[k]], "-", "");
return s;
