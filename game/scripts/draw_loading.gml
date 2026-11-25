// draw_loading(): a place's lesson on its way from the site — or, when it
// can't come and there is no copy, saying so, with a try again.
var y;
y = global.hh + 60;
if (global.les_want >= 0) y += tw(global.m, y, global.p_title[global.les_want], global.W - 2 * global.m, 22, 1, global.C_text);
y += 40;
if (!global.les_fail) { txtc(global.W / 2, y, global.t_loading, 18, 0, global.C_muted); exit; }
y += tw(global.m, y, "La retejo ne respondas, kaj ĉi tiu leciono ne estas en via retumilo.", global.W - 2 * global.m, 16, 0, global.C_text) + 30;
button(global.W / 2 - bw("Provi denove") / 2, y, global.W / 2 + bw("Provi denove") / 2, y + 48, "Provi denove", 14, 1);
global.def_act = 14;
