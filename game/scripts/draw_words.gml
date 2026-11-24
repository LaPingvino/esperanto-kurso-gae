// draw_words(): the collection: the words of every finished lesson.
var x, y, w, k, n, col, cw, rows, per, i, shown;
x = global.m; w = global.W - 2 * global.m;
y = global.hh + 20;
n = 0;
for (i = 0; i < global.nwords; i += 1) if (global.stars[global.w_lesson[i]] > 0) { global.coll[n] = i; n += 1; }
txt(x, y, "Viaj vortoj (" + string(n) + " el " + string(global.nwords) + ")", 22, 1, global.C_text);
y += 44;
if (n == 0) { tw(x, y, "Finu lecionon por kolekti ĝiajn vortojn.", w, 17, 0, global.C_muted); exit; }
col = 1; if (global.wide) col = 3;
cw = w / col;
rows = floor((global.H - 110 - y) / 28);
per = rows * col;
if (global.page * per >= n) global.page = 0;
shown = 0;
for (k = global.page * per; k < n && shown < per; k += 1) {
  i = global.coll[k];
  font(15, 1, 0);
  txt(x + (shown div rows) * cw, y + (shown mod rows) * 28, global.w_base[i], 15, 1, global.C_text);
  txt(x + (shown div rows) * cw + string_width(global.w_base[i]) + 8, y + (shown mod rows) * 28, "— " + global.w_def[i, global.lang_i], 15, 0, global.C_muted);
  shown += 1;
}
y = global.H - 72;
if (global.page > 0) button(x, y, x + bw(global.t_series_prev_fallback), y + 48, global.t_series_prev_fallback, 10, 0);
if ((global.page + 1) * per < n) { button(global.W - global.m - bw(global.t_series_next_fallback), y, global.W - global.m, y + 48, global.t_series_next_fallback, 9, 0); global.def_act = 9; }
