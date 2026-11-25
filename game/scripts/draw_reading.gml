// draw_reading(): the lesson's text first (as on the site), then the exercises.
var x, y, w, h, p, bot, more, fs;
x = global.m; w = global.W - 2 * global.m;
if (global.wide) { w = min(w, 900); x = (global.W - w) / 2; }
y = global.hh + 20;
y += tw(x, y, global.les_title, w, 24, 1, global.C_text) + 12;
fs = 18; if (!global.wide) fs = 16;
bot = global.H - 96;
// what fits on this page, measured first: the card is as tall as its text
more = 0; h = 14;
for (p = global.page; p < global.npara; p += 1) {
  if (y + h + th(global.para[p], w - 40, fs, 0) > bot - 14 && p > global.page) { more = 1; break; }
  h += th(global.para[p], w - 40, fs, 0) + 12;
}
rfill(x - 4, y - 4, x + w + 4, y + h + 4, global.C_panel, global.C_line);
y += 14;
for (p = global.page; p < global.npara; p += 1) {
  h = th(global.para[p], w - 40, fs, 0);
  if (y + h > bot - 14 && p > global.page) break;
  tw(x + 20, y, global.para[p], w - 40, fs, 0, global.C_text);
  y += h + 12;
}
y = global.H - 76;
// Komenci is always there (a reader may start before the last page); Enter
// turns the page while there is more, then starts.
button(x + w - bw(global.t_start_cta) - 20, y, x + w, y + 52, global.t_start_cta, 5, 1);
global.def_act = 5;
if (more) { button(x + w - bw(global.t_start_cta) - 32 - bw(global.t_series_next_fallback), y, x + w - bw(global.t_start_cta) - 32, y + 52, global.t_series_next_fallback, 9, 0); global.def_act = 9; }
if (global.page > 0) button(x, y, x + bw(global.t_series_prev_fallback), y + 52, global.t_series_prev_fallback, 10, 0);
else if (global.wide || !more) button(x, y, x + bw("Vidi en la retejo"), y + 52, "Vidi en la retejo", 12, 5); // a phone: no room beside Sekva
