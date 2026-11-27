// pv_type(s): the typed letters are s (pv_letters'); a falling word they
// spell whole — by either reading of the h-system (hfold: ankau is ankaŭ,
// flughaveno shown fluĝaveno still flughaveno) — is caught, the lowest first.
var f, k, b;
keyboard_string = argument0; global.typed = argument0; global.pv_buf = argument0;
if (argument0 == "") exit;
f = hfold(argument0); b = -1;
for (k = 0; k < global.pv_n; k += 1) if (global.pv_f[k] == f) { if (b < 0) b = k; else if (global.pv_y[k] > global.pv_y[b]) b = k; }
if (b < 0) exit;
global.pv_score += 1; global.pv_last = global.pv_w[b];
global.pv_fx = 15; global.pv_fxw = global.w_word[global.pv_w[b]]; global.pv_fxx = global.pv_x[b]; global.pv_fxy = global.pv_y[b]; global.pv_fxww = global.pv_ww[b];
pv_drop_out(b);
global.pv_fall = max(75, global.pv_fall * 0.96); // faster with every word
if (global.pv_n == 0) global.pv_t = 0; // nothing falling: the next at once
global.pv_buf = ""; keyboard_string = ""; global.typed = "";
sound_play(snd_right);
