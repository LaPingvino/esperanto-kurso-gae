// map_paper(y0): the map's paper below y0, folded in panels as a paper map is:
// each panel's shade is the mean light of the same panel of the Cartography
// Pack's parchmentFolded texture (ASSETS.md), at a third of its strength, at
// most 2 darker than the plain paper (every colour on the map keeps its
// contrast). Flat fills, not the picture: blending a 1280x656 picture took
// ~12 ms a frame natively (Go's image/draw); a fill costs next to nothing.
// The shades were computed once from the texture (ASSETS.md).
var y0;
y0 = argument0;
if (global.wide) {
  fill(0, y0 + 0, 319, y0 + 319, rgb(250, 245, 235));
  fill(320, y0 + 0, 639, y0 + 319, rgb(253, 248, 238));
  fill(640, y0 + 0, 959, y0 + 319, rgb(251, 246, 236));
  fill(960, y0 + 0, 1279, y0 + 319, rgb(252, 247, 237));
  fill(0, y0 + 320, 319, y0 + 639, rgb(245, 240, 230));
  fill(320, y0 + 320, 639, y0 + 639, rgb(250, 245, 235));
  fill(640, y0 + 320, 959, y0 + 639, rgb(244, 239, 229));
  fill(960, y0 + 320, 1279, y0 + 639, rgb(247, 242, 232));
  fill(0, y0 + 640, 319, y0 + 655, rgb(252, 247, 237));
  fill(320, y0 + 640, 639, y0 + 655, rgb(254, 249, 239));
  fill(640, y0 + 640, 959, y0 + 655, rgb(251, 246, 236));
  fill(960, y0 + 640, 1279, y0 + 655, rgb(254, 249, 239));
} else {
  fill(0, y0 + 0, 188, y0 + 188, rgb(250, 245, 235));
  fill(189, y0 + 0, 377, y0 + 188, rgb(253, 248, 238));
  fill(378, y0 + 0, 419, y0 + 188, rgb(251, 246, 236));
  fill(0, y0 + 189, 188, y0 + 377, rgb(245, 240, 230));
  fill(189, y0 + 189, 377, y0 + 377, rgb(250, 245, 235));
  fill(378, y0 + 189, 419, y0 + 377, rgb(244, 239, 229));
  fill(0, y0 + 378, 188, y0 + 566, rgb(252, 247, 237));
  fill(189, y0 + 378, 377, y0 + 566, rgb(254, 249, 239));
  fill(378, y0 + 378, 419, y0 + 566, rgb(251, 246, 236));
  fill(0, y0 + 567, 188, y0 + 755, rgb(244, 239, 229));
  fill(189, y0 + 567, 377, y0 + 755, rgb(247, 242, 232));
  fill(378, y0 + 567, 419, y0 + 755, rgb(245, 240, 230));
}
