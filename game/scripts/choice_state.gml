// choice_state(k): option k's tile state (tilebox): trying when picked;
// after Kontroli right, a wrong pick, or idle (out of play).
var st;
if (global.fb == 0) { if (argument0 == global.pick) return 1; return 0; }
st = 6;
if (argument0 == global.x_right[global.idx]) st = 2;
if (global.fb == 2) { if (argument0 == global.pick) st = 3; }
return st;
