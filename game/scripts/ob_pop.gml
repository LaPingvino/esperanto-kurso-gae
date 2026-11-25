// ob_pop(): the first queued answer is with the site: off the queue.
var i, g;
for (i = 0; i < global.ob_n - 1; i += 1) {
  global.ob_slug[i] = global.ob_slug[i + 1];
  global.ob_na[i] = global.ob_na[i + 1];
  for (g = 0; g < global.ob_na[i]; g += 1) global.ob_a[i, g] = global.ob_a[i + 1, g];
}
global.ob_n -= 1;
