// pv_drop_out(k): falling word k gone (caught or landed), the others closing up.
var j;
for (j = argument0; j < global.pv_n - 1; j += 1) {
  global.pv_w[j] = global.pv_w[j + 1]; global.pv_f[j] = global.pv_f[j + 1]; global.pv_ww[j] = global.pv_ww[j + 1];
  global.pv_x[j] = global.pv_x[j + 1]; global.pv_y[j] = global.pv_y[j + 1];
}
global.pv_n -= 1;
