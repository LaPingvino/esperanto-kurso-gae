// course_parse(text): the map (GET /ludo-api/kurso) into the course arrays;
// 1 when it is a whole one (KURSO … FINO), else 0 and the arrays untouched.
var l, ok, ns, np, nl;
global.ptext = argument0; global.peof = 0;
split_line(next_line());
if (global.fld[0] != "KURSO") return 0;
// check it is whole before changing anything
if (string_pos(chr(10) + "FINO", argument0) == 0) return 0;
global.course_ver = global.fld[1];
ns = 0; np = 0; nl = 0;
while (!global.peof) {
  l = next_line();
  if (l == "") continue;
  split_line(l);
  if (global.fld[0] == "N") { global.lang[nl] = global.fld[1]; nl += 1; }
  if (global.fld[0] == "S") {
    global.s_slug[ns] = global.fld[1]; global.s_level[ns] = global.fld[2]; global.s_label[ns] = global.fld[3];
    global.s_first[ns] = np; global.s_np[ns] = 0; ns += 1;
  }
  if (global.fld[0] == "P" && ns > 0) {
    global.p_slug[np] = global.fld[1]; global.p_kind[np] = global.fld[2]; global.p_nex[np] = real(global.fld[3]);
    global.p_title[np] = global.fld[4]; global.p_series[np] = ns - 1; global.s_np[ns - 1] += 1; np += 1;
  }
}
global.ns = ns; global.np = np;
if (nl == 0) { global.lang[0] = "en"; nl = 1; }
global.nlang = nl;
return 1;
