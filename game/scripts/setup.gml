// Once per game: colours, the player's progress, the course — from the
// cache, else the built-in part — and the site asked for the current one.
var t;
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
global.C_lock = rgb(214, 208, 198);   // a place not open yet
global.C_path = rgb(190, 182, 168);   // the path to it
global.C_focus = rgb(36, 84, 196);    // the keyboard's ring
global.C_gap = rgb(232, 242, 238);    // an open gap
course_data();                        // the interface strings and the built-in part
global.nst = 0; global.ob_n = 0; global.nword = 0; global.mylang = "";
load_progress();
global.ns = 0; global.np = 0; global.nlang = 1; global.lang[0] = "en";
t = file_read_all("ludo-kurso.txt");
global.course_txt = "";
if (course_parse(t)) global.course_txt = t; else course_parse(global.fb_course);
global.nex = 0; global.npara = 0; global.les_slug = ""; global.les_kind = ""; global.les_title = ""; global.les_video = "";
global.scr = 5;       // 5 levels, 0 a series' map, 6 loading, 1 reading, 7 video, 2 exercises, 3 done, 4 words
global.ser = -1; global.pl = -1;
global.focus = -1; global.nhs = 0; global.page = 0; global.mpage = 0; global.flash = 0;
global.typed = ""; global.fs = 15; global.lv_skip = 0; global.lv_next = -1;
global.fb = 0; global.kind = 0; global.idx = 0; global.qi = 0; global.qn = 0; global.qtotal = 0; global.qfirst = 0; global.got = 0;
global.def_act = -1; global.cg = 0; global.ngap = 0; global.bn = 0; global.pick = -1;
global.course_req = -1; global.les_req = -1; global.les_want = -1; global.les_fail = 0;
global.course_tried = 0; global.acclang = ""; global.want_slug = "";
keyboard_string = "";
sync_init();
global.ready = 1;
open_from_url();
