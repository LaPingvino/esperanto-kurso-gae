"""Make the site's word pictures from OpenMoji and write the word -> picture data.

usage: python -I make_words.py WORDS_TXT OPENMOJI_DIR SITE_DIR

OPENMOJI_DIR holds openmoji.json and svg/ (openmoji-svg-color.zip, release 17.0.0).
Writes SITE_DIR/static/bildoj/vortoj/<hexcode>.svg (minified: ids dropped,
coordinates rounded to 2 decimals) and SITE_DIR/internal/bildoj/vortbildoj.json.
File names are the emoji's code points, so a picture's URL never spells the word.

A ref written %KEY (a relation of familytree.py: %father, %cousins) is a family-tree
picture instead, drawn by familytree.py into SITE_DIR/static/bildoj/parencoj/.
"""
import importlib.util, json, os, re, sys

sys.dont_write_bytecode = True  # no __pycache__ in the repo
_spec = importlib.util.spec_from_file_location("familytree", os.path.join(os.path.dirname(os.path.abspath(__file__)), "familytree.py"))
familytree = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(familytree)

words_txt, om, site = sys.argv[1], sys.argv[2], sys.argv[3]
meta = json.load(open(os.path.join(om, "openmoji.json")))
by_emoji, by_hex = {}, {}
for m in meta:
    by_hex[m["hexcode"]] = m
    by_emoji.setdefault(m["emoji"], m)
    by_emoji.setdefault(m["emoji"].replace("️", ""), m)

NUM = re.compile(r"-?(?:\d+\.?\d*|\.\d+)(?:e-?\d+)?")


def fmt(v):
    s = f"{round(v, 2):.2f}".rstrip("0").rstrip(".")
    if s in ("-0", ""):
        s = "0"
    if s.startswith("0."):
        s = s[1:]
    elif s.startswith("-0."):
        s = "-" + s[2:]
    return s


def round_nums(s):
    out, i = [], 0
    for mt in NUM.finditer(s):
        out.append(s[i:mt.start()])
        r = fmt(float(mt.group()))
        prev = "".join(out)[-1:]
        if r[0].isdigit() and (prev.isdigit() or prev == "."):
            r = " " + r  # ".0049" became "0": keep it apart from "8.9"
        nxt = s[mt.end():mt.end() + 1]
        if nxt == "." and "." not in r:
            r += " "
        out.append(r)
        i = mt.end()
    out.append(s[i:])
    return "".join(out)


def minify(svg):
    svg = re.sub(r"<!--.*?-->", "", svg, flags=re.S)
    svg = re.sub(r'\s+id="[^"]*"', "", svg)
    svg = re.sub(r">\s+<", "><", svg).strip()
    # numbers only inside coordinate attributes
    def attr(m):
        # arc flags may be written without separators ("a2,2,0,011.5,3"):
        # path data with arcs is left as it is.
        if m.group(1) == "d" and re.search(r"[aA]", m.group(2)):
            return m.group(0)
        return f'{m.group(1)}="{round_nums(m.group(2))}"'
    svg = re.sub(r'\b(d|points|x|y|x1|x2|y1|y2|cx|cy|r|rx|ry|width|height|transform)="([^"]*)"', attr, svg)
    svg = svg.replace("<g></g>", "")
    return svg + "\n"


outdir = os.path.join(site, "static/bildoj/vortoj")
os.makedirs(outdir, exist_ok=True)
rows, missing = [], []
for line in open(words_txt, encoding="utf-8"):
    line = line.strip()
    if not line or line.startswith("#"):
        continue
    word, topic, ref, flags = (line.split("|") + [""])[:4]
    flags = dict((f.split("=") + [True])[:2] for f in flags.split(",") if f)
    if ref.startswith("%"):
        key = ref[1:]
        if key not in familytree.RELATIONS:
            missing.append((word, ref))
            continue
        name = familytree.filename(key)
        treedir = os.path.join(site, "static/bildoj/parencoj")
        os.makedirs(treedir, exist_ok=True)
        with open(os.path.join(treedir, name), "w") as f:
            f.write(familytree.render(key))
        row = {"word": word, "topic": topic, "file": "parencoj/" + name,
               "source": "esperanto-kurso.net family tree: " + key,
               "author": "esperanto-kurso.net", "license": "CC0 1.0"}
        for k in ("exact", "group", "set"):
            if flags.get(k):
                row[k] = flags[k]
        rows.append(row)
        continue
    if ref == "=griza":
        # OpenMoji's white circle (26AA) with a grey fill: an adaptation, so CC BY-SA too.
        base = by_hex["26AA"]
        svg = open(os.path.join(om, "svg", "26AA.svg")).read().replace('fill="#fff"', 'fill="#9b9b9b"')
        name = "26aa-9b9b9b.svg"
        m = dict(base, annotation="grey circle (white circle 26AA, fill changed to #9b9b9b)")
    else:
        m = by_hex.get(ref[1:]) if ref.startswith("@") else by_emoji.get(ref)
        if not m:
            missing.append((word, ref))
            continue
        svg = open(os.path.join(om, "svg", m["hexcode"] + ".svg")).read()
        name = m["hexcode"].lower() + ".svg"
    with open(os.path.join(outdir, name), "w") as f:
        f.write(minify(svg))
    row = {
        "word": word,
        "topic": topic,
        "file": "vortoj/" + name,
        "source": "OpenMoji 17.0.0: " + m["hexcode"] + " " + m["annotation"],
        "author": m["openmoji_author"],
        "license": "CC BY-SA 4.0",
    }
    if flags.get("exact"):
        row["exact"] = True
    if flags.get("group"):
        row["group"] = flags["group"]
    if flags.get("set"):
        row["set"] = flags["set"]
    rows.append(row)

if missing:
    sys.exit(f"not in OpenMoji (or not a familytree relation): {missing}")
os.makedirs(os.path.join(site, "internal/bildoj"), exist_ok=True)
with open(os.path.join(site, "internal/bildoj/vortbildoj.json"), "w", encoding="utf-8") as f:
    f.write("[\n" + ",\n".join(json.dumps(r, ensure_ascii=False) for r in rows) + "\n]\n")
print(len(rows), "words")
