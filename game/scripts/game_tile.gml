// game_tile(x, y, w, which): a bonus game's tile on the levels screen:
// 0 Vortludo (three cells in its states), 1 Vetkuro (the car), 2 Konstrui
// (three pieces joined into a word).
var x, y, w, l, s;
x = argument0; y = argument1; w = argument2;
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
} else {
  l = "Vetkuro"; s = "stiru al la vorto";
  rfill(x + w - 28, y + 8, x + w - 12, y + 32, global.C_main, global.C_main);
  rfill(x + w - 26, y + 12, x + w - 14, y + 17, global.C_tint, global.C_tint);
}
txt(x + 12, y + 10, fit(l, w - 84, 17, 1), 17, 1, global.C_text);
txt(x + 12, y + 38, fit(s, w - 24, 14, 0), 14, 0, global.C_muted);
if (argument3 == 2) hs(x, y, x + w, y + 64, 745); else hs(x, y, x + w, y + 64, 740 + argument3);
