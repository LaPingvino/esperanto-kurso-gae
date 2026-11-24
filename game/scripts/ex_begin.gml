// ex_begin(): the exercise at global.qi, nothing answered yet.
var g, e, t;
global.fb = 0;      // 0 answering, 1 right, 2 wrong
global.typed = "";
keyboard_string = "";
global.focus = -1;
global.kind = global.q_kind[global.qi];
global.idx = global.q_idx[global.qi];
if (global.kind == 1) {
  e = global.les * 100 + global.idx;
  global.ngap = global.f_ngap[global.les, global.idx];
  for (g = 0; g < global.ngap; g += 1) { global.fa[g] = ""; global.fok[g] = 0; }
  global.cg = 0;
}
if (global.kind == 2) {
  e = global.les * 100 + global.idx;
  global.bn = 0;
  for (t = 0; t < global.b_nt[global.les, global.idx]; t += 1) global.bused[t] = 0;
}
global.pick = -1;
