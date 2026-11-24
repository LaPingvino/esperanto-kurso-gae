// xsys(s): the x-system as the site's answer field writes it (static/eo.js):
// cx → ĉ, gx → ĝ, hx → ĥ, jx → ĵ, sx → ŝ, ux → ŭ, keeping capitals.
var s;
s = argument0;
s = string_replace_all(s, "cx", "ĉ"); s = string_replace_all(s, "cX", "ĉ");
s = string_replace_all(s, "Cx", "Ĉ"); s = string_replace_all(s, "CX", "Ĉ");
s = string_replace_all(s, "gx", "ĝ"); s = string_replace_all(s, "gX", "ĝ");
s = string_replace_all(s, "Gx", "Ĝ"); s = string_replace_all(s, "GX", "Ĝ");
s = string_replace_all(s, "hx", "ĥ"); s = string_replace_all(s, "hX", "ĥ");
s = string_replace_all(s, "Hx", "Ĥ"); s = string_replace_all(s, "HX", "Ĥ");
s = string_replace_all(s, "jx", "ĵ"); s = string_replace_all(s, "jX", "ĵ");
s = string_replace_all(s, "Jx", "Ĵ"); s = string_replace_all(s, "JX", "Ĵ");
s = string_replace_all(s, "sx", "ŝ"); s = string_replace_all(s, "sX", "ŝ");
s = string_replace_all(s, "Sx", "Ŝ"); s = string_replace_all(s, "SX", "Ŝ");
s = string_replace_all(s, "ux", "ŭ"); s = string_replace_all(s, "uX", "ŭ");
s = string_replace_all(s, "Ux", "Ŭ"); s = string_replace_all(s, "UX", "Ŭ");
return s;
