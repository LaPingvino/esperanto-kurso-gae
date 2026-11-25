// act(a): what a pressed thing does. 1 map, 2 words, 3 next language, 4 the
// site, 5 start the exercises, 6 next exercise, 7 "I don't know", 8 type
// with the phone's keyboard, 9 / 10 next / previous page, 11 lesson done →
// map, 12 this lesson on the site; 100+l a lesson; 200+t a fill-in tile;
// 300+t a sentence tile; 400+k take a placed word back; 500+k a meaning.
var a, s, e;
a = argument0;
if (a == 1 || a == 11) { global.scr = 0; global.focus = -1; exit; }
if (a == 2) { global.scr = 4; global.page = 0; global.focus = -1; exit; }
if (a == 3) { global.lang_i = (global.lang_i + 1) mod global.nlang; save_progress(); exit; }
if (a == 4) { site_open("/"); exit; }
if (a == 5) { lesson_start(); exit; }
if (a == 6) { if (global.fb != 0) ex_next(); exit; }
if (a == 7) { if (global.fb == 0) ex_done(0); exit; }
if (a == 8) {
  if (global.fb == 0 && global.kind == 1) {
    s = get_string(global.t_answer_placeholder, "");
    if (strip(s) != "") fill_pick(xsys(s), 1);
  }
  exit;
}
if (a == 9) { global.page += 1; global.focus = -1; exit; }
if (a == 10) { global.page = max(0, global.page - 1); global.focus = -1; exit; }
if (a == 12) { site_open("/ekzerco/" + global.l_slug[global.les]); exit; }
if (a >= 100 && a < 200) { lesson_open(a - 100); exit; }
if (global.fb != 0) exit; // answered: only Daŭrigi
e = global.les * 100 + global.idx;
if (a >= 200 && a < 300) { fill_pick(global.f_tile[e, global.cg * 10 + a - 200], 0); exit; }
if (a >= 300 && a < 400) { build_place(a - 300); exit; }
if (a >= 400 && a < 500) { build_take(a - 400); exit; }
if (a >= 500 && a < 600) { global.pick = a - 500; ex_done(global.w_opt[global.idx, global.pick] == global.idx); exit; }
