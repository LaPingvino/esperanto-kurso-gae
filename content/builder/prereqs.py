"""python content/builder/prereqs.py content/builder

Writes 70-prereqs.json: what the English course (angla.org, ~/Projects/angla-kurso/
PREREQS.md) assumes and the live course did not yet teach at or below its level,
A1-B1 (docs/GAPS.md, "Prerequisites of the English course"). Self-written.
Vocab cards: an Esperanto definition first, then en/nl/de/fr/es."""
import hashlib, json, os, sys

OUT = sys.argv[1]
SELF = "esperanto-kurso.net (konstruanto)"
MID = {"A1": 1100, "A2": 1300, "B1": 1500}


def ascii_slug(w):
    for a, b in (("ĉ", "cx"), ("ĝ", "gx"), ("ĥ", "hx"), ("ĵ", "jx"), ("ŝ", "sx"), ("ŭ", "ux"), (" ", "-")):
        w = w.replace(a, b)
    return w


def place(slug, opts):
    """Correct option first in the source; put it at a fixed, slug-derived index."""
    k = int(hashlib.sha256(slug.encode()).hexdigest(), 16) % len(opts)
    o = opts[1:]
    o.insert(k, opts[0])
    return o, k


items = []


def vocab(series, label, level, tag, rows):
    for n, line in enumerate(rows.strip().split("\n"), 1):
        w, eo, en, nl, de, fr, es = [x.strip() for x in line.split("|")]
        items.append({"slug": "pr-" + ascii_slug(w), "type": "vocab",
                      "rating": MID[level] - 40 + (n % 5) * 20,
                      "content": {"word": w, "definition": en,
                                  "definitions": {"eo": eo, "en": en, "nl": nl, "de": de, "fr": fr, "es": es}},
                      "tags": ["vortaro", tag, "angla-antauxkono"], "source": SELF,
                      "series_slug": series, "series_order": n, "series_label": label})


def exercises(series, label, level, tags, exs):
    for n, e in enumerate(exs, 1):
        slug = f"{series}-{n:02d}"
        if e[0] == "mc":
            _, q, opts = e
            o, k = place(slug, list(opts))
            c, typ = {"question": q, "options": o, "correct_index": k}, "multiplechoice"
        else:
            _, q, a, hint = e
            c, typ = {"question": q, "answer": a, "hint": hint}, "fillin"
        items.append({"slug": slug, "type": typ, "rating": MID[level] - 40 + (n % 5) * 20, "content": c,
                      "tags": tags + ["angla-antauxkono", "plenigi" if typ == "fillin" else "elekti"],
                      "source": SELF, "series_slug": series, "series_order": n, "series_label": label})


# ---------------------------------------------------------------- A1
vocab("vortaro-cxiutagaj-a1", "Ĉiutagaj vortoj (A1)", "A1", "ĉiutaga", """
stacio|loko, kie trajnoj aŭ busoj haltas, por ke homoj eniru aŭ eliru|station, stop|station, halte|Bahnhof, Haltestelle|gare, arrêt|estación, parada
kinejo|ejo, kie oni spektas filmojn|cinema|bioscoop|Kino|cinéma|cine
laca|tia, ke oni volas ripozi aŭ dormi|tired|moe|müde|fatigué|cansado
banko|loko, kie homoj konservas sian monon kaj povas prunti monon|bank|bank|Bank|banque|banco
ferioj|tempo sen laboro aŭ lernejo, por ripozi aŭ vojaĝi|holidays, vacation|vakantie|Ferien, Urlaub|vacances|vacaciones
ekzameno|provo, en kiu oni montras, kion oni lernis|exam|examen|Prüfung|examen|examen
ŝteli|preni ion de alia homo sen permeso|to steal|stelen|stehlen|voler (dérober)|robar
parki|lasi aŭton aŭ biciklon stari ie por iom da tempo|to park|parkeren|parken|garer|aparcar
ripozi|ĉesi labori aŭ moviĝi por iom da tempo, por reakiri forton|to rest|rusten|sich ausruhen|se reposer|descansar
promesi|diri, ke oni certe faros ion|to promise|beloven|versprechen|promettre|prometer
gajni|ricevi monon aŭ premion; venki en ludo|to win; to earn|winnen; verdienen|gewinnen; verdienen|gagner|ganar
loterio|ludo, en kiu oni aĉetas bileton, kaj la hazardo elektas, kiu gajnas|lottery|loterij|Lotterie|loterie|lotería
frizisto|homo, kiu tondas kaj aranĝas la harojn de aliaj homoj|hairdresser|kapper|Friseur|coiffeur|peluquero
paciento|homo, kiun kuracisto kuracas|patient|patiënt|Patient|patient|paciente
leĝo|regulo, kiun la ŝtato faris, kaj kiun ĉiuj devas sekvi|law|wet|Gesetz|loi|ley
firmao|grupo de homoj, kiuj kune laboras por vendi aŭ fari ion kaj gajni monon|company, firm|bedrijf, firma|Firma|entreprise|empresa
afabla|bona kaj agrabla al aliaj homoj|kind, friendly|vriendelijk|freundlich|aimable|amable
""")
vocab("vortaro-lingvaj-terminoj-a1", "Lingvaj terminoj (A1)", "A1", "lingva-termino", """
komo|la signo „,“ ene de frazo|comma|komma|Komma|virgule|coma
citilo|unu el la signoj „ kaj “, inter kiuj oni skribas la vortojn de iu|quotation mark|aanhalingsteken|Anführungszeichen|guillemet|comilla
demandosigno|la signo „?“ ĉe la fino de demando|question mark|vraagteken|Fragezeichen|point d'interrogation|signo de interrogación
""")
exercises("gramatiko-prereqs-a1-ekz", "Horoj kaj lingvaj vortoj (A1)", "A1", ["gramatiko", "horo"], [
    ("fi", "Je kioma horo komenciĝas la filmo? — Je la ___. (8:00)", "oka", "-a: oka = 8-a"),
    ("fi", "La buso venas je la deka kaj ___. (10:15)", "kvarono", "unu el kvar partoj de horo"),
    ("fi", "Nun estas la tria kaj ___. (3:30)", "duono", "unu el du partoj de horo"),
    ("fi", "Post la longa vojaĝo mi estis tre ___ kaj tuj enlitiĝis.", "laca", "lac-, a-vorto"),
    ("fi", "La longa marŝado estis ___: post ĝi mi estis laca.", "laciga", "lac- + -ig- + -a: kio faras onin laca"),
    ("mc", "Kiu el tiuj literoj estas vokalo?", ["e", "k", "s", "m"]),
    ("mc", "Kiu el tiuj literoj estas konsonanto?", ["t", "a", "i", "o"]),
    ("mc", "Kiun signon oni skribas ĉe la fino de demando?", ["demandosignon (?)", "komon (,)", "citilojn („ “)", "punkton (.)"]),
    ("mc", "Kiel oni prononcas la literon „c“ en Esperanto?", ["kiel „ts“", "kiel „k“", "kiel „s“", "kiel „ĉ“"]),
])

# ---------------------------------------------------------------- A2
vocab("vortaro-cxiutagaj-a2", "Ĉiutagaj vortoj (A2)", "A2", "ĉiutaga", """
diferenco|tio, per kio du aferoj ne estas samaj|difference|verschil|Unterschied|différence|diferencia
testo|mallonga provo de scio aŭ kapablo|test|toets, test|Test|test, contrôle|prueba
konversacio|interparolo inter du aŭ pli da homoj|conversation|gesprek|Gespräch|conversation|conversación
kutimo|io, kion oni ofte kaj regule faras|habit, custom|gewoonte|Gewohnheit|habitude|costumbre
lango|la moviĝema parto en la buŝo, per kiu oni gustumas kaj parolas|tongue|tong|Zunge|langue (organe)|lengua (órgano)
lipo|unu el la du randoj de la buŝo|lip|lip|Lippe|lèvre|labio
palato|la supra parto de la interno de la buŝo|palate|gehemelte|Gaumen|palais|paladar
listo|vico de nomoj aŭ aferoj, skribitaj unu sub la alia|list|lijst|Liste|liste|lista
literumi|diri aŭ skribi la literojn de vorto unu post la alia|to spell|spellen|buchstabieren|épeler|deletrear
monbileto|papera mono|banknote|bankbiljet|Geldschein|billet de banque|billete
valuto|la mono de lando, ekzemple la eŭro aŭ la dolaro|currency|valuta|Währung|devise, monnaie|divisa, moneda
pundo|la mono de Britio|pound (sterling)|pond|Pfund|livre sterling|libra
dolaro|la mono de Usono kaj de kelkaj aliaj landoj|dollar|dollar|Dollar|dollar|dólar
mordi|premi aŭ tranĉi ion per la dentoj|to bite|bijten|beißen|mordre|morder
permesi|diri, ke iu rajtas fari ion|to allow, to permit|toestaan|erlauben|permettre|permitir
rifuzi|diri, ke oni ne volas fari aŭ akcepti ion|to refuse|weigeren|ablehnen, verweigern|refuser|negarse, rechazar
insisti|daŭre diri aŭ peti ion, eĉ se aliaj ne volas|to insist|aandringen|bestehen auf|insister|insistir
klopodi|multe peni por atingi ion|to try hard|zich inspannen|sich bemühen|s'efforcer|esforzarse
eviti|resti for de io; fari tiel, ke io ne okazu|to avoid|vermijden|vermeiden|éviter|evitar
stiri|direkti aŭton, ŝipon aŭ biciklon|to steer, to drive|sturen|lenken, steuern|conduire, piloter|conducir
tekstmesaĝo|mallonga skribita mesaĝo, sendita per telefono|text message|sms-bericht|SMS, Textnachricht|SMS, texto|mensaje de texto
ĵus|antaŭ tre mallonga tempo|just (a moment ago)|zojuist, net|gerade (eben)|tout juste (venir de)|recién (acabar de)
""")
vocab("vortaro-lingvaj-terminoj-a2", "Lingvaj terminoj (A2)", "A2", "lingva-termino", """
frakcio|nombro, kiu montras parton de tuto, ekzemple duono aŭ triono|fraction|breuk|Bruch|fraction|fracción
interpunkcio|la signoj en skribita teksto, ekzemple punkto, komo kaj demandosigno|punctuation|interpunctie|Zeichensetzung|ponctuation|puntuación
dupunkto|la signo „:“|colon|dubbele punt|Doppelpunkt|deux-points|dos puntos
punktokomo|la signo „;“|semicolon|puntkomma|Semikolon|point-virgule|punto y coma
krisigno|la signo „!“|exclamation mark|uitroepteken|Ausrufezeichen|point d'exclamation|signo de exclamación
streketo|la mallonga signo „-“ inter vortoj aŭ inter partoj de vorto|hyphen|koppelteken|Bindestrich|trait d'union|guion
majusklo|granda litero, ekzemple A, B, Ĉ|capital letter|hoofdletter|Großbuchstabe|majuscule|mayúscula
minusklo|malgranda litero, ekzemple a, b, ĉ|lower-case letter|kleine letter|Kleinbuchstabe|minuscule|minúscula
numeralo|vorto por nombro (nombrovorto); orda numeralo diras la lokon en vico: unua, dua, tria|numeral; ordinal numeral|telwoord; rangtelwoord|Zahlwort; Ordnungszahl|numéral; ordinal|numeral; ordinal
voĉa|(sono) kun vibrado en la gorĝo, ekzemple b, d, z|voiced (sound)|stemhebbend|stimmhaft|sonore, voisé|sonoro
senvoĉa|(sono) sen vibrado en la gorĝo, ekzemple p, t, s|voiceless (sound)|stemloos|stimmlos|sourd|sordo
volitivo|la verbformo kun -u, por peto, ordono aŭ deziro: Venu! Ni iru!|volitive (u-form)|gebiedende/wensende wijs (u-vorm)|Volitiv (u-Form)|volitif (forme en -u)|volitivo (forma en -u)
ĉeffrazo|la ĉefa parto de frazo kun subfrazo; ĝi kutime povas stari sola|main clause|hoofdzin|Hauptsatz|proposition principale|oración principal
""")
exercises("gramatiko-prereqs-a2-ekz", "Datoj, kutimoj kaj lingvaj terminoj (A2)", "A2", ["gramatiko"], [
    ("fi", "Mi naskiĝis la ___ de majo. (3)", "trian", "orda numeralo; -n por la tago, kiam io okazas"),
    ("fi", "Hodiaŭ estas la ___ de majo. (3)", "tria", "orda numeralo; post „estas“ sen -n"),
    ("fi", "Kiam mi estis infano, mi ĉiutage ___ piede al la lernejo.", "iradis", "ir- + -ad- (ripetata ago) + -is"),
    ("fi", "Mia avo ĉiuvespere ___ al ni fabelojn.", "rakontadis", "rakont- + -ad- (ripetata ago) + -is"),
    ("fi", "Mi loĝas en ĉi tiu urbo jam ___ kvin jaroj, kaj mi ankoraŭ loĝas ĉi tie.", "de", "prepozicio (2 literoj): de iam en la pasinteco ĝis nun"),
    ("fi", "En Brazilo ___ parolas la portugalan. (= La portugala estas parolata en Brazilo.)", "oni", "pronomo por „homoj ĝenerale“"),
    ("mc", "Kiu frazo diras la samon kiel „La angla estas parolata ĉi tie“, sed per „oni“?",
     ["Ĉi tie oni parolas la anglan.", "Ĉi tie oni parolas la angla.", "Ĉi tie la angla parolas.", "Ĉi tie parolas la angla."]),
    ("fi", "Post la laboro mi iras ___ kaj kuiras.", "hejmen", "hejm- kun -n de direkto"),
    ("fi", "La kato saltis ___ sur la tablon.", "supren", "supr- kun -n de direkto"),
    ("mc", "Kiu frazo montras direkton (movon al loko)?", ["Mi iras tien.", "Mi estas tie.", "Mi loĝas tie.", "Mi laboras tie."]),
    ("fi", "La rivero estas larĝa cent ___.", "metrojn", "metr-; akuzativo de mezuro"),
    ("fi", "La bebo dormis la tutan ___.", "nokton", "nokt-; akuzativo de tempo: kiom longe"),
    ("fi", "Januaro estas la ___ monato de la jaro.", "unua", "orda numeralo"),
    ("fi", "Se oni dividas tuton en tri egalajn partojn, ĉiu parto estas ___.", "triono", "tri + -on- + -o"),
    ("mc", "En la vorto „malbela“, kio estas „mal-“?", ["prefikso", "sufikso", "finaĵo", "radiko"]),
    ("mc", "En la vorto „lernejo“, kio estas „-ej-“?", ["sufikso", "prefikso", "finaĵo", "radiko"]),
    ("mc", "Kio estas afikso?", ["prefikso aŭ sufikso", "nur la finaĵo -o", "ĉiu radiko", "signo de interpunkcio"]),
    ("mc", "Sur kiu silabo estas la akcento en „familio“?", ["li", "fa", "mi", "o"]),
    ("mc", "Kiom da silaboj havas la vorto „lernejo“?", ["tri", "du", "kvar", "unu"]),
    ("mc", "Kiu el tiuj vortoj estas demandvorto?", ["kiam", "tiam", "ĉiam", "neniam"]),
    ("mc", "„Mi ne scias, kion vi volas.“ Kiu parto estas la ĉeffrazo?", ["Mi ne scias", "kion vi volas", "kion", "vi volas"]),
    ("mc", "Kiu frazo havas la plej kutiman vortordon (subjekto – verbo – objekto)?",
     ["Petro legas libron.", "Libron legas Petro.", "Legas Petro libron.", "Libron Petro legas."]),
    ("mc", "Kiu signo estas apostrofo?", ["’ (kiel en „l’ amiko“)", "- (kiel en „unu-du“)", "; (punktokomo)", "! (krisigno)"]),
    ("mc", "Kiu frazo estas ĝentila peto?", ["Ĉu vi bonvolus fermi la pordon?", "Fermu la pordon!", "La pordon!", "Vi fermos la pordon."]),
    ("mc", "Kiu verbo estas en la prezenco?", ["legas", "legis", "legos", "legu"]),
    ("mc", "Kiu el tiuj konsonantoj estas voĉa?", ["b", "p", "t", "k"]),
    ("mc", "Kiu el tiuj konsonantoj estas senvoĉa?", ["s", "z", "v", "d"]),
    ("fi", "Mi ___ trinkas kafon matene, sed hodiaŭ mi trinkis teon.", "kutime", "kutim-, e-vorto"),
    ("fi", "Ni ___ alvenis — antaŭ du minutoj.", "ĵus", "vorteto kun ĵ: antaŭ tre mallonga tempo"),
    ("fi", "Hieraŭ la patrino ne ___ al la infanoj manĝi dolĉaĵojn.", "permesis", "permes-, pasinteco"),
])

# ---------------------------------------------------------------- B1
vocab("vortaro-cxiutagaj-b1", "Ĉiutagaj vortoj (B1)", "B1", "ĉiutaga", """
dungito|homo, kiun firmao aŭ alia homo pagas, por ke li laboru|employee|werknemer|Angestellter, Arbeitnehmer|employé|empleado
dunganto|homo aŭ firmao, kiu pagas aliajn, por ke ili laboru|employer|werkgever|Arbeitgeber|employeur|empleador
ebleco|tio, ke io povas esti aŭ okazi|possibility|mogelijkheid|Möglichkeit|possibilité|posibilidad
malprobabla|tia, ke ĝi verŝajne ne okazos aŭ ne estas vera|unlikely|onwaarschijnlijk|unwahrscheinlich|improbable|improbable
enui|senti, ke la tempo pasas malrapide, ĉar nenio interesas onin|to be bored|zich vervelen|sich langweilen|s'ennuyer|aburrirse
enuiga|tia, ke ĝi enuigas; tute ne interesa|boring|saai|langweilig|ennuyeux|aburrido (que aburre)
escepto|io, kio ne sekvas la regulon|exception|uitzondering|Ausnahme|exception|excepción
fondi|starigi ion novan, ekzemple firmaon, lernejon aŭ urbon|to found|oprichten|gründen|fonder|fundar
institucio|granda organizaĵo, kiu daŭras longe, ekzemple universitato aŭ parlamento|institution|instelling|Institution|institution|institución
insularo|grupo de insuloj|archipelago|eilandengroep, archipel|Inselgruppe|archipel|archipiélago
dezerto|granda seka tereno kun multe da sablo aŭ ŝtonoj kaj preskaŭ sen plantoj|desert|woestijn|Wüste|désert|desierto
oceano|tre granda maro inter kontinentoj|ocean|oceaan|Ozean|océan|océano
intenci|voli kaj plani fari ion|to intend|van plan zijn|beabsichtigen|avoir l'intention de|tener la intención de
konfuzi|miksi aferojn en la kapo, tiel ke oni ne plu scias, kio estas kio; preni unu aferon por alia|to confuse, to mix up|verwarren|verwechseln, verwirren|confondre|confundir
konkludi|decidi, kio estas vera, post pripensado de la faktoj|to conclude|concluderen|folgern|conclure|concluir
kunteksto|la cetera teksto aŭ situacio ĉirkaŭ vorto aŭ evento, kiu helpas kompreni ĝin|context|context|Kontext, Zusammenhang|contexte|contexto
maltrafi|ne trafi; alveni tro malfrue, ekzemple por trajno|to miss|missen|verpassen, verfehlen|manquer, rater|perder (el tren), fallar
parkere|tiel, ke oni scias ion el la memoro, sen legi|by heart|uit het hoofd|auswendig|par cœur|de memoria
fantomo|spirito de mortinto, kiun iuj kredas vidi|ghost|spook|Gespenst|fantôme|fantasma
specimeno|unu ekzemplero, kiu montras, kia estas la tuta grupo|specimen, sample|exemplaar, monster|Exemplar, Probe|spécimen, échantillon|espécimen, muestra
arkitekto|homo, kiu planas kaj desegnas domojn kaj aliajn konstruaĵojn|architect|architect|Architekt|architecte|arquitecto
sekurzono|zono en aŭto aŭ aviadilo, kiu tenas la vojaĝanton sur la seĝo|seat belt|veiligheidsgordel|Sicherheitsgurt|ceinture de sécurité|cinturón de seguridad
graveda|(virino) kiu atendas bebon|pregnant|zwanger|schwanger|enceinte|embarazada
alteriĝi|(aviadilo) veni malsupren kaj halti sur la tero|to land|landen|landen|atterrir|aterrizar
eksiĝi|memvole forlasi sian oficon aŭ laboron|to resign|ontslag nemen|zurücktreten, kündigen|démissionner|dimitir
promocii|doni al iu pli altan postenon|to promote|bevorderen|befördern|promouvoir|ascender, promover
specifa|propra al unu afero aŭ grupo; ne ĝenerala|specific|specifiek|spezifisch|spécifique|específico
unika|tia, ke ekzistas nur unu tia|unique|uniek|einzigartig|unique|único
validi|havi forton aŭ esti vera por iu tempo aŭ kazo: la bileto validas unu tagon|to be valid, to apply|gelden|gelten|être valable|ser válido
vidpunkto|la maniero, kiel iu vidas kaj juĝas aferon|point of view|standpunt|Standpunkt|point de vue|punto de vista
fako|parto de scienco aŭ de laboro, pri kiu iu specialiĝas; studfako: tio, kion oni studas|field; subject (of study)|vak, vakgebied|Fach|discipline, matière|campo, asignatura
strukturo|la maniero, kiel la partoj de io estas kunmetitaj|structure|structuur|Struktur|structure|estructura
stilo|la maniero, kiel oni skribas, parolas aŭ faras ion|style|stijl|Stil|style|estilo
emfazi|diri ion pli forte, por montri, ke ĝi estas grava|to emphasise|benadrukken|betonen|souligner|enfatizar
fiksa|ne ŝanĝiĝanta; fiksa esprimo: esprimo, kies vortojn oni ne ŝanĝas|fixed; set phrase|vast; vaste uitdrukking|fest; feste Wendung|fixe; expression figée|fijo; frase hecha
citaĵo|vortoj, kiujn oni ripetas precize el alia teksto aŭ el alia parolanto|quotation|citaat|Zitat|citation|cita
""")
vocab("vortaro-lingvaj-terminoj-b1", "Lingvaj terminoj (B1)", "B1", "lingva-termino", """
akordo|(en gramatiko) kiam adjektivo ricevas la samajn finaĵojn -j kaj -n kiel sia substantivo: bonaj libroj, bonan libron|agreement (grammar)|congruentie|Kongruenz|accord|concordancia
diftongo|vokalo kaj duonvokalo (j aŭ ŭ) en unu silabo, ekzemple aj, oj, aŭ, eŭ|diphthong|tweeklank|Diphthong|diphtongue|diptongo
falsa amiko|vorto, kiu similas al vorto de alia lingvo, sed signifas ion alian|false friend|valse vriend|falscher Freund|faux ami|falso amigo
kalkulebla|(substantivo) por aferoj, kiujn oni povas nombri: unu libro, du libroj; ne kiel akvo aŭ lakto|countable (noun)|telbaar|zählbar|dénombrable|contable
kondiĉo|io, kio devas esti vera aŭ okazi, por ke alia afero okazu|condition|voorwaarde|Bedingung|condition|condición
koncedo|(en gramatiko) subfrazo pri io, kio ne malhelpas la ĉefan aferon: Kvankam pluvis, ni iris.|concession (grammar)|toegevende bijzin|Konzession, Einräumung|concession|concesión
subjunkcio|vorto, kiu komencas subfrazon, ekzemple ke, ĉar, kvankam, se|subordinating conjunction|onderschikkend voegwoord|unterordnende Konjunktion|conjonction de subordination|conjunción subordinante
nerekta parolo|raporto pri tio, kion iu diris, sen ties precizaj vortoj: Li diris, ke li venos.|reported (indirect) speech|indirecte rede|indirekte Rede|discours indirect|estilo indirecto
rekta parolo|la precizaj vortoj de parolanto, kutime inter citiloj: Li diris: „Mi venos.“|direct speech|directe rede|direkte Rede|discours direct|estilo directo
nerekta demando|demando, kiu estas subfrazo: Mi ne scias, kie li loĝas.|indirect question|indirecte vraag|indirekte Frage|interrogation indirecte|pregunta indirecta
perfekto|verbformo por ago jam finita; en Esperanto kutime simple -is, precize per estas + -inta: Mi estas manĝinta.|perfect (tense)|voltooide tijd|Perfekt|parfait, passé composé|perfecto
subjunktivo|en aliaj lingvoj: modo por deziroj kaj nerealaj aferoj; Esperanto uzas -u aŭ -us|subjunctive|aanvoegende wijs|Konjunktiv|subjonctif|subjuntivo
refleksiva|(pronomo) kiu montras reen al la subjekto: si, sin, sia|reflexive|wederkerend|reflexiv|réfléchi|reflexivo
rilativa|(pronomo aŭ subfrazo) kiu rilatas al antaŭa vorto: la libro, kiun mi legis|relative (pronoun, clause)|betrekkelijk|relativ|relatif|relativo
stativa|(verbo) kiu montras staton, ne agon: scii, ami, havi|stative (verb)|toestands- (werkwoord)|Zustands-|d'état|estativo
dinamika|(verbo) kiu montras agon aŭ okazon: kuri, skribi, fali|dynamic (action verb)|handelings- (werkwoord)|Handlungs-, dynamisch|d'action|dinámico
vortspeco|grupo de vortoj kun la sama gramatika rolo, ekzemple substantivoj, verboj, adjektivoj|part of speech, word class|woordsoort|Wortart|catégorie grammaticale|categoría gramatical
sibla|(sono) kun akra, siblanta bruo, kiel s, z, ŝ, ĵ, c, ĉ, ĝ|sibilant|sis- (klank)|Zischlaut|sifflant(e)|sibilante
ĝermana|rilata al la lingvoj kiel la angla, la germana kaj la nederlanda|Germanic|Germaans|germanisch|germanique|germánico
latina|rilata al la lingvo de la antikvaj romanoj|Latin|Latijns|lateinisch|latin|latino
varianto|iom alia formo de la sama afero, ekzemple de vorto aŭ de lingvo|variant|variant|Variante|variante|variante
""")
exercises("gramatiko-prereqs-b1-ekz", "Konstruoj kaj terminoj (B1)", "B1", ["gramatiko"], [
    ("fi", "La teo estis ___ varma, ke mi ne povis trinki ĝin.", "tiel", "TI-vorto de grado, kun „ke“"),
    ("fi", "Li havis ___ domon, ke ĉiuj gastoj trovis lokon.", "tian", "TI-vorto de kvalito, kun akuzativo"),
    ("fi", "Esperanto estas facile ___.", "lernebla", "lern- + -ebl- + -a"),
    ("fi", "Mi mem ne tondas mian hararon: mi ___ ĝin ĉe frizisto.", "tondigas", "tond- + -ig-: alia homo tondas por mi; prezenco"),
    ("fi", "Mia aŭto estis difektita, do mi ___ ĝin en garaĝo.", "riparigis", "ripar- + -ig-: alia homo riparis por mi; pasinteco"),
    ("fi", "Kiam vi venos, mi jam estos ___ la laboron.", "fininta", "fin-; aktiva participo de finita ago"),
    ("fi", "Kiam ni alvenis, la vendejo jam estis ___, kaj ni ne povis eniri.", "fermita", "ferm-; pasiva participo de finita ago"),
    ("fi", "Li ne estas malsata: li jam estas ___.", "manĝinta", "manĝ-; aktiva participo de finita ago"),
    ("fi", "Se nur mi ___ pli da tempo!", "havus", "hav-; deziro pri nereala afero"),
    ("fi", "La ĵus ___ letero estas de mia fratino.", "alveninta", "alven-; aktiva participo de finita ago, a-vorto"),
    ("fi", "Ili amas unu la ___.", "alian", "reciproka; ali- + -a + -n"),
    ("fi", "La firmao havas cent ___.", "dungitojn", "dung- + -it- + -o; pluralo, akuzativo"),
    ("fi", "Ni ___ la buson kaj devis atendi duonan horon.", "maltrafis", "mal- + traf-; pasinteco"),
    ("mc", "En „Li estis vokita“, kiu vorto estas la helpverbo kaj kiu la ĉefverbo?",
     ["„estis“ estas la helpverbo, „vokita“ la ĉefverbo", "„vokita“ estas la helpverbo, „estis“ la ĉefverbo",
      "ambaŭ estas helpverboj", "ambaŭ estas ĉefverboj"]),
    ("mc", "Kiu vorto en „Eĉ Petro venis“ estas vorteto (partikulo)?", ["eĉ", "Petro", "venis", "neniu el ili"]),
    ("mc", "Kiu frazo enhavas nerektan demandon?", ["Mi ne scias, ĉu li venos.", "Ĉu li venos?", "Li venos.", "Venu!"]),
    ("mc", "Kiu frazo estas en rekta parolo?",
     ["Ŝi diris: „Mi estas laca.“", "Ŝi diris, ke ŝi estas laca.", "Ŝi estis laca.", "Ŝi ne diris, ĉu ŝi estas laca."]),
    ("mc", "Kiu substantivo estas kalkulebla?", ["libro", "akvo", "lakto", "sablo"]),
    ("mc", "Kiu vorto estas subjunkcio?", ["ĉar", "kaj", "sed", "aŭ"]),
    ("mc", "En „Li lavas sin“, kia pronomo estas „sin“?", ["refleksiva", "rilativa", "demanda", "montra"]),
    ("mc", "Kiu verbo montras staton, ne agon?", ["scii", "kuri", "skribi", "salti"]),
    ("mc", "Kio estas falsa amiko?", ["vorto, kiu similas al vorto de alia lingvo, sed signifas ion alian",
                                      "amiko, kiu mensogas", "vorto sen signifo", "vorto kun du radikoj"]),
    ("mc", "Kiu frazo havas koncedon?", ["Kvankam pluvis, ni iris promeni.", "Ĉar pluvis, ni restis hejme.",
                                         "Se pluvos, ni restos hejme.", "Pluvis, do ni restis hejme."]),
])

os.makedirs(OUT, exist_ok=True)
with open(os.path.join(OUT, "70-prereqs.json"), "w", encoding="utf-8") as fh:
    json.dump(items, fh, ensure_ascii=False, indent=1)
    fh.write("\n")
print("70-prereqs.json", len(items))
