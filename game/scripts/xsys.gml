// xsys(s): the x-system as the site's answer field writes it (static/eo.js):
// cx → ĉ, gx → ĝ, hx → ĥ, jx → ĵ, sx → ŝ, ux → ŭ, keeping capitals; then the
// h-system: ch → ĉ, gh → ĝ, hh → ĥ, jh → ĵ, sh → ŝ (its ŭ is a plain u). The
// h-system's letters also occur in real words (flughaveno, dishaki), so a
// typed answer is judged by both readings (hfold, gap_fix).
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
s = string_replace_all(s, "ch", "ĉ"); s = string_replace_all(s, "cH", "ĉ");
s = string_replace_all(s, "Ch", "Ĉ"); s = string_replace_all(s, "CH", "Ĉ");
s = string_replace_all(s, "gh", "ĝ"); s = string_replace_all(s, "gH", "ĝ");
s = string_replace_all(s, "Gh", "Ĝ"); s = string_replace_all(s, "GH", "Ĝ");
s = string_replace_all(s, "hh", "ĥ"); s = string_replace_all(s, "hH", "ĥ");
s = string_replace_all(s, "Hh", "Ĥ"); s = string_replace_all(s, "HH", "Ĥ");
s = string_replace_all(s, "jh", "ĵ"); s = string_replace_all(s, "jH", "ĵ");
s = string_replace_all(s, "Jh", "Ĵ"); s = string_replace_all(s, "JH", "Ĵ");
s = string_replace_all(s, "sh", "ŝ"); s = string_replace_all(s, "sH", "ŝ");
s = string_replace_all(s, "Sh", "Ŝ"); s = string_replace_all(s, "SH", "Ŝ");
return s;
