// dock_tw(): the width of the verdict's text: beside its disc, and on a
// wide screen beside Daŭrigi too.
var w;
w = global.bx1 - global.bx0 - 44;
if (global.wide) w -= bw(global.t_continue_cta) + 44;
return w;
