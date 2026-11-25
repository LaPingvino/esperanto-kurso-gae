"""Make the site's lesson pictures and exercise-kind icons from Kenney's CC0 packs.

usage: python -I make_site_icons.py KENNEY_ROOT SITE_DIR

KENNEY_ROOT holds the unzipped packs (cartographypack, kenney_board-game-icons,
Kenney_gameIcons), downloaded from their OpenGameArt pages.

- static/bildoj/lokoj/lNN.png: lesson NN's place on the game's map, the same
  icons in the same order as the game's spr_place, in the map's ink (104, 86, 62),
  96 px (drawn at 48 CSS px, sharp on 2x screens).
- static/bildoj/specoj/<kind>.png: one icon per exercise kind, white shape on
  transparent, 40 px; the site uses it as a CSS mask filled with the text colour,
  so it follows the theme.
Each icon is used as a shape only (its alpha), fitted with Lanczos, saved with
no metadata chunks.
"""
import os, sys
from PIL import Image

src, site = sys.argv[1], sys.argv[2]
CA = os.path.join(src, "cartographypack/PNG/Retina")
BG = os.path.join(src, "kenney_board-game-icons/PNG/Double (128px)")
GI = os.path.join(src, "Kenney_gameIcons/PNG/White/2x")


def shape(path, size, pad=0):
    a = Image.open(path).convert("RGBA").getchannel("A")
    a = a.crop(a.getbbox())
    w, h = a.size
    inner = size - 2 * pad
    s = inner / max(w, h)
    nw, nh = max(1, round(w * s)), max(1, round(h * s))
    a = a.resize((nw, nh), Image.LANCZOS)
    out = Image.new("L", (size, size), 0)
    out.paste(a, ((size - nw) // 2, (size - nh) // 2))
    return out


def save(alpha, rgb, path):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    im = Image.new("RGBA", alpha.size, rgb + (0,))
    im.putalpha(alpha)
    clean = Image.new("RGBA", im.size)
    clean.putdata(list(im.get_flattened_data()))
    clean.save(path, optimize=True)


ink = (104, 86, 62)
places = ["houseSmall", "houses", "ship", "houseChimney", "house", "campfire",
          "waterWheel", "tent", "treePines", "mill", "well", "dock"]
for i, p in enumerate(places, 1):
    save(shape(os.path.join(CA, p + ".png"), 96, pad=4), ink,
         os.path.join(site, f"static/bildoj/lokoj/l{i:02d}.png"))

kinds = {
    "reading": os.path.join(BG, "book_open.png"),
    "vocab": os.path.join(BG, "cards_fan.png"),
    "fillin": os.path.join(BG, "notepad_write.png"),
    "phrasebook": os.path.join(BG, "cards_order.png"),
    "multiplechoice": os.path.join(GI, "menuList.png"),
    "listening": os.path.join(GI, "audioOn.png"),
    "video": os.path.join(GI, "video.png"),
    "image": os.path.join(GI, "zoom.png"),
}
for k, p in kinds.items():
    save(shape(p, 40), (255, 255, 255), os.path.join(site, f"static/bildoj/specoj/{k}.png"))
print("ok")
