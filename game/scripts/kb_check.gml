// kb_check(gave_up): Kontroli on Konstrui — right when the placed pieces
// spell the word (pieces that spell it otherwise would be the same word);
// "Mi ne scias" (gave_up) is a wrong one. Nothing goes to the site: only a
// course exercise's typed answer moves the ratings.
if (global.fb != 0) exit;
if (!argument0 && global.kb_n == 0) exit;
global.kb_tries += 1;
if (!argument0 && kb_built() == global.kb_word) { global.fb = 1; global.kb_ok += 1; }
else global.fb = 2;
global.focus = -1;
