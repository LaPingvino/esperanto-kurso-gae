# The game's pictures: sources and licences

Every picture in `sprites/` (and the paper's shades in `scripts/map_paper.gml`)
comes from an openly licensed pack. All of them are CC0 (public domain dedication): no attribution is
required, so the game shows no credits line. They are credited here anyway.

| Resource | What | From | Author | Licence |
|---|---|---|---|---|
| `scripts/map_paper.gml` | the map's paper folded in panels: each of `Textures/parchmentFolded.png`'s 4×4 panels as one flat shade (its mean light at a third of its strength) over the game's paper colour (246, 241, 231) | Cartography Pack (2020) | Kenney | CC0 1.0 |
| `spr_place` (44 px), `spr_place_s` (28 px) | a place's picture (`scripts/place_pic.gml`), frames 0–14 in ink (104, 86, 62), 15–29 the same faded (176, 164, 144) for a place not open yet. 0–11 one per Zagreb lesson (by the number in its slug, `zagr-lNN-teksto`): `houseSmall`, `houses`, `ship`, `houseChimney`, `house`, `campfire`, `waterWheel`, `tent`, `treePines`, `mill`, `well`, `dock`; every other place by its kind: 12 `castle` (a reading), 13 `bridge` (exercises), 14 `lighthouse` (a video) | Cartography Pack | Kenney | CC0 1.0 |
| `spr_compass` (60 px) | `compass`, ink at 55 % | Cartography Pack | Kenney | CC0 1.0 |
| `spr_kind` (22 px) | the exercise kinds (`ex_begin.gml`), in the course's green: 0 `book_open` (word cards), 1 `notepad_write` (fill-ins), 2 `cards_order` (sentences to build), 3 `hexagon_question` (multiple choice), 4 `card_flip` (vocabulary), 5 `book_closed` (typed answers: the phrasebook) | Board Game Icons 1.1 (2024) | Kenney | CC0 1.0 |

Sources (each pack's own `License.txt` states CC0 1.0,
https://creativecommons.org/publicdomain/zero/1.0/):

- Cartography Pack: https://kenney.nl/assets/cartography-pack, downloaded from
  https://opengameart.org/content/cartography-pack (`cartographypack.zip`,
  `PNG/Retina/*.png` and `Textures/parchmentFolded.png`).
- Board Game Icons: https://kenney.nl/assets/board-game-icons, downloaded from
  https://opengameart.org/content/board-game-icons (`kenney_board-game-icons.zip`,
  `PNG/Double (128px)/*.png`).

How they were made: each icon's shape (its alpha), cropped to its outline, is
filled with one colour, scaled with Lanczos (its longer side to the frame, centred) to the exact size the game draws it at (the engine scales
with nearest neighbour, so nothing is drawn stretched), and saved as a new PNG
with no text, time or profile chunks.

The paper is flat fills, not the texture itself: blending the texture as a
1280×656 picture took about 12 ms a frame, measured natively with Go's
image/draw as the engine draws it (the browser player redraws every frame; not
measured there), while a fill costs next to nothing. The shades were computed
once from the texture by a small script (not in the repository). Each panel is
mostly lighter than the plain paper and at most 2 below it, so every colour on
the map keeps its contrast at the darkest panel: the muted text 5.4:1 (AA),
the green 6.0:1, the stars 3.4:1 and the locked places' edge 3.0:1.

Where they are drawn: the paper (`map_paper.gml`) on a series' map
(`draw_map.gml`) and on the levels screen (`draw_levels.gml`); the compass on
the map when wide. Each place's picture above it on a wide map (beside it for
a page's 7th place, where the path comes down from above), at the row's end on
a phone (a long title is shortened to keep its stars or "‹ komencu" clear of
it). A series' picture is its first place's (`series_pic.gml`), at the right
of its tile on the levels screen: the Zagreb series shows its first lesson's,
the others what their places are. The course comes from the site at run time,
so pictures go by what a place is, never by a list of series. The exercise's
kind at the right of the progress line (`draw_progress.gml`). The reading and
exercise screens keep the plain paper.
