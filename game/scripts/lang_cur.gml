// lang_cur(): the language the meanings are shown in — the player's choice,
// else the account's (/ludo-progreso), else English; one the course has.
var i;
for (i = 0; i < global.nlang; i += 1) if (global.lang[i] == global.mylang) return global.mylang;
for (i = 0; i < global.nlang; i += 1) if (global.lang[i] == global.acclang) return global.acclang;
return "en";
