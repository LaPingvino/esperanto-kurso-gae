// game_tile(x, y, w, which): a bonus game's tile on the levels screen:
// 0 Vortludo (three cells in its states), 1 Vetkuro (the car), 2 Konstrui
// (three pieces joined into a word), 3 Memoro (a card down, a pair found),
// 4 Frazotrajno (a locomotive and two wagons), 5 Vortpluvo (words falling
// to the ground), 6 Bonvorto (a word's two cells, the second the accent's).
var x, y, w, l, s, iw;
x = argument0; y = argument1; w = argument2; iw = 64;
box(x, y, x + w, y + 64, global.C_panel, global.C_edge, 1);
if (argument3 == 0) {
  l = "Vortludo"; s = "la vorto de la tago";
  rfill(x + w - 64, y + 12, x + w - 48, y + 28, global.C_main, global.C_main);
  rfill(x + w - 44, y + 12, x + w - 28, y + 28, global.C_notebg, global.C_note);
  rfill(x + w - 24, y + 12, x + w - 8, y + 28, global.C_sunk, global.C_edge);
} else if (argument3 == 2) {
  l = "Konstrui"; s = "vorto el pecoj";
  rfill(x + w - 66, y + 12, x + w - 50, y + 30, global.C_panel, global.C_edge);
  rfill(x + w - 48, y + 12, x + w - 28, y + 30, global.C_tint, global.C_main);
  rfill(x + w - 26, y + 12, x + w - 12, y + 30, global.C_okbg, global.C_okedge);
} else if (argument3 == 3) {
  l = "Memoro"; s = "trovu la parojn";
  rfill(x + w - 62, y + 10, x + w - 40, y + 34, global.C_main, global.C_main);
  rfill(x + w - 34, y + 10, x + w - 12, y + 34, global.C_okbg, global.C_okedge);
} else if (argument3 == 4) {
  l = "Frazotrajno"; s = "vagonoj en ordo"; iw = 40; // a locomotive and a wagon
  thick(x + w - 40, y + 33, x + w - 8, y + 33, 2, global.C_edge);
  rfill(x + w - 40, y + 12, x + w - 26, y + 29, global.C_main, global.C_main);
  rfill(x + w - 23, y + 15, x + w - 8, y + 29, global.C_okbg, global.C_okedge);
} else if (argument3 == 6) {
  l = "Bonvorto"; s = "el bazaj radikoj"; iw = 48; // a word crossed for a built one
  rfill(x + w - 48, y + 12, x + w - 30, y + 30, global.C_sunk, global.C_edge);
  thick(x + w - 46, y + 21, x + w - 32, y + 21, 2, global.C_muted);
  rfill(x + w - 26, y + 12, x + w - 8, y + 30, global.C_main, global.C_main);
} else if (argument3 == 5) {
  l = "Vortpluvo"; s = "tajpu la vortojn"; iw = 48; // two words falling
  rfill(x + w - 48, y + 8, x + w - 22, y + 19, global.C_panel, global.C_edge);
  rfill(x + w - 36, y + 23, x + w - 10, y + 34, global.C_tint, global.C_main);
} else {
  l = "Vetkuro"; s = "stiru al la vorto";
  rfill(x + w - 28, y + 8, x + w - 12, y + 32, global.C_main, global.C_main);
  rfill(x + w - 26, y + 12, x + w - 14, y + 17, global.C_tint, global.C_tint);
}
font(17, 1, 0); if (string_width(l) > w - 20 - iw) overflow("a game's name on its tile: " + l);
txt(x + 12, y + 10, fit(l, w - 20 - iw, 17, 1), 17, 1, global.C_text);
txt(x + 12, y + 38, fit(s, w - 24, 14, 0), 14, 0, global.C_muted);
if (argument3 == 6) hs(x, y, x + w, y + 64, 950); else if (argument3 == 5) hs(x, y, x + w, y + 64, 940); else if (argument3 == 4) hs(x, y, x + w, y + 64, 850); else if (argument3 == 3) hs(x, y, x + w, y + 64, 747); else if (argument3 == 2) hs(x, y, x + w, y + 64, 745); else hs(x, y, x + w, y + 64, 740 + argument3);
