// press_key(): Enter. Sends what was typed; else presses what has the
// keyboard's focus; else the screen's main button.
draw_all(); // the hotspots as they stand now
if (global.scr == 2 && global.kind == 1 && global.fb == 0 && strip(global.typed) != "") { fill_pick(strip(global.typed), 1); exit; }
if (global.focus >= 0 && global.focus < global.nhs) { act(global.hs_act[global.focus]); exit; }
if (global.def_act >= 0) act(global.def_act);
