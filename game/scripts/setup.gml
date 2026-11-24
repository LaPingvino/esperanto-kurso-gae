// Once per game: colours, the course, the player's progress.
var i;
global.C_bg = rgb(246, 241, 231);     // warm paper
global.C_panel = rgb(255, 253, 248);  // cards and tiles
global.C_line = rgb(150, 136, 112);   // their edges (3:1 on the paper, WCAG 1.4.11)
global.C_text = rgb(34, 45, 54);
global.C_muted = rgb(88, 98, 106);
global.C_main = rgb(26, 102, 84);     // the course's green: buttons, the open path
global.C_main2 = rgb(19, 79, 65);     // header buttons
global.C_on = rgb(255, 255, 255);     // text on green
global.C_link = rgb(26, 92, 76);
global.C_ok = rgb(30, 110, 52);
global.C_okbg = rgb(224, 242, 228);
global.C_bad = rgb(168, 50, 38);
global.C_badbg = rgb(251, 229, 224);
global.C_star = rgb(180, 115, 0);     // earned stars, 3.5:1 on the paper
global.C_lock = rgb(214, 208, 198);   // a lesson not open yet
global.C_path = rgb(190, 182, 168);   // the path to it
global.C_focus = rgb(36, 84, 196);    // the keyboard's ring
global.C_gap = rgb(232, 242, 238);    // an open gap
course_data();
for (i = 0; i < global.nl; i += 1) global.stars[i] = 0;
global.lang_i = 0;
load_progress();
global.scr = 0;       // 0 map, 1 reading, 2 exercises, 3 lesson done, 4 words
global.les = -1;
global.focus = -1;
global.nhs = 0;
global.page = 0;
global.flash = 0;
global.typed = "";
global.fs = 15;       // button text size
global.fb = 0; global.kind = 0; global.idx = 0; global.qi = 0; global.qn = 0; global.qtotal = 0; global.qfirst = 0; global.got = 0;
global.def_act = -1; global.np = 0; global.cg = 0; global.ngap = 0; global.bn = 0; global.pick = -1;
keyboard_string = "";
global.ready = 1;
