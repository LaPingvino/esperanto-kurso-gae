// parity_dump(): the parity check (game/parity.toml): every place of the
// map fetched from the site like any lesson (play -base at a site), each
// exercise answered through the game's own checks, into parity.txt for the
// Go test that judges the same answers with the site's checkAnswer. Lines,
// tab-separated:
//   R slug verdict answers…   the right answers (a choice: its option text)
//   W slug verdict answers…   a wrong one (a wrong tile / option)
//   T slug gap n              how many of a gap's tiles the game accepts
//   B place:i verdict / O place:i verdict   a sentence in order / in the tiles' order
//   C place:i verdict-right verdict-other   a word card
// It runs as a chain over other-62: parity_next asks for the next place.
global.par_i = 0;
file_open_write("parity.txt"); file_close(); // fresh
global.par_stage = 0; // first the map
global.par_req = http_get("/ludo-api/kurso");
