"""Write seed/bildvortaro.json: the A0 picture series, one per topic of vortbildoj.json.

usage: python -I make_seed.py SITE_DIR

Each series bildoj-<topic>: a short reading (the pictures show above it), then a
picture -> choose exercise per word (multiplechoice; content "bildo" names the
word whose picture shows, "definitions" give the picture's alt text; three options from the same topic, never from the
same look-alike group), then a vocab item per word (the picture is its prompt).
Vocab definitions come from the site's own seed vocabulary where the word or
its root is there (many languages), else an English gloss below.
"""
import json, os, random, sys, glob

site = sys.argv[1]
rows = json.load(open(os.path.join(site, "internal/bildoj/vortbildoj.json"), encoding="utf-8"))

ENDINGS = ["ojn", "ajn", "oj", "aj", "on", "an", "en", "as", "is", "os", "us", "o", "a", "e", "i", "u"]


def root(w):
    for e in ENDINGS:
        if w.endswith(e) and len(w) - len(e) >= 2:
            return w[: -len(e)]
    return None


def slugify(w):
    for a, b in zip("ĉĝĥĵŝŭ", ["cx", "gx", "hx", "jx", "sx", "ux"]):
        w = w.replace(a, b)
    return w.replace(" ", "-")


# definitions from the existing seed vocabulary: by word, then by root
by_word, by_root = {}, {}
for f in sorted(glob.glob(os.path.join(site, "seed/*.json"))):
    if f.endswith("bildvortaro.json"):
        continue
    for x in json.load(open(f, encoding="utf-8")):
        if (x.get("Type") or x.get("type")) != "vocab":
            continue
        c = x.get("Content") or x.get("content") or {}
        w = (c.get("word") or "").lower()
        defs = dict(c.get("definitions") or {})
        if not defs:
            en = c.get("en") or c.get("definition")
            if en:
                defs = {"en": en}
        if not w or not defs:
            continue
        tags = x.get("Tags") or x.get("tags") or []
        if any(t in tags for t in ("sufikso", "prefikso", "tabelvorto")):
            continue
        if len(defs) > len(by_word.get(w, {})):
            by_word[w] = defs
        r = root(w) or w
        if len(defs) > len(by_root.get(r, {})):
            by_root[r] = defs

EN = {
    "seĝo": "chair", "horloĝo": "clock", "lito": "bed", "ŝlosilo": "key", "biciklo": "bicycle",
    "ŝipo": "ship", "trajno": "train", "aviadilo": "aeroplane", "buso": "bus", "ombrelo": "umbrella",
    "ŝuo": "shoe", "ĉemizo": "shirt", "robo": "dress", "ĉapelo": "hat", "letero": "letter",
    "mono": "money", "krajono": "pencil", "pilko": "ball", "kulero": "spoon", "tranĉilo": "knife",
    "lernejo": "school", "hotelo": "hotel", "hospitalo": "hospital", "hundo": "dog", "kato": "cat",
    "birdo": "bird", "fiŝo": "fish", "ĉevalo": "horse", "bovino": "cow", "porko": "pig", "ŝafo": "sheep",
    "kokino": "hen", "muso": "mouse", "kuniklo": "rabbit", "urso": "bear", "leono": "lion",
    "elefanto": "elephant", "simio": "monkey", "serpento": "snake", "abelo": "bee", "papilio": "butterfly",
    "rano": "frog", "testudo": "tortoise", "lakto": "milk", "kafo": "coffee", "teo": "tea", "pomo": "apple",
    "banano": "banana", "oranĝo": "orange", "citrono": "lemon", "vinbero": "grape", "frago": "strawberry",
    "fromaĝo": "cheese", "ovo": "egg", "viando": "meat", "rizo": "rice", "kuko": "cake",
    "glaciaĵo": "ice cream", "supo": "soup", "terpomo": "potato", "tomato": "tomato", "karoto": "carrot",
    "vino": "wine", "viro": "man", "virino": "woman", "knabo": "boy", "knabino": "girl", "bebo": "baby",
    "oranĝkolora": "orange (colour)", "viola": "purple", "nul": "zero", "manĝi": "to eat",
    "trinki": "to drink", "dormi": "to sleep", "kuri": "to run", "iri": "to go, to walk", "naĝi": "to swim",
    "legi": "to read", "paroli": "to speak", "aŭskulti": "to listen", "vidi": "to see", "kanti": "to sing",
    "danci": "to dance", "kuiri": "to cook", "ridi": "to laugh", "plori": "to cry", "pensi": "to think",
    "aĉeti": "to buy", "okulo": "eye", "orelo": "ear", "nazo": "nose", "buŝo": "mouth", "dento": "tooth",
    "piedo": "foot", "kruro": "leg", "koro": "heart", "suno": "sun", "luno": "moon", "stelo": "star",
    "nubo": "cloud", "pluvo": "rain", "neĝo": "snow", "arbo": "tree", "floro": "flower",
    "monto": "mountain", "maro": "sea", "fajro": "fire", "ŝtono": "stone", "ĉielarko": "rainbow",
}

TOPICS = [
    ("objektoj", "Objektoj"), ("bestoj", "Bestoj"), ("mangxajxoj", "Manĝaĵoj"),
    ("familio", "Homoj kaj familio"), ("koloroj", "Koloroj"), ("nombroj", "Nombroj"),
    ("agoj", "Agoj"), ("korpo", "Korpo"), ("naturo", "Naturo"),
]

COLOUR = {
    "ruĝa": "La pomo estas ruĝa.", "oranĝkolora": "La oranĝo estas oranĝkolora.",
    "flava": "La citrono estas flava.", "verda": "La arbo estas verda.", "blua": "La maro estas blua.",
    "viola": "La vinberoj estas violaj.", "bruna": "La urso estas bruna.",
    "nigra": "La ĉapelo estas nigra.", "blanka": "La ovo estas blanka.", "griza": "La ŝtono estas griza.",
}


def text_for(topic, words):
    if topic == "koloroj":
        return "\n".join(COLOUR[w] for w in words)
    if topic == "nombroj":
        line = ", ".join(words).capitalize() + "."
        return line + "\n\nMi havas du okulojn kaj du orelojn. Mi havas unu buŝon kaj unu nazon."
    if topic == "agoj":
        return "\n".join("Mi " + w[:-1] + "as." for w in words)
    return "\n".join("Jen " + w + "." for w in words)


def defs_for(w):
    d = by_word.get(w) or by_root.get(root(w) or w)
    d = dict(d or {})
    if not d.get("en"):
        d["en"] = EN[w]
    return d


def item(slug, typ, content, tags, rating, series, order, label):
    return {
        "Slug": slug, "Type": typ, "Content": content, "Tags": tags,
        "Source": "esperanto-kurso.net", "AuthorID": "seed", "Status": "approved",
        "Rating": rating, "RD": 200, "Volatility": 0.06, "VoteScore": 0, "Version": 1,
        "ImageURL": "", "SeriesSlug": series, "SeriesOrder": order, "SeriesLabel": label,
        "SeriesParent": "",
    }


out = []
for ti, (topic, name) in enumerate(TOPICS):
    trows = [r for r in rows if r["topic"] == topic]
    words = [r["word"] for r in trows]
    series, label = "bildoj-" + topic, "Bildoj: " + name
    tag = {"mangxajxoj": "manĝaĵoj"}.get(topic, topic)
    base = 800 + ti * 10
    order = 1
    out.append(item(series + "-teksto", "reading", {"title": label, "text": text_for(topic, words)},
                    ["A0", "bildvortaro", tag, "legado"], base, series, order, label))
    for r in trows:
        w = r["word"]
        rnd = random.Random(w)
        pool = [x["word"] for x in trows if x["word"] != w and (not r.get("group") or x.get("group") != r.get("group"))]
        opts = rnd.sample(pool, 3) + [w]
        rnd.shuffle(opts)
        order += 1
        out.append(item("bildo-" + slugify(w) + "-elekto", "multiplechoice",
                        {"question": "Kio estas tio?", "options": opts, "correct_index": opts.index(w), "bildo": w,
                         "definitions": defs_for(w)},
                        ["A0", "bildvortaro", tag, "bildo"], base + 20, series, order, label))
    for r in trows:
        w = r["word"]
        order += 1
        # tagged with the reading's slug: the reading links its words and
        # its "practice the vocabulary" button finds them
        tags = ["A0", "bildvortaro", tag, "vortaro", series + "-teksto"]
        out.append(item("bildo-" + slugify(w), "vocab", {"word": w, "definitions": defs_for(w)},
                        tags, base + 60, series, order, label))

with open(os.path.join(site, "seed/bildvortaro.json"), "w", encoding="utf-8") as f:
    json.dump(out, f, ensure_ascii=False, indent=1)
    f.write("\n")
own = sum(1 for x in out if x["Type"] == "vocab" and list(x["Content"]["definitions"]) == ["en"])
print(len(out), "items;", own, "vocab items with only an English gloss")
