// keys_screen(a0, st): the screen's keyboard of the 28 letters at the foot
// of the screen — a–h, ĥ–p, then Viŝi and r–z — for Vortludo and
// Vortpluvo: letter k presses a0 + k, Viŝi a0 + 30 (a0 < 0: none pressable);
// st 1 shows each letter's best state in Vortludo (vk_s). Returns its top.
var x0, x1, kw, kh, kg, kt, r, n, k, x, l, s;
x0 = global.m; x1 = global.W - global.m;
kh = 46; kg = 6;
kt = global.H - 16 - 3 * kh - 2 * kg;
kw = floor((x1 - x0 - 9 * kg) / 10); if (global.wide) kw = 52;
for (r = 0; r < 3; r += 1) {
  n = 10; if (r == 2) n = 8;
  x = (global.W - (n * kw + (n - 1) * kg)) / 2;
  if (r == 2) {
    x = (global.W - (n * kw + n * kg + floor(kw * 1.5))) / 2;
    vl_keybox(x, kt + r * (kh + kg), floor(kw * 1.5), kh, "Viŝi", -1, (argument0 + 30) * (argument0 >= 0) - (argument0 < 0));
    x += floor(kw * 1.5) + kg;
  }
  for (k = 0; k < n; k += 1) {
    l = string_char_at(global.t_alpha, r * 10 + k + 1);
    s = -1; if (argument1) s = global.vk_s[r * 10 + k];
    vl_keybox(x, kt + r * (kh + kg), kw, kh, string_upper(l), s, (argument0 + r * 10 + k) * (argument0 >= 0) - (argument0 < 0));
    x += kw + kg;
  }
}
return kt;
