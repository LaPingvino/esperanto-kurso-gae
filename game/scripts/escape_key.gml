// escape_key(): Esc clears what was typed; else back to the map.
if (global.typed != "") { global.typed = ""; keyboard_string = ""; exit; }
if (global.scr != 0) { global.scr = 0; global.focus = -1; }
