// game_start(): the bonus game asked for (global.game), its words there.
if (global.nw == 0) { global.wd_fail = 1; global.scr = 10; exit; }
if (global.game == 1) vl_start(global.vl_daily);
if (global.game == 2) rc_ready();
if (global.game == 3) kb_ready();
if (global.game == 4) mm_ready();
if (global.game == 5) ft_ready();
