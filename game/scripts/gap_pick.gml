// gap_pick(text, typed): text into the current gap (typed: 1 typed, 0 a
// tile), then on to the next empty gap. With every gap filled, another
// pick or typed answer replaces the last gap's; Kontroli (ex_check) judges.
var g;
g = global.cg;
if (g >= global.ngap) g = global.ngap - 1;
global.fa[g] = argument0;
global.ftyped[g] = argument1;
gap_next();
global.typed = ""; keyboard_string = ""; global.focus = -1;
