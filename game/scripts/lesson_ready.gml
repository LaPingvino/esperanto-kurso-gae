// lesson_ready(): the lesson is in: its text first (as on the site), a
// video's own screen, or straight to the exercises.
global.page = 0; global.focus = -1;
if (global.les_video != "") { global.scr = 7; exit; }
if (global.npara > 0) { global.scr = 1; exit; }
lesson_start();
