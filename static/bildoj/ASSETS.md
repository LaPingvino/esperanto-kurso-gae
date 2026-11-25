# The site's pictures: sources and licences

Every picture under `static/bildoj/` comes from an openly licensed set or is
our own drawing. The
site's credits page (`/fontoj`) shows the same, with every word picture's author.

| Folder | What | From | Author | Licence |
|---|---|---|---|---|
| `vortoj/` | a picture per word (145 words, the row per file in `internal/bildoj/vortbildoj.json`) | OpenMoji 17.0.0, https://openmoji.org | the OpenMoji designers; each file's author is in its row | CC BY-SA 4.0 |
| `parencoj/` | the family words (avo … genepoj, 24 words): one family tree, "me" ringed, the word's person(s) highlighted | `internal/bildoj/tools/familytree.py` | esperanto-kurso.net (our own work) | CC0 1.0 |
| `lokoj/` | a picture per Zagreb lesson, its place on the game's map: `houseSmall`, `houses`, `ship`, `houseChimney`, `house`, `campfire`, `waterWheel`, `tent`, `treePines`, `mill`, `well`, `dock` (lessons 1–12, as in the game's `spr_place`) | Cartography Pack | Kenney (kenney.nl) | CC0 1.0 |
| `specoj/` | a picture per exercise kind: `book_open` (reading), `cards_fan` (vocab), `notepad_write` (fill-in), `cards_order` (phrasebook) from Board Game Icons; `menuList` (multiple choice), `audioOn` (listening), `video` (video), `zoom` (image) from Game Icons | Board Game Icons 1.1, Game Icons | Kenney (kenney.nl) | CC0 1.0 |

## OpenMoji (CC BY-SA 4.0)

"All emojis designed by OpenMoji – the open-source emoji and icon project.
License: CC BY-SA 4.0", https://creativecommons.org/licenses/by-sa/4.0/.

Downloaded from the project's GitHub release 17.0.0
(https://github.com/hfg-gmuend/openmoji/releases/tag/17.0.0,
`openmoji-svg-color.zip`; the authors from the release's `data/openmoji.json`,
field `openmoji_author`).

Changes made (so these files are adaptations, under the same CC BY-SA 4.0):

- every file: the `id` attributes and white space removed and coordinates
  rounded to 2 decimals (in a 72-unit box; paths with arcs are left as they
  were), which renders the same at the sizes the site uses, compared pixel by
  pixel at 96 px (`internal/bildoj/tools/make_words.py`);
- `vortoj/26aa-9b9b9b.svg` (griza, grey): OpenMoji's white circle 26AA with its
  fill changed to #9b9b9b.

The files are named by their code points, so a picture's URL never spells its
word. The rest of the repository is MIT; these pictures stay CC BY-SA 4.0.

## The family tree (CC0 1.0)

`parencoj/` is drawn by `internal/bildoj/tools/familytree.py` from circles and
paths, borrowing only OpenMoji's conventions (72-unit box, flat fills of its
yellow and blue, black round-joined outlines), no OpenMoji artwork. Our own
work, dedicated to the public domain under CC0 1.0
(https://creativecommons.org/publicdomain/zero/1.0/), so it can be reused
anywhere (angla.org) without credit; the script is the source.

## Kenney (CC0 1.0)

Public domain dedication: no attribution required; credited anyway.
kenney.nl refuses automated downloads, so the packs came from Kenney's own
uploads on OpenGameArt:

- Cartography Pack: https://opengameart.org/content/cartography-pack (`cartographypack.zip`, `PNG/Retina/`)
- Board Game Icons: https://opengameart.org/content/board-game-icons (`kenney_board-game-icons.zip`, `PNG/Double (128px)/`)
- Game Icons: https://opengameart.org/content/game-icons (`Kenney_gameIcons.zip`, `PNG/White/2x/`)

Each icon is used as a shape only (its alpha): the lesson places filled with the
map's ink (104, 86, 62) at 96 px (drawn at 48 CSS px), the kind icons white at
40 px, used as a CSS mask in the text colour so they follow the theme. Resized
with Lanczos, saved with no metadata, then `optipng -o5`.
