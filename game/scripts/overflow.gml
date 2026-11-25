// overflow(what): something didn't fit its box or the screen: kept in
// global.ovf (the tests expect 0) and global.ovf_what (which).
global.ovf += 1;
global.ovf_what = argument0;
show_debug_message("overflow: " + string(argument0));
