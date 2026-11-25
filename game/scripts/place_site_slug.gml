// place_site_slug(): the site page of the open place: a reading's or a
// video's own; for a place of exercises, its first exercise's.
var i;
if (global.les_kind != "E") return global.les_slug;
for (i = 0; i < global.nex; i += 1) if (global.x_slug[i] != "") return global.x_slug[i];
return "";
