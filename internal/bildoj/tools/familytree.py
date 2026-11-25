"""Family-tree word pictures: one small tree, "me" marked, the word's person(s) highlighted.

Emoji cannot tell father from uncle, so family words get this instead: the same
five-generation tree in every picture (grandparents, parents with an uncle and
aunt, me with a brother, sister, spouse and two cousins, children, a nephew and
niece, grandchildren). "Me" stands in a blue ring; the word's person(s) are
drawn a little larger in yellow with a black outline, everyone else faint grey. Gender is the
body's shape, not a colour: men square shoulders, women a dress (a triangle),
"me" and the spouse a rounded neutral body (the spouse takes a gender only for
husband / wife).

Drawn from primitives in a 72-unit box with OpenMoji's conventions (flat
fills from its palette, black round-joined outlines), so the pictures sit with
the OpenMoji ones. Our own work: CC0 1.0. Python 3 standard library only.

Language-neutral: relations have English keys; a site maps its own words to
them. File names are the highlighted slots' numbers, so a picture's URL never
spells its word in any language.

usage:
  python -I familytree.py --list                       the relation keys
  python -I familytree.py OUT_DIR KEY...               write OUT_DIR/<name>.svg per key,
                                                       print {key: file name} as JSON
used by make_words.py for words.txt refs written %KEY.
"""
import json, os, sys

# slot: (x, generation, gender) — gender m, f, or n (neutral: me, the spouse)
SLOTS = {
    "gf": (19, 0, "m"), "gm": (31, 0, "f"),
    "au": (6, 1, "f"), "un": (16, 1, "m"), "fa": (34, 1, "m"), "mo": (46, 1, "f"),
    "cm": (6, 2, "m"), "cf": (16, 2, "f"), "br": (29, 2, "m"), "si": (39, 2, "f"),
    "me": (51, 2, "n"), "sp": (63, 2, "n"),
    "ne": (29, 3, "m"), "ni": (39, 3, "f"), "so": (51, 3, "m"), "da": (63, 3, "f"),
    "gs": (51, 4, "m"), "gd": (63, 4, "f"),
}
ORDER = list(SLOTS)  # slot numbers for file names
# couples (drawn joined) and their children; single parents drop a line alone
COUPLES = [("gf", "gm", ["un", "fa"]), ("au", "un", ["cm", "cf"]),
           ("fa", "mo", ["br", "si", "me"]), ("me", "sp", ["so", "da"])]
SINGLE = [("br", "ne"), ("si", "ni"), ("so", "gs"), ("da", "gd")]

# relation -> highlighted slots; "sp:m" / "sp:f" give the spouse a gender
RELATIONS = {
    "grandfather": ["gf"], "grandmother": ["gm"], "grandparents": ["gf", "gm"],
    "father": ["fa"], "mother": ["mo"], "parents": ["fa", "mo"],
    "uncle": ["un"], "aunt": ["au"], "uncle and aunt": ["un", "au"],
    "brother": ["br"], "sister": ["si"], "siblings": ["br", "si"],
    "cousin (m)": ["cm"], "cousin (f)": ["cf"], "cousins": ["cm", "cf"],
    "husband": ["sp:m"], "wife": ["sp:f"],
    "son": ["so"], "daughter": ["da"], "children": ["so", "da"],
    "nephew": ["ne"], "niece": ["ni"], "nephew and niece": ["ne", "ni"],
    "grandson": ["gs"], "granddaughter": ["gd"], "grandchildren": ["gs", "gd"],
}

TOP, PITCH = 2.2, 13.8          # first row's top; row pitch
INK, FAINT = "#000", "#a8a8a8"
HI, PALE, ME = "#FCEA2B", "#e8e6e1", "#92D3F5"  # OpenMoji yellow, a pale grey, OpenMoji blue
LINE = "#8c8c8c"


def f(v):
    return f"{v:.2f}".rstrip("0").rstrip(".")


def person(x, gen, gender, fill, ink, sw, scale=1):
    y = TOP + gen * PITCH
    head = f'<circle cx="{f(x)}" cy="{f(y + 2.4)}" r="2.4"/>'
    b0, b1 = y + 5.6, y + 11.2
    if gender == "m":    # square shoulders
        body = (f'<path d="M{f(x-4)},{f(b1)}V{f(b0+1.6)}Q{f(x-4)},{f(b0)} {f(x-2.4)},{f(b0)}'
                f'H{f(x+2.4)}Q{f(x+4)},{f(b0)} {f(x+4)},{f(b0+1.6)}V{f(b1)}Z"/>')
    elif gender == "f":  # a dress
        body = (f'<path d="M{f(x-4.6)},{f(b1)}L{f(x-1.6)},{f(b0)}H{f(x+1.6)}'
                f'L{f(x+4.6)},{f(b1)}Z"/>')
    else:                # rounded, neither
        body = (f'<path d="M{f(x-3.4)},{f(b1)}V{f(b0+3)}A3.4,3 0 0 1 {f(x+3.4)},{f(b0+3)}'
                f'V{f(b1)}Z"/>')
    grow = ""
    if scale != 1:  # about the figure's centre
        cy = y + 5.6
        grow = f' transform="translate({f(x*(1-scale))},{f(cy*(1-scale))}) scale({scale})"'
    return (f'<g fill="{fill}" stroke="{ink}" stroke-width="{f(sw/scale)}" stroke-linejoin="round"{grow}>'
            f'{head}{body}</g>')


def lines():
    out = []
    mid = lambda g: TOP + g * PITCH + 7.5  # a couple's joining line: at the bodies
    gap = lambda g: TOP + (g + 1) * PITCH - 1.4  # sibling bar: in the gap below row g
    for a, b, kids in COUPLES:
        xa, g, _ = SLOTS[a]
        xb = SLOTS[b][0]
        lo, hi = sorted((xa, xb))
        y = mid(g)
        out.append(f"M{f(lo+4)},{f(y)}H{f(hi-4)}")
        cx = (xa + xb) / 2
        kx = [SLOTS[k][0] for k in kids]
        yb = gap(g)
        out.append(f"M{f(cx)},{f(y)}V{f(yb)}")
        out.append(f"M{f(min(kx + [cx]))},{f(yb)}H{f(max(kx + [cx]))}")
        for k in kx:
            out.append(f"M{f(k)},{f(yb)}V{f(TOP + (g + 1) * PITCH - .2)}")
    for p, c in SINGLE:
        x, g, _ = SLOTS[p]
        out.append(f"M{f(x)},{f(TOP + g * PITCH + 11.2)}V{f(TOP + (g + 1) * PITCH - .2)}")
    return (f'<path fill="none" stroke="{LINE}" stroke-width="1" stroke-linecap="round" '
            f'd="{"".join(out)}"/>')


def parse(key):
    hi, spouse = set(), "n"
    for s in RELATIONS[key]:
        slot, _, g = s.partition(":")
        hi.add(slot)
        if g:
            spouse = g
    return hi, spouse


def render(key):
    hi, spouse = parse(key)
    x, g, _ = SLOTS["me"]
    cy = TOP + g * PITCH + 5.6
    parts = [f'<svg viewBox="0 0 72 72" xmlns="http://www.w3.org/2000/svg">', lines(),
             f'<circle cx="{f(x)}" cy="{f(cy)}" r="6.6" fill="{ME}" stroke="{INK}" stroke-width="1.2"/>']
    hl = []
    for slot, (x, g, gender) in SLOTS.items():
        if slot == "sp":
            gender = spouse
        if slot in hi:
            hl.append(person(x, g, gender, HI, INK, 1.4, 1.25))  # a little larger, drawn on top
        elif slot == "me":
            parts.append(person(x, g, gender, "#fff", INK, 1.1))
        else:
            parts.append(person(x, g, gender, PALE, FAINT, .9))
    parts += hl + ["</svg>"]
    return "".join(parts) + "\n"


def filename(key):
    """The slots' numbers (and the spouse's gender as a/b): no word in any language."""
    hi, spouse = parse(key)
    name = "-".join(f"{ORDER.index(s):02d}" for s in sorted(hi, key=ORDER.index))
    if "sp" in hi:
        name += {"m": "a", "f": "b"}[spouse]
    return "p" + name + ".svg"


def _check():
    seen = {}
    for k in RELATIONS:
        n = filename(k)
        assert n not in seen, f"{k} and {seen[n]} draw the same picture"
        seen[n] = k


_check()

if __name__ == "__main__":
    if sys.argv[1:] == ["--list"]:
        print("\n".join(RELATIONS))
        sys.exit()
    if len(sys.argv) < 3:
        sys.exit(__doc__)
    out, keys = sys.argv[1], sys.argv[2:]
    os.makedirs(out, exist_ok=True)
    names = {}
    for k in keys:
        if k not in RELATIONS:
            sys.exit(f"unknown relation {k!r}; --list shows them")
        names[k] = filename(k)
        with open(os.path.join(out, names[k]), "w") as fh:
            fh.write(render(k))
    print(json.dumps(names, ensure_ascii=False, indent=1))
