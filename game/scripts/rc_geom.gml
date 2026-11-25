// rc_geom(): Vetkuro's road, from the room alone (the step moves on it, the
// draw draws it): global.rx0 / rx1 its sides, ry0 its top, cy the car's top,
// lw a lane's width (three lanes), qh the question card's height.
global.qh = 104; if (room_width > room_height) global.qh = 84;
global.rx0 = 16; global.rx1 = room_width - 16;
if (room_width > room_height) { global.rx0 = (room_width - 640) / 2; global.rx1 = global.rx0 + 640; }
global.ry0 = 60 + 12 + global.qh + 12; if (room_width > room_height) global.ry0 = 64 + 12 + global.qh + 12;
global.cy = room_height - 120;
global.lw = (global.rx1 - global.rx0) / 3;
