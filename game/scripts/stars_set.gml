// stars_set(place slug, n): kept when more than before (never less); 1 when it changed.
var i;
for (i = 0; i < global.nst; i += 1) if (global.st_slug[i] == argument0) {
  if (argument1 > global.st_n[i]) { global.st_n[i] = argument1; return 1; }
  return 0;
}
if (argument1 <= 0) return 0;
global.st_slug[global.nst] = argument0; global.st_n[global.nst] = argument1; global.nst += 1;
return 1;
