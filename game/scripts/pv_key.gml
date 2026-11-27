// pv_key(s): a letter pressed on the screen's keyboard (or "" for Viŝi).
if (global.pv_st != 1) exit;
if (argument0 == "") pv_type(string_copy(global.pv_buf, 1, string_length(global.pv_buf) - 1));
else pv_type(pv_letters(global.pv_buf + argument0));
