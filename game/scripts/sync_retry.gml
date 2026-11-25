// sync_retry(): wait, then try again; each failure waits twice as long (at most a minute).
global.sync_ok = 0;
global.sync_wait = global.sync_back;
global.sync_back = min(global.sync_back * 2, 1800);
