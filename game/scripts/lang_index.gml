// lang_index(): lang_cur's place in the course's languages.
var i;
for (i = 0; i < global.nlang; i += 1) if (global.lang[i] == lang_cur()) return i;
return 0;
