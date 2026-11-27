// mm_ok(i): word i of w_* can be a pair of Memoro: the course's own (not
// derived for Konstrui), no word of it longer than a phone's card holds,
// with its picture, else an Esperanto definition short enough for a card —
// no translations (gamedata.MemoOK: 66 letters, none of its words over 12).
var s, p;
if (global.w_drv[argument0]) return 0;
if (!mm_short(global.w_word[argument0])) return 0;
if (global.w_pic[argument0] != "") { if (bildo_frame(global.w_pic[argument0]) >= 0) return 1; }
s = global.w_def[argument0];
return s != "" && string_length(s) <= 66 && mm_short(s);
