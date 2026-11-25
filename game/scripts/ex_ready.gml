// ex_ready(): there is an answer to check (Kontroli is pressable).
if (global.fb != 0) return 0;
if (global.kind == 0 || global.kind == 3) return global.pick >= 0;
if (global.kind == 2) return global.bn >= global.x_nt[global.idx, 0];
return global.cg >= global.ngap;
