// hfold(s): s as the h-system writes it, small letters (ĉ → ch, ĝ → gh,
// ĥ → hh, ĵ → jh, ŝ → sh, ŭ → u): two spellings of one word fold alike, so
// "flughaveno" typed (shown as fluĝaveno) is still flughaveno.
var s;
s = string_lower(strip(argument0));
s = string_replace_all(s, "ĉ", "ch"); s = string_replace_all(s, "ĝ", "gh"); s = string_replace_all(s, "ĥ", "hh");
s = string_replace_all(s, "ĵ", "jh"); s = string_replace_all(s, "ŝ", "sh"); s = string_replace_all(s, "ŭ", "u");
return s;
