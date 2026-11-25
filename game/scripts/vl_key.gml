// vl_key(s): a letter pressed on the screen's keyboard (or "" for ⌫).
if (global.vl_state != 0 || global.vl_wait > 0) exit;
if (argument0 == "") global.vl_cur = string_copy(global.vl_cur, 1, string_length(global.vl_cur) - 1);
else if (string_length(global.vl_cur) < global.vl_n) global.vl_cur += argument0;
keyboard_string = global.vl_cur; global.typed = global.vl_cur; global.vl_msg = "";
