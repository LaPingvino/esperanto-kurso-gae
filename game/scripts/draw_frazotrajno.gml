// draw_frazotrajno(): Frazotrajno — a sentence of the course as a train to
// couple: the depot's wagons (the sentence's words, shuffled) go behind the
// locomotive one by one, a tap or the arrows and Enter (or a number key);
// a coupled one goes back when pressed. The last one coupled, the train is
// judged: right, it shows the sentence and goes on; wrong, the wagons from
// the first one out of place show ✗ and come back. The depot is drawn
// first, every slot pressable, so the arrow keys keep their place.
var x0, x1, y, t, p, top, bot, x, w, h, rowh, k;
x0 = global.m; x1 = global.W - global.m;
if (global.wide) { x0 = (global.W - 640) / 2; x1 = x0 + 640; }
y = global.hh + 12;
txt(x0, y, "Frazotrajno · " + level_me(), 17, 1, global.C_text);
p = " provoj"; if (global.ft_tries == 1) p = " provo";
t = string(min(global.ft_i + 1, global.ft_nr)) + "/" + string(global.ft_nr) + " · " + string(global.ft_tries) + p;
font(15, 0, 0); txt(x1 - string_width(t), y + 2, t, 15, 0, global.C_muted);
y += 40; top = y;
bot = ft_train(x0, x1, top, 0);
y = bot + 2;
if (global.ft_st >= 2) { t = global.ft_s + "."; fits(t, x1 - x0, 18, 1); tw(x0, y, t, x1 - x0, 18, 1, global.C_link); }
y += th(global.ft_s + ".", x1 - x0, 18, 1) + 16;
// the depot
x = x0; rowh = 0;
for (k = 0; k < global.ft_nb; k += 1) {
  w = min(tile_w(global.ft_t[k], 18, 0), x1 - x0);
  if (x + w > x1 && x > x0) { x = x0; y += rowh + 10; rowh = 0; }
  if (global.ft_used[k]) h = tilebox(x, y, w, global.ft_t[k], -1, 5, 18, 0);
  else if (global.ft_st != 0) h = tilebox(x, y, w, global.ft_t[k], -1, 6, 18, 0);
  else h = tilebox(x, y, w, global.ft_t[k], -1, 0, 18, 0);
  hs(x, y, x + w, y + h, 880 + k);
  rowh = max(rowh, h);
  x += w + 10;
}
y += rowh + 16;
ft_train(x0, x1, top, 1);
if (global.ft_st == 1 || global.ft_st == 2) global.def_act = 853; // Enter: on at once
if (global.ft_st == 3) { // the round done
  box(x0, y, x1, y + 72, global.C_okbg, global.C_okedge, 2);
  mark(x0 + 26, y + 36, 11, 1);
  t = "Finita! " + string(global.ft_tries) + p;
  txt(x0 + 46, y + 25, fit(t, x1 - x0 - 60 - bw("Denove"), 16, 1), 16, 1, global.C_text);
  button(x1 - 14 - bw("Denove"), y + 14, x1 - 14, y + 58, "Denove", 852, 1);
  global.def_act = 852;
  y += 72;
}
if (y > global.H - 8) overflow("Frazotrajno runs off the screen");
