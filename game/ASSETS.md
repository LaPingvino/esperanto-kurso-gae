# The game's pictures: sources and licences

Every picture in `sprites/` comes from an openly licensed pack. All of them are CC0 (public domain dedication): no attribution is
required, so the game shows no credits line. They are credited here anyway.

| Resource | What | From | Author | Licence |
|---|---|---|---|---|
| `spr_place` (44 px), `spr_place_s` (28 px) | a place's picture (`scripts/place_pic.gml`), frames 0–14 in kurso-design's muted ink (44, 74, 69), 15–29 the same faded (128, 156, 150) for a place not open yet. 0–11 one per Zagreb lesson (by the number in its slug, `zagr-lNN-teksto`): `houseSmall`, `houses`, `ship`, `houseChimney`, `house`, `campfire`, `waterWheel`, `tent`, `treePines`, `mill`, `well`, `dock`; every other place by its kind: 12 `castle` (a reading), 13 `bridge` (exercises), 14 `lighthouse` (a video) | Cartography Pack | Kenney | CC0 1.0 |
| `spr_compass` (60 px) | `compass`, the muted ink at 45 % opacity | Cartography Pack | Kenney | CC0 1.0 |
| `spr_kind` (22 px) | the exercise kinds (`ex_begin.gml`), in the accent text green (15, 80, 36): 0 `book_open` (word cards), 1 `notepad_write` (fill-ins), 2 `cards_order` (sentences to build), 3 `hexagon_question` (multiple choice), 4 `card_flip` (vocabulary), 5 `book_closed` (typed answers: the phrasebook) | Board Game Icons 1.1 (2024) | Kenney | CC0 1.0 |

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

They were first drawn in the game's earlier warm palette and recoloured (alpha kept) when the game moved
to kurso-design (2026-11-25). The folded-paper shading of the map (Cartography Pack's
`parchmentFolded`) was dropped then: its panels read as rendering glitches.

Where they are drawn: the compass on a series' map (`draw_map.gml`) when wide. Each place's picture above it on a wide map (beside it for
a page's 7th place, where the path comes down from above), at the row's end on
a phone (a long title is shortened to keep its stars or "‹ komencu" clear of
it). A series' picture is its first place's (`series_pic.gml`), at the right
of its tile on the levels screen: the Zagreb series shows its first lesson's,
the others what their places are. The course comes from the site at run time,
so pictures go by what a place is, never by a list of series. The exercise's
kind beside its word at the top of the stage (`draw_exercise.gml`).

The sounds (`sounds/`) are the site's own, rendered by `cmd/game-sounds` from
`static/eo.js`'s Web Audio settings: made for the site, no licence beyond it.
