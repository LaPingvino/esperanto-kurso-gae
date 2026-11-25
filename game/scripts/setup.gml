// Once per game: colours, the player's progress, the course — from the
// cache, else the built-in part — and the site asked for the current one.
var t;
// The colours: kurso-design's tokens.css (light), the Esperanto site's
// accent. Text is 7:1 or more on whatever it stands on; edges 3:1 (DESIGN.md).
global.C_bg = rgb(230, 242, 238);     // --paper
global.C_panel = rgb(250, 253, 252);  // --surface: cards, tiles, the top bar, the dock
global.C_sunk = rgb(211, 230, 224);   // --sunk: wells, a used tile, a track
global.C_text = rgb(16, 44, 40);      // --ink
global.C_muted = rgb(44, 74, 69);     // --ink-muted
global.C_line = rgb(191, 216, 209);   // --line: decorative dividers only
global.C_edge = rgb(95, 127, 121);    // --edge: tile, button and station edges
global.C_main = rgb(23, 99, 47);      // --eo-fill: the primary button, done stations
global.C_on = rgb(255, 255, 255);     // --eo-on-fill
global.C_link = rgb(15, 80, 36);      // --eo-text: accent text, the picked tile
global.C_tint = rgb(213, 237, 217);   // --eo-tint
global.C_acc = rgb(42, 128, 70);      // --eo-line: the open path, progress
global.C_ok = rgb(20, 82, 36);        // --right-text
global.C_okbg = rgb(216, 240, 220);   // --right-tint
global.C_okedge = rgb(47, 138, 69);   // --right-edge
global.C_bad = rgb(138, 35, 23);      // --wrong-text
global.C_badbg = rgb(251, 225, 218);  // --wrong-tint
global.C_badedge = rgb(196, 80, 60);  // --wrong-edge
global.C_star = rgb(198, 138, 0);     // --star (earned)
global.C_staredge = rgb(122, 85, 0);  // --star-edge
global.C_focus = rgb(23, 99, 47);     // the keyboard's ring: the accent
global.C_note = rgb(90, 58, 0);       // --note-text: Vortludo's "elsewhere in the word"
global.C_notebg = rgb(251, 235, 198); // --note-tint
global.C_pic = rgb(242, 247, 245);    // --pic-bg: behind a word picture
global.t_alpha = "abcĉdefgĝhĥijĵklmnoprsŝtuŭvz"; // the alphabet: Vortludo's letters and keyboard
global.ovf = 0; global.ovf_what = ""; // a text that didn't fit its box (overflow.gml)
course_data();                        // the interface strings and the built-in part
global.t_check = "Kontroli";          // the dock's button before the verdict (kurso-design)
global.nst = 0; global.ob_n = 0; global.nword = 0; global.mylang = "";
load_progress();
global.ns = 0; global.np = 0; global.nlang = 1; global.lang[0] = "en";
t = file_read_all("ludo-kurso.txt");
global.course_txt = "";
if (course_parse(t)) global.course_txt = t; else course_parse(global.fb_course);
global.nex = 0; global.npara = 0; global.les_slug = ""; global.les_kind = ""; global.les_title = ""; global.les_video = "";
global.scr = 5;       // 5 levels, 0 a series' map, 6 loading, 1 reading, 7 video, 2 exercises, 3 done, 4 words, 8 Vortludo, 9 Vetkuro, 10 their words loading
global.ser = -1; global.pl = -1;
global.focus = -1; global.nhs = 0; global.page = 0; global.mpage = 0; global.flash = 0;
global.typed = ""; global.fs = 15; global.lv_skip = 0; global.lv_next = -1;
global.fb = 0; global.kind = 0; global.idx = 0; global.qi = 0; global.qn = 0; global.qtotal = 0; global.qfirst = 0; global.got = 0;
global.def_act = -1; global.cg = 0; global.ngap = 0; global.bn = 0; global.pick = -1;
global.course_req = -1; global.les_req = -1; global.les_want = -1; global.les_fail = 0;
global.course_tried = 0; global.acclang = ""; global.want_slug = "";
// the bonus games (Vortludo 8, Vetkuro 9, their words loading 10)
global.rating = 1500; global.acclevel = ""; global.nw = 0; global.nd = 0; global.wd_req = -1; global.wd_asked = 0; global.wd_fresh = 0;
global.wd_lang = ""; global.wd_fail = 0; global.wd_day = ""; global.game = 0; global.ret_scr = 5; global.vl_daily = 1;
global.vl_state = 0; global.vl_row = 0; global.vl_cur = ""; global.rc_state = 0; global.rc_best = 0; global.rc_place = 0;
global.rc_pressed = 0; global.rc_scroll = 0;
keyboard_string = "";
sync_init();
global.ready = 1;
open_from_url();
