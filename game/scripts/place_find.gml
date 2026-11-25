// place_find(slug): the place's index in the map, -1 when it has none.
var i;
for (i = 0; i < global.np; i += 1) if (global.p_slug[i] == argument0) return i;
return -1;
