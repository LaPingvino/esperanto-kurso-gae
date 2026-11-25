// The game's state lives in globals, so it survives the switch between the
// wide room and the tall one (a phone held upright).
if (!variable_global_exists("ready")) { setup(); global.par_done = 0; global.par_req = -1; if (string(PARITY) == "1") parity_dump(); }
