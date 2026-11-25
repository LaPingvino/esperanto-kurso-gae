// escape_key(): Esc clears what was typed; else back (act 1).
if (global.typed != "") { global.typed = ""; keyboard_string = ""; exit; }
if (global.scr != 5) act(1);
