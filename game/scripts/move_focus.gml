// move_focus(step): the next or previous thing that can be pressed.
draw_all(); // the hotspots as they stand now
if (global.nhs <= 0) exit;
if (global.focus < 0) { if (argument0 > 0) global.focus = 0; else global.focus = global.nhs - 1; exit; }
global.focus = (global.focus + argument0 + global.nhs) mod global.nhs;
