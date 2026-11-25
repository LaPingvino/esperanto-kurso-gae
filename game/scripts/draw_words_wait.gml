// draw_words_wait(): the bonus games' words on their way from the site —
// or, when they can't come and none are kept, saying so, with a try again.
var y;
y = global.hh + 60;
if (!global.wd_fail) { txtc(global.W / 2, y, global.t_loading, 18, 0, global.C_muted); exit; }
y += tw(global.m, y, "La ludoj prenas siajn vortojn de la retejo, kiu nun ne respondas.", global.W - 2 * global.m, 16, 0, global.C_text) + 30;
button(global.W / 2 - bw("Provi denove") / 2, y, global.W / 2 + bw("Provi denove") / 2, y + 48, "Provi denove", 26, 1);
global.def_act = 26;
