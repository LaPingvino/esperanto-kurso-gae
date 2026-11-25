// ob_add(): the exercise just answered, typed in every gap, queued for the
// site (sync_tick), and kept in the progress file until the site has it.
var g, n;
if (!global.site || global.x_slug[global.idx] == "") exit;
n = global.ob_n;
global.ob_slug[n] = global.x_slug[global.idx];
global.ob_na[n] = global.ngap;
for (g = 0; g < global.ngap; g += 1) global.ob_a[n, g] = global.fa[g];
global.ob_n += 1;
save_progress();
