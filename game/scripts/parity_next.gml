// parity_next(): the next place's lesson, asked for (none left: done).
if (global.par_i >= global.np) { show_debug_message("parity: " + string(global.np) + " places"); global.par_done = 1; exit; }
global.par_req = http_get("/ludo-api/leciono/" + global.p_slug[global.par_i] + "?lingvo=en");
