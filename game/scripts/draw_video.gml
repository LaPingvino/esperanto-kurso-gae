// draw_video(): a video of the course: watched on the site (a new tab);
// back here, the place is done.
var y, w;
y = global.hh + 40;
y += tw(global.m, y, global.les_title, global.W - 2 * global.m, 24, 1, global.C_text) + 30;
tw(global.m, y, "Ĉi tiu video estas en la retejo. Spektu ĝin tie; la ludo restas malfermita ĉi tie.", global.W - 2 * global.m, 17, 0, global.C_muted);
y += 90;
w = bw("Spekti en la retejo");
button(global.W / 2 - w / 2 - 10, y, global.W / 2 + w / 2 + 10, y + 52, "Spekti en la retejo", 13, 1);
global.def_act = 13;
