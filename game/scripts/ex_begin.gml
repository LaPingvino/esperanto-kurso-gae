// ex_begin(): the exercise at global.qi, nothing answered yet. Kinds:
// 0 a word card, 1 a fill-in, 2 a sentence to build, 3 multiple choice,
// 4 a vocabulary word, 5 a typed answer (gamedata/course.go).
var g, t;
global.fb = 0; global.typed = ""; keyboard_string = ""; global.focus = -1; global.pick = -1;
global.idx = global.q_idx[global.qi];
global.kind = global.x_kind[global.idx];
global.ngap = global.x_ng[global.idx];
for (g = 0; g < max(1, global.ngap); g += 1) { global.fa[g] = ""; global.fok[g] = 0; global.ftyped[g] = 0; }
global.cg = 0;
global.bn = 0;
for (t = 0; t < 10; t += 1) global.bused[t] = 0;
