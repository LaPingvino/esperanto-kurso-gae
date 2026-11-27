// pv_ok(i): word i of w_* can fall in Vortpluvo: the course's own (not
// derived for Konstrui), one word of the alphabet's small letters, two to
// twelve of them (gamedata.RainOK).
var s, k;
if (global.w_drv[argument0]) return 0;
s = global.w_word[argument0];
if (string_length(s) < 2 || string_length(s) > 12) return 0;
for (k = 1; k <= string_length(s); k += 1) if (string_pos(string_char_at(s, k), global.t_alpha) == 0) return 0;
return 1;
