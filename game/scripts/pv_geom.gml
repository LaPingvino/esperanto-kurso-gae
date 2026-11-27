// pv_geom(): Vortpluvo's field, from the room alone (the step moves the
// words in it, the draw draws them): px0 / px1 its sides, pt its top, pg
// the ground (a word lands when its foot reaches it), pb the typing bar's
// top, under the ground and above the screen's keyboard (keys_screen).
var m;
m = 16; global.pt = 60 + 12 + 34;
if (room_width > room_height) { m = 40; global.pt = 64 + 12 + 34; }
global.px0 = m; global.px1 = room_width - m;
if (room_width > room_height) { global.px0 = (room_width - 720) / 2; global.px1 = global.px0 + 720; }
global.pb = room_height - 16 - 3 * 46 - 2 * 6 - 12 - 48;
global.pg = global.pb - 10;
