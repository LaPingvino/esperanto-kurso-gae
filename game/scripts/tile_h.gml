// tile_h(label, width, size, num): the height of a tile that wide, its label
// wrapped inside it (at least 56, kurso-design's --tile-min); the ledge
// under it not counted.
var inner;
inner = argument1 - 32 - 28 * (argument3 != 0) - 24 * (argument3 > 0);
return max(56, th(argument0, inner, argument2, 0) + 24);
