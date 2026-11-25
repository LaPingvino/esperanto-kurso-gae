// tile_w(label, size, num): a word tile's width for its label (num != 0:
// with its key number at the left; see tilebox).
font(argument1, 0, 0);
return string_width(argument0) + 32 + 28 * (argument2 != 0) + 24 * (argument2 > 0);
