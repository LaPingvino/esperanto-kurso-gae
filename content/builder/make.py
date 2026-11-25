"""Run sources/fetch.sh first (it fetches the pages into a directory and
converts them to text), then: python make.py SRCDIR OUTDIR.

Builds content/builder/*.json (builder API format) from the fetched
Wikisource texts (argv[1] = src dir) and the hand-written exercises below.
argv[2] = output dir."""
import json, re, sys, hashlib, os

SRC, OUT = sys.argv[1], sys.argv[2]
SELF = "esperanto-kurso.net (konstruanto)"
PD = "publika havaĵo (public domain)"
LVL = {"A0": 900, "A1": 1100, "A2": 1300, "B1": 1500, "B2": 1700, "C1": 1900, "C2": 2100}


def read(n):
    return open(os.path.join(SRC, n), encoding="utf-8").read().replace("\u202f", " ").replace("\xa0", " ")


def between(t, start, end):
    i = t.index(start)
    j = t.index(end, i) + len(end)
    return t[i:j]


def tidy(t, join_lines=True):
    t = t.replace(" ", " ").replace("\xa0", " ")
    if join_lines:
        paras = [re.sub(r"\s*\n\s*", " ", p.strip()) for p in re.split(r"\n\s*\n", t)]
        t = "\n\n".join(p for p in paras if p)
    t = re.sub(r" +([:;!?])", r"\1", t)
    t = t.replace("« ", "„").replace(" »", "“").replace("«", "„").replace("»", "“")
    # ”…” pairs (the letter) → „…“
    out, open_q = [], True
    for ch in t:
        if ch == "”":
            out.append("„" if open_q else "“")
            open_q = not open_q
        else:
            out.append(ch)
    t = "".join(out)
    t = re.sub(r"[ \t]+", " ", t)
    t = re.sub(r" ,", ",", t)
    return t.strip()


def place(slug, opts):
    """Correct option first in the source; put it at a fixed, slug-derived index."""
    k = int(hashlib.sha256(slug.encode()).hexdigest(), 16) % len(opts)
    o = opts[1:]
    o.insert(k, opts[0])
    return o, k


items = {}
reading_order = {}


def add(file, it):
    items.setdefault(file, []).append(it)


def reading(file, slug, level, title, text, source, url, tags, note="", rating=None):
    c = {"title": title, "text": text, "source_url": url, "license": PD}
    if note:
        c["note"] = note
    series = "konstruanto-" + level.lower() + "-legado"
    reading_order[series] = reading_order.get(series, 0) + 1
    it = {"slug": slug, "type": "reading", "level": level, "content": c, "tags": tags,
          "source": source, "series_slug": series, "series_order": reading_order[series],
          "series_label": "Legaĵoj " + level}
    if rating:
        it["rating"] = rating
    add(file, it)


def exercises(file, parent, series, label, level, tags, exs, base=None):
    base = base or LVL[level]
    for n, e in enumerate(exs, 1):
        slug = f"{series}-{n:02d}"
        if e[0] == "mc":
            _, q, opts = e
            o, k = place(slug, list(opts))
            c = {"question": q, "options": o, "correct_index": k}
            typ = "multiplechoice"
        else:
            _, q, a, hint = e
            c = {"question": q, "answer": a}
            if hint:
                c["hint"] = hint
            typ = "fillin"
        it = {"slug": slug, "type": typ, "rating": base + (n % 5) * 15, "content": c,
              "tags": tags + (["plenigi"] if typ == "fillin" else ["elekti"]), "source": SELF,
              "series_slug": series, "series_order": n, "series_label": label}
        if parent:
            it["series_parent"] = parent
        add(file, it)


# ---------------------------------------------------------------- C2 texts
F = "10-c2-legajhoj.json"

t = read("h1.txt")
fund = "\n\n".join([
    between(t, "Por ke lingvo internacia povu", "la komenco de nia morto."),
    between(t, "Havante la karakteron de fundamento", "eĉ kune kun siaj eraroj."),
    between(t, "Mi diris, ke la fundamento de nia lingvo", "senkonfuza kaj sendanĝera."),
    between(t, "1) Riĉigadi la lingvon", "« Aldonon al la Fundamento »"),
    between(t, "2) Se ia aŭtoritata centra institucio", "por la estontaj generacioj."),
])
reading(F, "c2-fundamento-antauxparolo", "C2", "Antaŭparolo de la Fundamento (eltiro)", tidy(fund),
        "Fundamento de Esperanto, Antaŭparolo — L. L. Zamenhof (1905)",
        "https://eo.wikisource.org/wiki/Fundamento_de_Esperanto/Anta%C5%ADparolo",
        ["legado", "literatura", "zamenhof", "fundamento", "C2"],
        "Eltiro (5 alineoj el 9); tipografio modernigita (citiloj, spacoj antaŭ interpunkcio).")
exercises(F, "c2-fundamento-antauxparolo", "c2-fundamento-ekz", "Ekzercoj: Antaŭparolo de la Fundamento", "C2",
          ["fundamento", "kompreno", "C2"], [
    ("mc", "Laŭ Zamenhof, kiam aŭtoritata komitato rajtos fari ŝanĝojn en la fundamento?", [
        "Kiam la registaroj de la plej ĉefaj regnoj oficiale akceptos la lingvon kaj garantios ĝin per leĝo",
        "Kiam la aŭtoro de Esperanto mem trovos tion necesa",
        "Kiam la plimulto de la esperantistoj voĉdonos por ŝanĝo",
        "Neniam, eĉ ne post oficiala akcepto"]),
    ("mc", "Kial Zamenhof lasis nekorektitaj la erarojn en la nacia traduko de la Universala Vortaro?", [
        "Ĉar ĉiu plibonigo jam estus ŝanĝo, kaj la fundamento devas resti netuŝebla eĉ kun siaj eraroj",
        "Ĉar li ne rimarkis ilin antaŭ la presado",
        "Ĉar la eraroj troviĝas nur en la angla parto, kiun neniu uzas",
        "Ĉar la kongreso malpermesis al li korekti ilin"]),
    ("mc", "Kio laŭ la teksto okazos kun malnova formo, se oni rekomendos novan formon paralele kun ĝi?", [
        "Ĝi iom post iom fariĝos arĥaismo, sed ĉiam restos presata en lernolibroj kaj vortaroj",
        "Ĝi tuj estos forigita el la vortaroj",
        "Ĝi restos la sola oficiala formo, kaj la nova estos malpermesita",
        "Ĝi fariĝos deviga nur en la literaturo"]),
    ("mc", "„…la momento, en kiu ni ektuŝus tiun principon, estus la komenco de nia morto.“ Kion esprimas la prefikso ek- en „ektuŝus“?", [
        "Komencon aŭ unufojan, eĉ nur malgrandan faron de la ago",
        "Ripetadon de la ago",
        "Fortan, intensan agon",
        "Agon faritan de pluraj personoj"]),
    ("mc", "Kion signifas „netuŝebla“ en ĉi tiu teksto?", [
        "Tia, ke neniu rajtas ŝanĝi ĝin",
        "Tia, ke oni fizike ne povas atingi ĝin",
        "Tia, ke ankoraŭ neniu legis ĝin",
        "Tia, ke ĝi estas tro malfacila por komencantoj"]),
    ("mc", "Kiel, laŭ la teksto, oni uzu novajn vortojn, kiuj ne troviĝas en la Fundamento?", [
        "Ili estu rekomendataj, ne altrudataj; skribante al nekonatoj oni prefere uzu vortojn el la Fundamento",
        "Ili estas devigaj por ĉiuj, se la aŭtoro uzas ilin",
        "Oni tute ne rajtas uzi ilin ĝis oficiala decido",
        "Oni uzu ilin nur parole, neniam en la literaturo"]),
    ("fi", "La fundamento devas resti severe ___ eĉ kune kun siaj eraroj.", "netuŝebla", "vorto el la teksto: ne-tuŝ-ebl-a"),
    ("fi", "Por ke lingvo internacia povu bone kaj regule ___ … (progres-)", "progresadi", "i-formo kun la sufikso de daŭro -ad-, kiel en la teksto"),
    ("fi", "…nia lingvo havos la plenan eblon ne sole konstante ___, sed eĉ konstante pliboniĝadi kaj perfektiĝadi (riĉ-)", "riĉiĝadi", "i-formo kun -iĝ- kaj -ad-, kiel en la teksto"),
])

t = read("h2.txt")
let = "\n\n".join([
    between(t, "Mi naskiĝis en Bjelostoko", "mi nepre forigos tiun ĉi malbonon."),
    between(t, "Germanan kaj francan lingvojn", "kelkajn paĝojn."),
    between(t, "Unu fojon, kiam mi estis en la 6-a", "kaj ili komencis rapide malgrandiĝi antaŭ miaj okuloj."),
    between(t, "”La problemo estas solvita!”", "la tutan gramatikon kaj malgrandan vortaron."),
])
let = tidy(let).replace("ĉe la infano. ĉar al mi", "ĉe la infano. Ĉar al mi") + "\n\n[Notoj de la retejo: * ŝvejcarskaja (ruse) — pordistejo; ** konditorskaja (ruse) — sukeraĵejo.]"
reading(F, "c2-letero-deveno", "C2", "Letero pri la deveno de Esperanto (eltiro)", let,
        "Letero pri la deveno de Esperanto (al N. Borovko, ĉ. 1895) — L. L. Zamenhof; el Lingvaj Respondoj (1925)",
        "https://eo.wikisource.org/wiki/Lingvaj_Respondoj_(1925)/Letero_pri_la_deveno_de_Esperanto",
        ["legado", "literatura", "zamenhof", "historio", "C2"],
        "Eltiro; tradukita el la rusa (la tradukinto ne estas nomita en la fonto); tipografio modernigita; „ĉar“ post punkto majuskligita.")
exercises(F, "c2-letero-deveno", "c2-letero-ekz", "Ekzercoj: Letero pri la deveno", "C2",
          ["zamenhof", "kompreno", "C2"], [
    ("mc", "Kial, laŭ Zamenhof, lia naskiĝurbo donis la direkton al liaj celadoj?", [
        "Ĉar tie kvar popoloj kun malsamaj lingvoj vivis neamike unu apud la alia",
        "Ĉar tie ĉiuj flue parolis kvar lingvojn",
        "Ĉar lia patro instruis tie lingvojn",
        "Ĉar tie li unuafoje legis pri Volapük"]),
    ("mc", "Kion Zamenhof rimarkis, kiam li komencis lerni la anglan?", [
        "Ke la riĉeco de gramatikaj formoj estas blinda historia okazo, ne neceso de lingvo",
        "Ke la angla taŭgus kiel lingvo internacia",
        "Ke la vortaroj de naturaj lingvoj estas malgrandaj",
        "Ke la latina kaj la greka estas pli simplaj ol la angla"]),
    ("mc", "Kio solvis por li la problemon de la grandegaj vortaroj?", [
        "La sufiksoj, kiuj ebligas fari el unu vorto aliajn vortojn",
        "La prunto de ĉiuj vortoj el la latina",
        "Mallongaj elpensitaj vortoj kiel „pa“ por „interparoli“",
        "La forĵeto de ĉiuj sinonimoj"]),
    ("mc", "„…mi subite eksentis la teron sub la piedoj.“ Kion signifas tiu esprimo ĉi tie?", [
        "Li subite ekhavis firman bazon kaj certecon",
        "Li falis teren pro ekscito",
        "Li decidis forlasi la urbon",
        "Li ekdubis pri sia tuta laboro"]),
    ("mc", "Zamenhof skribas „mi forĵetadis“, „mi ripetadis“, „oni edukadis min“. Kion aldonas la sufikso -ad- al tiuj verboj?", [
        "Daŭron aŭ ripetiĝon de la ago",
        "Pasivan signifon",
        "La komencon de la ago",
        "Malgrandigon de la ago"]),
    ("mc", "Kial vortoj kiel „patrino“, „mallarĝa“, „tranĉilo“ povas „malaperi el la vortaro“?", [
        "Ĉar oni formas ilin el aliaj radikoj per afiksoj: patr-in-o, mal-larĝ-a, tranĉ-il-o",
        "Ĉar ili estas tro longaj kaj do forigindaj",
        "Ĉar ili ne havas internacian formon",
        "Ĉar ili estis anstataŭigitaj per vortoj el la rusa"]),
    ("fi", "En tia urbo pli ol ie la impresema naturo sentas la multepezan malfeliĉon de ___.", "diverslingveco", "vorto el la teksto: divers-lingv-ec-o"),
    ("fi", "La meĥaniko de la lingvo estis antaŭ mi kvazaŭ sur la ___.", "manplato", "esprimo el la teksto: „tute klare videbla“"),
])

t = read("h3.txt")
ps = [p for p in t.split("\n\n") if len(p) > 400]
p0 = ps[0][: ps[0].index("La homoj\nmiras")].strip()
es = tidy(p0 + "\n\n" + ps[1]).replace("iras. kiel", "iras, kiel").replace("samlempuloj", "samtempuloj")
reading(F, "c2-esenco-pioniroj", "C2", "Esenco kaj estonteco de la ideo de lingvo internacia (komenco)", es,
        "Esenco kaj estonteco de la ideo de lingvo internacia (1900) — L. L. Zamenhof; el Fundamenta Krestomatio (1903)",
        "https://eo.wikisource.org/wiki/Fundamenta_Krestomatio_(1903)/Esenco_kaj_estonteco_de_la_ideo_de_lingvo_internacia",
        ["legado", "literatura", "zamenhof", "eseo", "C2"],
        "La du unuaj alineoj (la unua mallongigita je la fino); du OCR-eraroj de la fonto korektitaj (samlempuloj → samtempuloj, „iras. kiel“ → „iras, kiel“); tipografio modernigita.")
exercises(F, "c2-esenco-pioniroj", "c2-esenco-ekz", "Ekzercoj: Esenco kaj estonteco", "C2",
          ["zamenhof", "kompreno", "stilo", "C2"], [
    ("mc", "Kiel, laŭ la teksto, la samtempuloj kutime renkontas novajn grandajn ideojn?", [
        "Kun obstina malkonfido kaj eĉ malamikeco",
        "Kun entuziasmo, sed sen kompreno",
        "Kun indiferento, ĉar ili ne aŭdas pri ili",
        "Kun respekto, sed sen mono"]),
    ("mc", "La vortoj „spritajn“ kaj „scias antaŭe“ staras en citiloj. Kion la aŭtoro esprimas per tio?", [
        "Ironion: li ne kredas, ke la artikoloj vere estas spritaj aŭ ke tiuj homoj vere scias",
        "Ke li citas laŭvorte konatan gazeton",
        "Ke temas pri fremdaj vortoj",
        "Ke li mem verkis tiujn artikolojn"]),
    ("mc", "Kion la posteuloj, laŭ la teksto, ne povas kompreni?", [
        "Kiamaniere oni povis miljarojn vivi sen la nova afero",
        "Kial la pioniroj tiom longe laboris",
        "Kiel la historiistoj trovis la rakontojn",
        "Kial la ideo estis tiel malfacila"]),
    ("mc", "Kion signifas „bubo“ en „la unua renkontita tre malmulte lerninta bubo“?", [
        "Malestime: nematura, malmulte sperta junulo",
        "Profesia ĵurnalisto",
        "Maljuna instruisto",
        "Lernanto de bazlernejo"]),
    ("mc", "„…la publiko, kiu ĉiam iras, kiel anaro da ŝafoj post la kriemuloj…“ Kion diras tiu komparo?", [
        "Ke la publiko sekvas la plej laŭtajn sen propra pensado",
        "Ke la publiko estas paca kaj senkulpa",
        "Ke la publiko loĝas en la kamparo",
        "Ke la kriemuloj gardas la publikon"]),
    ("fi", "Post longa vico da batalado kaj suferoj la „buboj-___“ atingis la celon.", "fantaziuloj", "vorto el la teksto: fantazi-ul-o-j"),
])

t = read("h6.txt")
rv = tidy(between(t, "Urbestro. — Kial! Certe", "li ne scias ruse eĉ unu vorton."))
rv = rv.replace("— Kial!", "— Kial!", 1)
rv = "(El la unua akto. La urbestro ĵus sciigis al la urbaj oficistoj, ke sekrete venas revizoro el la ĉefurbo. — Noto de la retejo.)\n\n" + rv.replace(". — ", ". — ")
reading(F, "c2-revizoro-urbestro", "C2", "La Revizoro: la urbestro avertas siajn oficistojn", rv,
        "La Revizoro, akto unua — N. V. Gogol, trad. L. L. Zamenhof (1907)",
        "https://eo.wikisource.org/wiki/La_Revizoro/Akto_unua",
        ["legado", "literatura", "teatraĵo", "zamenhof", "C2"],
        "Eltiro el la unua sceno; la enkonduka linio estas de la retejo; tipografio modernigita.")
exercises(F, "c2-revizoro-urbestro", "c2-revizoro-ekz", "Ekzercoj: La Revizoro", "C2",
          ["teatraĵo", "kompreno", "ironio", "C2"], [
    ("mc", "Kion supozas Ammos Fjodoroviĉ pri la kaŭzo de la revizio?", [
        "Ke Rusujo volas militi, kaj la ministraro sekrete serĉas perfidon",
        "Ke la urbestro mem petis revizoron",
        "Ke la malsanulejoj estas tro malpuraj",
        "Ke iu oficisto ŝtelis monon"]),
    ("mc", "„Eh kien vi ensaltis! Kaj ankoraŭ saĝa homo!“ Kian tonon havas la urbestro ĉi tie?", [
        "Mokan: li ridindigas la teorion de Ammos",
        "Admiran: li laŭdas la saĝecon de Ammos",
        "Timan: li kredas la teorion",
        "Oficialan kaj ĝentilan"]),
    ("mc", "Kial la urbestro trovas perfidon en ilia urbo absurda?", [
        "Ĉar la urbo estas tiel malproksime de la limo, ke eĉ post tri jaroj da rajdado oni atingus nenian alian regnon",
        "Ĉar ĉiuj loĝantoj estas fidelaj",
        "Ĉar la urbo havas grandan armeon",
        "Ĉar la revizoro jam alvenis"]),
    ("mc", "Artemij diras: „ju pli proksime al la naturo, des pli bone, — karajn medikamentojn ni ne uzas.“ Kion montras la satiro?", [
        "Ke li prezentas sian neglektadon de la malsanuloj kiel principon",
        "Ke li estas fervora kuracisto de natura medicino",
        "Ke la urbo ne havas monon pro milito",
        "Ke la malsanuloj mem preferas naturajn kuracilojn"]),
    ("mc", "Kiu frazo uzas „ju … des“ ĝuste?", [
        "Ju pli longe mi atendas, des pli mi koleriĝas.",
        "Des pli longe mi atendas, ju pli mi koleriĝas.",
        "Ju pli longe mi atendas, pli des mi koleriĝas.",
        "Ju longe mi atendas, des mi koleriĝas pli."]),
    ("fi", "Ĝis nun, dank’ al Dio, oni ___ aliajn urbojn; nun venis la vico al nia urbo. (atak-)", "atakadis", "is-formo kun -ad-, kiel en la teksto"),
])

t = read("h8.txt")
st = between(t, "LITVOLITVO! Patrujo mia!", "la kampa limrando.").replace("LITVOLITVO!", "Litvo!")
st = re.sub(r"\[\d+\]", "", st)
st = re.sub(r" +\d+\s*$", "", st, flags=re.M)
st = "\n".join(l.strip() for l in st.split("\n") if l.strip())
st = tidy(st, join_lines=False)
reading(F, "c2-sinjoro-tadeo-litvo", "C2", "Sinjoro Tadeo: la komenco („Litvo! Patrujo mia!“)", st,
        "Sinjoro Tadeo, Libro I — Adam Mickiewicz, trad. Antoni Grabowski (1918)",
        "https://eo.wikisource.org/wiki/Sinjoro_Tadeo/Libro_I",
        ["legado", "literatura", "poezio", "grabowski", "C2"],
        "La unuaj 22 versoj; piednotaj numeroj kaj versnumeroj forigitaj.")
exercises(F, "c2-sinjoro-tadeo-litvo", "c2-tadeo-ekz", "Ekzercoj: Sinjoro Tadeo", "C2",
          ["poezio", "kompreno", "stilo", "C2"], [
    ("mc", "Kiam, laŭ la poeto, oni ekkonas la grandan valoron de la patrujo?", [
        "Kiam oni ĝin perdis",
        "Kiam oni naskiĝas en ĝi",
        "Kiam oni vojaĝas tra ĝi",
        "Kiam oni priskribas ĝin"]),
    ("mc", "En „Sankta Virgin’!“ kaj „de tritik’ oritaj“ mankas la finaĵo -o. Kial tio estas permesata?", [
        "La Fundamento permesas anstataŭigi la finaĵon -o de substantivo per apostrofo",
        "Ĉar „Virgin“ kaj „tritik“ estas propraj nomoj",
        "Ĉar temas pri preseraroj",
        "Ĉar en poezio ĉiuj finaĵoj estas laŭvolaj"]),
    ("mc", "Kion signifas „hejmsopiras“?", [
        "Sopiras al sia hejmo, kiun li ne povas atingi",
        "Sopiras resti hejme anstataŭ vojaĝi",
        "Sopiras konstrui novan hejmon",
        "Sopiras forlasi sian hejmon"]),
    ("mc", "Kio estas „sekalo“ kaj „tritiko“ en la priskribo de la kampoj?", [
        "Specoj de greno",
        "Specoj de floroj",
        "Specoj de arboj",
        "Koloroj de la ĉielo"]),
    ("mc", "„Kaj ĉion zonas, kvazaŭ per verda rubando, / Kun maldensaj pirarboj la kampa limrando.“ Kio estas la subjekto de „zonas“?", [
        "la kampa limrando",
        "ĉion",
        "verda rubando",
        "maldensaj pirarboj"]),
    ("fi", "Al montetoj arbaraj, herbejoj ___, / Larĝe apud lazura Njemen tiriĝantaj (verd-)", "verdantaj", "aktiva participo, nuno, pluralo, kiel en la teksto"),
])

# ---------------------------------------------------------------- C1 texts
F = "20-c1.json"
t = read("h5.txt")
bd = "\n\n".join([
    between(t, "1. La Esperantismo estas penado", "por kiu la Esperantismo ne respondas."),
    between(t, "3. Ĉar la aŭtoro de la lingvo Esperanto", "la plej bonaj kaj plej talentaj verkistoj en tiu ĉi lingvo."),
    between(t, "4. Esperanto havas neniun personan leĝdonanton", "en kiu neniu havas la rajton fari ŝanĝon."),
    between(t, "5. Esperantisto estas nomata", "sed ne deviga."),
])
reading(F, "c1-bulonja-deklaracio", "C1", "Deklaracio pri la esenco de la Esperantismo (1905, eltiro)", tidy(bd),
        "Deklaracio pri la esenco de la Esperantismo (Bulonja Deklaracio, 1905) — L. L. Zamenhof kaj la Unua Kongreso",
        "https://eo.wikisource.org/wiki/Bulonja_deklaracio",
        ["legado", "zamenhof", "historio", "C1"], "Punktoj 1, 3, 4 (komenco) kaj 5.")
exercises(F, "c1-bulonja-deklaracio", "c1-bulonja-ekz", "Ekzercoj: Bulonja Deklaracio", "C1",
          ["historio", "kompreno", "C1"], [
    ("mc", "Laŭ la deklaracio, kio estas la Esperantismo?", [
        "Penado disvastigi la uzadon de neŭtrale homa lingvo, kiu ne celas elpuŝi la naciajn lingvojn",
        "Movado por anstataŭigi ĉiujn naciajn lingvojn",
        "Religia instruo de la aŭtoro",
        "Politika partio de la esperantistoj"]),
    ("mc", "Kion signifas, ke Esperanto estas „nenies propraĵo“?", [
        "Neniu havas personajn rajtojn pri la lingvo; ĉiu rajtas uzi ĝin kaj eldoni en ĝi",
        "Neniu volas lerni ĝin",
        "Ĝi apartenas nur al la kongreso",
        "Ĝi estas posedaĵo de la aŭtoro"]),
    ("mc", "Kio estas, laŭ punkto 4, la sola deviga fundamento de la lingvo?", [
        "La verketo „Fundamento de Esperanto“",
        "Ĉiuj verkoj de la kreinto de Esperanto",
        "La decidoj de ĉiu kongreso",
        "La verkoj de la plej bonaj verkistoj"]),
    ("mc", "Kiu estas esperantisto laŭ punkto 5?", [
        "Ĉiu, kiu scias kaj uzas Esperanton, egale por kiaj celoj",
        "Nur membro de esperanta societo",
        "Nur tiu, kiu subskribis la deklaracion",
        "Nur tiu, kiu kunhavas la ideojn de la aŭtoro"]),
    ("mc", "Kio estas la diferenco inter „rekomendinda“ kaj „rekomendata“?", [
        "„Rekomendinda“: indas rekomendi ĝin; „rekomendata“: iu efektive rekomendas ĝin",
        "Ili signifas tute la samon",
        "„Rekomendata“ estas pli forta: ĝi signifas „deviga“",
        "„Rekomendinda“ estas la pasinta formo de „rekomendata“"]),
    ("fi", "Apartenado al ia aktiva societo Esperantista por ĉiu Esperantisto estas ___, sed ne deviga.", "rekomendinda", "rekomend-ind-a"),
])

t = read("h7.txt")
po = between(t, "En la mondon venis nova sento", "Por eterna ben' efektiviĝos.")
lines = [l.strip() for l in po.split("\n") if l.strip()]
po = "\n\n".join("\n".join(lines[i:i + 4]) for i in range(0, len(lines), 4))
reading(F, "c1-la-espero", "C1", "La Espero (L. L. Zamenhof)", po,
        "La Espero — L. L. Zamenhof; el Fundamenta Krestomatio (1903)",
        "https://eo.wikisource.org/wiki/Fundamenta_Krestomatio_(1903)/La_espero",
        ["legado", "poezio", "zamenhof", "himno", "C1"])
exercises(F, "c1-la-espero", "c1-espero-ekz", "Ekzercoj: La Espero", "C1",
          ["poezio", "kompreno", "C1"], [
    ("mc", "Kion la „nova sento“ promesas al la mondo, laŭ la dua strofo?", [
        "Sanktan harmonion", "Venkon en milito", "Riĉecon", "Novan regnon"]),
    ("mc", "„Per laboro de la esperantoj“ — kiuj estas „la esperantoj“?", [
        "Homoj, kiuj esperas (esperanto = persono, kiu esperas)",
        "Libroj en Esperanto",
        "Diversaj dialektoj de Esperanto",
        "La esperoj de la homaro"]),
    ("mc", "„Al la mond' eterne militanta“ — kio estas „mond'“?", [
        "mondo, kun la finaĵo -o anstataŭita per apostrofo", "mondon", "monda", "mondoj"]),
    ("mc", "Per kio, laŭ la poemo, la baroj inter la popoloj estos disbatitaj?", [
        "La sankta amo", "La glavo", "La registaroj", "La vento"]),
    ("fi", "Sur neŭtrala lingva ___, / Komprenante unu la alian…", "fundamento", "vorto el la poemo"),
])

exercises(F, None, "c1-nuancoj", "Nuancoj de la lingvo (C1)", "C1", ["nuanco", "gramatiko", "C1"], [
    ("mc", "„Li ja venis!“ Kion aldonas „ja“?", [
        "Emfazon: tio estas certa, oni devus jam scii aŭ akcepti ĝin", "Neon", "Demandon", "Estontecon"]),
    ("mc", "Elektu la ĝustan vorton: „Mi estas pli alta ___ mia frato.“", ["ol", "kiel", "tiel", "de"]),
    ("mc", "„Petro vidis Paŭlon kaj lian hundon.“ Kies estas la hundo?", [
        "Ne de Petro: de Paŭlo aŭ de iu alia", "Certe de Petro", "De ambaŭ kune", "De neniu"]),
    ("mc", "Kiu frazo diras, ke la hundo apartenas al Petro mem?", [
        "Petro promenigas sian hundon.", "Petro promenigas lian hundon.", "Petro promenigas ŝian hundon.", "Petro promenigas ilian hundon."]),
    ("mc", "Kio estas la diferenco inter „glaso da vino“ kaj „glaso de vino“?", [
        "„da“ diras kvanton (glaso plena de vino); „de“ montras alian rilaton, ekz. glaso por vino (vinglaso)",
        "Ili signifas tute la samon",
        "„de“ diras kvanton; „da“ diras posedon",
        "„da“ uziĝas nur kun pluralo"]),
    ("mc", "„Kvankam pluvis, ni promenis.“ Kiu frazo diras la samon?", [
        "Malgraŭ la pluvo ni promenis.", "Ĉar pluvis, ni promenis.", "Se pluvos, ni promenos.", "Pro la pluvo ni ne promenis."]),
    ("mc", "Kion signifas „ekrigardi“, kompare kun „rigardi“?", [
        "Rigardi subite aŭ nur mallonge", "Rigardi longe", "Rigardi denove", "Ne rigardi"]),
    ("mc", "„Mi ne povas ne ridi.“ Kion tio signifas?", [
        "Mi nepre devas ridi; mi ne povas reteni la ridon", "Mi tute ne povas ridi", "Mi ne volas ridi", "Mi ridis hieraŭ"]),
    ("mc", "Kiu frazo estas en neŭtrala, ne poezia stilo?", [
        "La mondo estas granda.", "La mond' estas granda.", "Granda estas la mond'.", "Grandas la mondo."]),
    ("mc", "„Li estas tro laca por labori.“ Kion tio signifas?", [
        "Li estas tiel laca, ke li ne povas labori", "Li estas laca, ĉar li laboris", "Li laboras, kvankam li estas laca", "Li estas iom laca, sed laboras"]),
])

# ---------------------------------------------------------------- grammar (A2–B2)
F = "30-gramatiko.json"
exercises(F, None, "gramatiko-tabelvortoj-ekz", "Tabelvortoj (korelativoj)", "B1", ["gramatiko", "tabelvorto"], [
    ("fi", "___ vi loĝas? — Mi loĝas en Utrecht.", "Kie", "KI-vorto (demanda) pri loko"),
    ("fi", "Mi serĉis la ŝlosilon ĉie, sed ___ trovis ĝin.", "nenie", "NENI-vorto pri loko"),
    ("fi", "___ kostas tiu ĉi libro? — Dek eŭrojn.", "Kiom", "KI-vorto (demanda) pri kvanto"),
    ("fi", "Li faris ĝin ___, kiel mi diris al li.", "tiel", "TI-vorto (montra) pri maniero"),
    ("fi", "Ĉu vi aŭdis ___? — Ne, mi aŭdis nenion.", "ion", "I-vorto (nedifina) pri afero, kun akuzativo"),
    ("fi", "___ venos al la festo? — Mia plej bona amiko.", "Kiu", "KI-vorto (demanda) pri persono"),
    ("fi", "Mi ne scias, ___ li ne venis.", "kial", "KI-vorto pri kaŭzo"),
    ("fi", "La domo, en ___ mi naskiĝis, estas malnova.", "kiu", "KI-vorto (rilata), ununombro, sen direkto"),
    ("fi", "___ homo havas siajn proprajn revojn.", "Ĉiu", "ĈI-vorto pri persono"),
    ("fi", "Ni iru ___ — al la parko!", "tien", "TI-vorto (montra) pri loko, kun -n de direkto"),
    ("fi", "___ ajn vi iros, mi sekvos vin.", "Kien", "KI-vorto pri direkto, antaŭ „ajn“"),
    ("fi", "Li havas ___ da mono, ke li ne scias, kion fari.", "tiom", "TI-vorto (montra) pri kvanto"),
    ("fi", "___ fojon, kiam mi vidas lin, li ridetas.", "Ĉiun", "ĈI-vorto, akuzativo de tempo"),
    ("fi", "___ libro estas tiu ĉi? — Ĝi estas de Petro.", "Kies", "KI-vorto (demanda) pri posedo"),
])
exercises(F, None, "gramatiko-participoj-ekz", "Participoj", "B2", ["gramatiko", "participo"], [
    ("fi", "La letero, ___ hieraŭ, jam alvenis. (skrib-)", "skribita", "pasiva participo, pasinteco"),
    ("fi", "La knabino ___ en la ĝardeno estas mia fratino. (leg-)", "leganta", "aktiva participo, nuno"),
    ("fi", "La ___ gastoj jam telefonis, ke ili malfruos. (ven-)", "venontaj", "aktiva participo, estonteco, pluralo"),
    ("fi", "La domo estas ___ nun; la laboristoj ankoraŭ laboras. (konstru-)", "konstruata", "pasiva participo, nuno"),
    ("fi", "___, li ekdormis. (manĝ-)", "Manĝinte", "e-participo, aktiva, pasinteco"),
    ("fi", "La ___ jaro estis malfacila. (pas-)", "pasinta", "aktiva participo, pasinteco"),
    ("fi", "La ___ de Esperanto ofte rapide progresas. (lern-)", "lernantoj", "aktiva participo, nuno, kiel substantivo, pluralo"),
    ("fi", "La ___ estis kondukita al la policejo. (arest-)", "arestito", "pasiva participo, pasinteco, kiel substantivo"),
    ("mc", "Kiu frazo diras, ke la pordo jam estis en fermita stato (ne, ke iu ĝin fermadis)?", [
        "La pordo estis fermita.", "La pordo estis fermata.", "La pordo estis fermanta.", "La pordo estis fermonta."]),
    ("mc", "„Li estas mortonta.“ Kion tio signifas?", [
        "Li baldaŭ mortos", "Li jam mortis", "Li mortas ĝuste nun", "Oni mortigos lin"]),
])
exercises(F, None, "gramatiko-direkto-ekz", "Akuzativo de direkto", "B1", ["gramatiko", "akuzativo", "direkto"], [
    ("mc", "La kato estis sur la planko kaj saltis supren al la tablo. Kiu frazo diras tion?", [
        "La kato saltis sur la tablon.", "La kato saltis sur la tablo.", "La kato saltis sur tablo.", "La kato saltis sur la tabloj."]),
    ("mc", "„La infanoj kuris en la ĝardeno.“ Kion tio signifas?", [
        "La infanoj estis en la ĝardeno kaj kuris tie.", "La infanoj kuris de ekstere en la ĝardenon.",
        "La infanoj kuris for el la ĝardeno.", "La infanoj volis kuri al la ĝardeno."]),
    ("mc", "„La infanoj kuris en la ĝardenon.“ Kion tio signifas?", [
        "Ili kuris de ekstere kaj eniris la ĝardenon.", "Ili kuris tien kaj reen ene de la ĝardeno.",
        "Ili kuris for el la ĝardeno.", "Ili staris en la ĝardeno."]),
    ("fi", "Morgaŭ ni veturos ___. (Pariz-)", "Parizon", "direkto per akuzativo, sen prepozicio"),
    ("fi", "Kien vi iras? — Mi iras ___. (hejm-)", "hejmen", "adverbo kun -n de direkto"),
    ("fi", "Metu la librojn ___ la tablon.", "sur", "prepozicio: poste la libroj kuŝos supre de la tablo"),
    ("mc", "En kiu frazo „urbo“ ĝuste staras sen -n, kvankam temas pri direkto?", [
        "Mi iras al la urbo.", "Mi iras en la urbo.", "Mi iras la urbo.", "Mi iras sur la urbo."]),
    ("fi", "La birdo flugis ___ al la tegmento. (supr-)", "supren", "adverbo kun -n de direkto"),
])
exercises(F, None, "gramatiko-tempoj-modoj-ekz", "Verbaj tempoj kaj modoj", "A2", ["gramatiko", "verbo", "tempo", "modo"], [
    ("fi", "Hieraŭ mi ___ libron. (leg-)", "legis", "pasinteco (is-tenso)"),
    ("fi", "Morgaŭ ni ___ la avinon. (vizit-)", "vizitos", "estonteco (os-tenso)"),
    ("fi", "Nun ŝi ___ en la koruso. (kant-)", "kantas", "nuno (as-tenso)"),
    ("fi", "Se mi havus tempon, mi ___ vin. (help-)", "helpus", "kondicionalo (us-modo)"),
    ("fi", "___ tuj al mi! (ven-)", "Venu", "imperativo (u-modo)"),
    ("fi", "Mi volas, ke vi ___ la veron. (dir-)", "diru", "u-modo post „ke“ de volo"),
    ("fi", "Mi ŝatas ___ en la maro. (naĝ-)", "naĝi", "infinitivo (i-formo)"),
    ("fi", "Se mi ___ riĉa, mi vojaĝus. (est-)", "estus", "kondicionalo (us-modo)"),
    ("mc", "„Li diris, ke li venos.“ Kiam li venos, rilate al la momento, kiam li diris tion?", [
        "Post la momento de la diro", "Antaŭ la momento de la diro", "Samtempe kun la diro", "Neniam"]),
    ("mc", "„Li diris, ke li estas laca.“ Kiam li estis laca?", [
        "Samtempe kun sia diro", "Antaŭ sia diro", "Nur nun, kiam ni rakontas tion", "Post sia diro"]),
    ("mc", "Kiel nomiĝas la verbformo „farus“?", [
        "kondicionalo (us-modo)", "imperativo (u-modo)", "futuro (os-tenso)", "infinitivo (i-formo)"]),
], base=1300)
exercises(F, None, "gramatiko-afiksoj-ekz", "Vortfarado: afiksoj", "B1", ["gramatiko", "vortfarado", "afikso"], [
    ("fi", "Persono, kies profesio estas instrui, estas ___.", "instruisto", "instru- + sufikso de profesio"),
    ("fi", "Loko, kie oni lernas, estas ___.", "lernejo", "lern- + sufikso de loko"),
    ("fi", "La ina formo de „patro“ estas ___.", "patrino", "sufikso de ina sekso"),
    ("fi", "Malgranda domo estas ___.", "dometo", "sufikso de malgrandigo"),
    ("fi", "Tre granda domo estas ___.", "domego", "sufikso de grandigo"),
    ("fi", "La kontraŭo de „bona“ estas ___.", "malbona", "prefikso de kontraŭo"),
    ("fi", "Ilo por tranĉi estas ___.", "tranĉilo", "sufikso de ilo"),
    ("fi", "Iu, kiu loĝas en Parizo, estas ___.", "parizano", "sufikso -an-"),
    ("fi", "Tio, kion oni povas manĝi, estas ___.", "manĝebla", "sufikso de ebleco, a-vorto"),
    ("fi", "Libro, kiun indas legi, estas ___.", "leginda", "sufikso de indeco, a-vorto"),
    ("fi", "Subite komenci dormi: ___", "ekdormi", "prefikso ek-, i-formo"),
    ("fi", "Doni ion reen al ĝia posedanto: ___", "redoni", "prefikso de reveno, i-formo"),
    ("fi", "La patro kaj la patrino kune estas la ___.", "gepatroj", "prefikso de ambaŭ seksoj, pluralo"),
    ("fi", "Fari ion pura: ___", "purigi", "sufikso -ig-, i-formo"),
    ("fi", "Fariĝi ruĝa: ___", "ruĝiĝi", "sufikso -iĝ-, i-formo"),
    ("fi", "Unu ero de neĝo (unu neĝfloko) estas ___.", "neĝero", "neĝ- + sufikso -er-"),
    ("fi", "Granda aro da arboj estas ___.", "arbaro", "sufikso de aro"),
    ("fi", "La eco de tiu, kiu estas libera, estas ___.", "libereco", "sufikso de eco"),
], base=1400)
exercises(F, None, "gramatikaj-terminoj-ekz", "Gramatikaj terminoj", "B1", ["gramatiko", "terminoj"], [
    ("mc", "En la frazo „Petro legas libron“, kiu vorto estas la objekto?", ["libron", "Petro", "legas", "neniu"]),
    ("mc", "„Hieraŭ la hundo mordis la poŝtiston.“ Kiu estas la subjekto?", ["la hundo", "la poŝtiston", "hieraŭ", "mordis"]),
    ("mc", "Kiu vorto en „La rapida knabo kuras rapide“ estas adverbo?", ["rapide", "rapida", "knabo", "kuras"]),
    ("mc", "Kiu el tiuj verboj estas netransitiva?", ["dormi", "vidi", "manĝi", "doni"]),
    ("mc", "Kio estas „-ant-“?", ["sufikso de aktiva participo, nuno", "sufikso de pasiva participo, pasinteco", "prefikso de kontraŭo", "finaĵo de la akuzativo"]),
    ("mc", "Kiun finaĵon havas la akuzativo?", ["-n", "-j", "-a", "-e"]),
    ("mc", "En „la libro, kiun mi legis“, kio estas „kiun mi legis“?", ["rilata subfrazo", "ĉeffrazo", "infinitivo", "prepozicio"]),
    ("mc", "Kiel nomiĝas „pli granda“ en la gramatiko?", ["komparativo", "superlativo", "participo", "imperativo"]),
    ("mc", "Kiu el tiuj frazoj estas en la pasivo?", ["La pomo estas manĝata.", "La knabo manĝas pomon.", "La knabo manĝos.", "Manĝu la pomon!"]),
    ("mc", "Kio estas la radiko de „malsanulejo“?", ["san", "mal", "ul", "ej"]),
    ("mc", "Kiu el tiuj vortoj estas pronomo?", ["ŝi", "kaj", "sur", "ho"]),
    ("mc", "Kiu el tiuj vortoj estas konjunkcio?", ["sed", "sub", "tre", "ili"]),
], base=1450)

# vocab: grammar terms (word; en, nl, de, fr, es)
TERMS = """substantivo|noun|zelfstandig naamwoord|Substantiv|nom|sustantivo
adjektivo|adjective|bijvoeglijk naamwoord|Adjektiv|adjectif|adjetivo
adverbo|adverb|bijwoord|Adverb|adverbe|adverbio
verbo|verb|werkwoord|Verb|verbe|verbo
pronomo|pronoun|voornaamwoord|Pronomen|pronom|pronombre
prepozicio|preposition|voorzetsel|Präposition|préposition|preposición
konjunkcio|conjunction|voegwoord|Konjunktion|conjonction|conjunción
artikolo|article (grammar)|lidwoord|Artikel|article|artículo
interjekcio|interjection|tussenwerpsel|Interjektion|interjection|interjección
nombrovorto|numeral|telwoord|Numerale, Zahlwort|numéral|numeral
akuzativo|accusative|accusatief|Akkusativ|accusatif|acusativo
nominativo|nominative|nominatief|Nominativ|nominatif|nominativo
infinitivo|infinitive|infinitief|Infinitiv|infinitif|infinitivo
imperativo|imperative|gebiedende wijs|Imperativ|impératif|imperativo
kondicionalo|conditional (mood)|voorwaardelijke wijs|Konditional|conditionnel|condicional
participo|participle|deelwoord|Partizip|participe|participio
prezenco|present tense (as-tenso)|tegenwoordige tijd|Präsens|présent|presente
preterito|past tense (is-tenso)|verleden tijd|Präteritum, Vergangenheit|passé|pretérito
futuro|future tense (os-tenso)|toekomende tijd|Futur|futur|futuro
tenso|tense (grammar)|werkwoordstijd|Tempus|temps (grammatical)|tiempo verbal
modo|mood (grammar)|wijs (grammatica)|Modus|mode (grammatical)|modo (gramatical)
aspekto|aspect (grammar)|aspect|Aspekt|aspect|aspecto
subjekto|subject (grammar)|onderwerp|Subjekt|sujet|sujeto
objekto|object (grammar)|voorwerp|Objekt|complément d'objet|objeto
predikato|predicate|gezegde|Prädikat|prédicat|predicado
predikativo|predicative complement (PMEG: perverba priskribo)|naamwoordelijk deel van het gezegde|Prädikativ|attribut|predicativo
komplemento|complement (grammar)|bepaling|Ergänzung|complément|complemento
frazo|sentence|zin|Satz|phrase|oración
subfrazo|subordinate clause|bijzin|Nebensatz|proposition subordonnée|oración subordinada
transitiva|transitive|overgankelijk|transitiv|transitif|transitivo
netransitiva|intransitive|onovergankelijk|intransitiv|intransitif|intransitivo
aktivo|active voice|bedrijvende vorm|Aktiv|voix active|voz activa
pasivo|passive voice|lijdende vorm|Passiv|voix passive|voz pasiva
prefikso|prefix|voorvoegsel|Präfix|préfixe|prefijo
afikso|affix|affix|Affix|affixe|afijo
kunmetaĵo|compound word|samenstelling|Kompositum|mot composé|palabra compuesta
vokalo|vowel|klinker|Vokal|voyelle|vocal
konsonanto|consonant|medeklinker|Konsonant|consonne|consonante
akcento|stress (accent)|klemtoon|Betonung|accent tonique|acento
pluralo|plural|meervoud|Plural|pluriel|plural
komparativo|comparative|vergrotende trap|Komparativ|comparatif|comparativo
superlativo|superlative|overtreffende trap|Superlativ|superlatif|superlativo
tabelvorto|correlative (table word)|correlatief|Korrelativ|corrélatif|correlativo
helpverbo|auxiliary verb|hulpwerkwoord|Hilfsverb|auxiliaire|verbo auxiliar
demandvorto|question word|vraagwoord|Fragewort|mot interrogatif|palabra interrogativa
gramatiko|grammar|grammatica|Grammatik|grammaire|gramática
vortordo|word order|woordvolgorde|Wortstellung|ordre des mots|orden de las palabras
konjugacio|conjugation|vervoeging|Konjugation|conjugaison|conjugación
deklinacio|declension|verbuiging|Deklination|déclinaison|declinación
ortografio|spelling, orthography|spelling|Rechtschreibung|orthographe|ortografía
prononco|pronunciation|uitspraak|Aussprache|prononciation|pronunciación
idiomaĵo|idiom|uitdrukking, idioom|Redewendung|expression idiomatique|modismo
sinonimo|synonym|synoniem|Synonym|synonyme|sinónimo
antonimo|antonym|antoniem|Antonym|antonyme|antónimo"""
F = "40-terminoj.json"
for n, line in enumerate(TERMS.split("\n"), 1):
    w, en, nl, de, fr, es_ = line.split("|")
    add(F, {"slug": "termino-" + w.replace("ĵ", "jx").replace("ĉ", "cx").replace("ŝ", "sx").replace("ĝ", "gx").replace("ĥ", "hx").replace("ŭ", "ux"),
            "type": "vocab", "rating": 1350 + (n % 6) * 40,
            "content": {"word": w, "definition": en, "definitions": {"en": en, "nl": nl, "de": de, "fr": fr, "es": es_}},
            "tags": ["vortaro", "gramatika-termino"], "source": SELF,
            "series_slug": "vortaro-gramatikaj-terminoj", "series_order": n, "series_label": "Gramatikaj terminoj"})

os.makedirs(OUT, exist_ok=True)
for f, its in items.items():
    with open(os.path.join(OUT, f), "w", encoding="utf-8") as fh:
        json.dump(its, fh, ensure_ascii=False, indent=1)
        fh.write("\n")
    print(f, len(its))
