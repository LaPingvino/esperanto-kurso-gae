// act(a): what a pressed thing does. 1 back (a series' map → the levels;
// elsewhere → the series' map), 2 the words, 3 the meanings' language, 4
// the site, 5 start the exercises, 6 next exercise, 7 "I don't know", 8
// type with the phone's keyboard, 9 / 10 next / previous page, 11 place
// done → its map, 12 this place on the site, 13 watch the video (on the
// site), 14 try loading again, 15 / 16 the map's next / previous page,
// 17 / 18 the levels' first / next page, 19 Kontroli;
// 200+t a tile; 300+t a sentence's tile; 400+k take a placed word back;
// 500+k a choice; 600+g empty a filled gap; 10000+p a place; 20000+s a series.
var a, s;
a = argument0;
if (a == 1) {
  global.focus = -1; global.les_want = -1;
  if (global.scr >= 8) { global.scr = global.ret_scr; global.rc_place = 0; global.mm_place = 0; global.ft_place = 0; global.pv_place = 0; exit; } // a bonus game: back where it came from
  if (global.scr == 0 || global.ser < 0) { global.scr = 5; exit; }
  global.scr = 0; exit;
}
if (a == 11) { global.scr = 0; global.focus = -1; exit; }
if (a == 2) { global.scr = 4; global.page = 0; global.focus = -1; exit; }
if (a == 3) { global.mylang = global.lang[(lang_index() + 1) mod global.nlang]; save_progress(); exit; }
if (a == 4) { site_open("/"); exit; }
if (a == 5) { lesson_start(); exit; }
if (a == 6) { if (global.fb != 0) { if (global.scr == 11) kb_next(); else ex_next(); } exit; }
if (a == 7) { if (global.fb == 0) { if (global.scr == 11) kb_check(1); else ex_done(0); } exit; }
if (a == 19) { if (global.scr == 11) kb_check(0); else ex_check(); exit; }
if (a == 8) {
  if (global.fb == 0 && global.ngap > 0) {
    s = get_string(global.t_answer_placeholder, "");
    if (strip(s) != "") gap_pick(xsys(s), 1);
  }
  exit;
}
if (a == 9) { global.page += 1; global.focus = -1; exit; }
if (a == 10) { global.page = max(0, global.page - 1); global.focus = -1; exit; }
if (a == 12) { site_open("/ekzerco/" + place_site_slug()); exit; }
if (a == 13) { url_open("/ekzerco/" + global.les_video); lesson_done(); exit; }
if (a == 14) { global.les_fail = 0; exit; }
if (a == 17) { global.lv_skip = 0; global.focus = -1; exit; }
if (a == 18) { if (global.lv_next >= 0) global.lv_skip = global.lv_next; global.focus = -1; exit; }
if (a == 15) { global.mpage += 1; global.focus = -1; exit; }
if (a == 16) { global.mpage = max(0, global.mpage - 1); global.focus = -1; exit; }
// the bonus games: 700+k a letter of Vortludo's keyboard, 730 ⌫, 731 Provi,
// 732 / 733 / 734 the hint (picture, definition, meaning), 735 a new word,
// 736 the day's word; 740 / 741 Vortludo / Vetkuro from the levels, 743 /
// 744 the same as the bonus after a place; 742 the race's start; 26 their
// words again. Konstrui: 745 from the levels, 746 after a place; 760+k a
// placed piece back, 780+t a piece of the bank onto the line. Memoro: 747
// from the levels, 748 after a place, 749 a new board, 800+c a card.
// Frazotrajno: 850 from the levels, 851 after a place, 852 a new round, 853
// a judged train on at once, 860+k the train's wagon back, 880+t the
// depot's wagon coupled. Vortpluvo: 940 from the levels, 941 after a
// place, 942 Komenci or Denove, 900+k the screen's letter k, 930 Viŝi,
// 931 the typed letters cleared.
if (a >= 700 && a < 730) { vl_key(string_char_at(global.t_alpha, a - 700 + 1)); exit; }
if (a == 730) { vl_key(""); exit; }
if (a == 731) { vl_submit(); exit; }
if (a >= 732 && a <= 734) { global.vl_hint = a - 731; exit; }
if (a == 735) { vl_start(0); exit; }
if (a == 736) { vl_start(1); exit; }
if (a == 740 || a == 741) { global.ret_scr = 5; global.rc_place = 0; global.vl_daily = 1; words_open(a - 739); exit; }
if (a == 743 || a == 744) { global.ret_scr = 3; global.rc_place = (a == 744); global.vl_daily = 0; words_open(a - 742); exit; }
if (a == 742) { global.rc_lives = 3; global.rc_score = 0; global.rc_n = 0; global.rc_crash = 0; global.rc_lane = 1; rc_next(); global.focus = -1; exit; }
if (a == 745 || a == 746) { global.ret_scr = 5 - 2 * (a == 746); global.kb_place = (a == 746); words_open(3); exit; }
if (a >= 760 && a < 780) { kb_take(a - 760); exit; }
if (a >= 780 && a < 800) { kb_place(a - 780); exit; }
if (a == 747 || a == 748) { global.ret_scr = 5 - 2 * (a == 748); global.mm_place = (a == 748); words_open(4); exit; }
if (a == 749) { mm_deal(); exit; }
if (a >= 800 && a < 840) { mm_flip(a - 800); exit; }
if (a == 850 || a == 851) { global.ret_scr = 5 - 2 * (a == 851); global.ft_place = (a == 851); words_open(5); exit; }
if (a == 852) { ft_round(); exit; }
if (a == 853) { if (global.ft_st == 1) ft_back(); else if (global.ft_st == 2) ft_go(); exit; }
if (a >= 860 && a < 880) { ft_uncouple(a - 860); exit; }
if (a >= 880 && a < 900) { ft_couple(a - 880); exit; }
if (a == 940 || a == 941) { global.ret_scr = 5 - 2 * (a == 941); global.pv_place = (a == 941); words_open(6); exit; }
if (a == 942) { pv_new(); global.pv_st = 1; exit; }
if (a >= 900 && a < 928) { pv_key(string_char_at(global.t_alpha, a - 900 + 1)); exit; }
if (a == 930) { pv_key(""); exit; }
if (a == 931) { pv_type(""); exit; }
if (a == 26) { global.wd_fail = 0; words_open(global.game); exit; }
if (a >= 20000) { // a series: its map, at the page of the place to go on with
  global.ser = a - 20000; global.mpage = 0; global.scr = 0; global.focus = -1;
  for (s = 0; s < global.s_np[global.ser]; s += 1) if (stars_get(global.p_slug[global.s_first[global.ser] + s]) == 0) { global.mpage = s div 12; break; }
  exit;
}
if (a >= 10000) { lesson_open(a - 10000); exit; }
if (global.fb != 0 || global.scr != 2) exit; // answered: only Daŭrigi
if (a >= 200 && a < 300) { gap_pick(global.x_tile[global.idx, min(global.cg, global.ngap - 1) * 10 + a - 200], 0); exit; }
if (a >= 300 && a < 400) { build_place(a - 300); exit; }
if (a >= 400 && a < 500) { build_take(a - 400); exit; }
if (a >= 500 && a < 600) { global.pick = a - 500; global.focus = -1; exit; } // picked: Kontroli judges
if (a >= 600 && a < 700) { gap_clear(a - 600); exit; }
