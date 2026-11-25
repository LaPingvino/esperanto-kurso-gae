// stars_get(place slug): its stars (0: not done).
var i;
for (i = 0; i < global.nst; i += 1) if (global.st_slug[i] == argument0) return global.st_n[i];
return 0;
