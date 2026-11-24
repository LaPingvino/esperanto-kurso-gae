# Esperanto-kurso: la ludo — design note

A GameViber version of esperanto-kurso.net: **another way into the same course, beside the site, not a
replacement.** The site stays the place for the adaptive path, accounts, comments and every series; the game
takes one well-shaped part of it, the Zagreb method's lessons, and plays it as a journey.

## Where the content comes from

The game never carries its own copy of the course. `cmd/export-game` (code in `internal/gamedata`) reads the
site's seed data at build time and writes `game/scripts/course_data.gml`, a GML script of global arrays (GML
has no JSON reader; the same approach as Sidenote's game). The file is generated and git-ignored;
`game/build.sh` makes it, then the web build.

| In the game | From the site | How |
|---|---|---|
| 12 lessons, in order, with their texts | `zagr-metodo` readings in `seed/ekzercoj-zagr-restructured.json` | as is |
| Fill-ins (62) | each reading's child series (`zagr-lNN`) | `Question()`, `GapAnswers()` through the site's own `model.ContentItem` |
| The interface words the site has ("Elektu la ĝustan respondon:", "Kion signifas tiu ĉi vorto?", "Mi ne scias", "Komenci", "Daŭrigi", "Sekva", "Antaŭa", "Tajpi", "Via respondo…") | `internal/locale/eo.json` | as is |
| A lesson's words (263) | `eo.ExtractWords` on the lesson text (the site's "practise the words of this text"), matched to the `zagr-vok` roots and their definitions | **derived**: the course links no words to lessons; the game's matching strips the ending and common suffixes (instruistino → instru) |
| Meanings in 20 languages | the `zagr-vok` definitions in the languages that are among the site's interface languages (`internal/locale`), complete for every word, drawable by the game's font (Latin, Greek, Cyrillic scripts), and not mostly copies of the English (as the Hindi ones are) | as is |
| Four tiles per gap | other gap answers of the same kind (an ending like *-oj*, or a word), the same lesson first | **derived** from the answers, never made up |
| Sentences to build (2 per lesson) | the lesson text's plain sentences of 3–7 words | **derived**, the words shuffled |
| Word cards: four meanings | other words of the same lesson | **derived** |

The answers are judged as the site judges them (`checkAnswer`): ends trimmed, capitals ignored, an answer per
gap or one answer cycled over every gap; the site checks no alternatives for fill-ins, so the game doesn't
either. Typed answers get the x-system exactly as the site's answer field does (`static/eo.js`).

## How the course becomes a game

- **The map:** the lessons are places on a path (two rows on a wide screen, a winding column on a phone). The
  first is open; each finished lesson opens the next and keeps its stars.
- **A lesson** opens with its text, as on the site ("Vidi en la retejo" opens the same reading there), then its
  exercises in a row:
  1. up to five of its words: *which meaning is this word's?* (four choices),
  2. the site's own fill-ins: a tile per gap, or typed (desktop keyboard, or "Tajpi", which brings up the phone's
     keyboard),
  3. two sentences of its text to put back in order.
  A wrong answer shows the right one and comes back at the end of the row, as the site keeps a series open until
  each item is right. Stars count the first tries (3: 90 %, 2: 60 %).
- **The collection:** the words of finished lessons ("Vortoj N" on the map), each with its meaning in the
  chosen language ("Signifoj: EN" cycles through the 20).
- **Words with affixes** show as their root (*sport-*, "en la teksto: sportisto"), so the meaning shown is
  the root's: *patrino* never appears as "father".

### Audio

The course has no audio yet: the seed holds `audio_url` on four items, all empty, and the site's listening
template plays whatever URL an item has. So the game ships no sound and no listening exercise. The hook is
there: an exercise kind "listen and answer" can play a sound per item (GameViber's sounds) wherever the export
finds an `audio_url`. The natural source is the esperanto-voice Piper voice once it is released (rendered at build
time per sentence of the lesson texts) — a question for Joop, not done here.

## Progress, and a site account later

Progress is local: a small file (`esperanto-kurso-ludo.txt`: version, language, the stars per lesson) through
GameViber's `file_*` functions, which in the browser live in the page's localStorage
(`gameviber/0/files/…`); on the desktop it is a file in the working folder.

Linking to an account later, if served from esperanto-kurso.net itself (e.g. `/ludo/`): the site already
keeps its token in that origin's localStorage (`eo_token`) and accepts attempts at
`POST /ekzerco/{slug}/provo` with `X-Auth-Token`. Every fill-in in the game carries its site slug, so a right or
wrong answer could be sent as an attempt, and the site's progress could fill the map's stars. What that needs:
a way out of the game to the page (GameViber has no HTTP function; a small generic `page_call` / JS bridge on
the web build would do), and a decision on whether game attempts should move the Glicko ratings (the tiles make
a fill-in easier than typing it). The game's own exercises (word cards, sentences) have no site slug and
would stay local.

## Look and input

- Calm and friendly: warm paper, one green from the course, gold stars; no timers, no lives.
- Contrast (WCAG): text 12.5:1 on the paper; muted text 5.5:1; white on green 6.8:1; the green as text
  6.1:1 (the quieter link green 7.0:1); right 5.4:1 and wrong 5.5:1 on their tints; locked lesson numbers
  4.1:1; tile and button edges 3.1:1 (1.4.11); earned stars 3.5:1 on the paper (unearned ones are the pale
  lock colour); the keyboard's focus ring 5.9:1.
- A wide room (1280×720) and a tall one (420×860), switched by the window's shape, as Sidenote's game does;
  the text is drawn at the screen's real resolution.
- Keyboard: arrows move the focus, Enter presses (or the screen's main button), 1–4 pick a choice, typing
  fills a gap, Esc clears or goes back to the map. Mouse and touch: press what you see.

## GameViber changes this needed (branch `web-extras`, generic)

- `[settings] touch = "tap"`: a finger is the left mouse button where it touches, with no arrow pads. Without
  it (checked in headless Chromium with phone emulation) a tap on lesson 1 opens nothing and the first touch
  shrinks the game for arrow pads: Ebitengine cancels the browser's touch-to-mouse emulation.
- Typed characters reach `keyboard_string` (nothing fed it before). Backspace works, and
  `[settings] typing = true` makes M a letter rather than the sound shortcut.
- The margin round the frame takes the room's background colour instead of black; the player's sound and
  full-screen buttons and the pads draw dark on a light margin.

## Checks

- `internal/handler/game_parity_test.go`: the game's parity variant (`game/parity.toml`) answers every exercise
  through its own GML. The site's `checkAnswer` must agree on all 62 fill-ins, right and wrong; every gap must
  offer its answer exactly once; 24 sentences and 263 word cards must accept their answer and reject another.
- `internal/gamedata/play_test.go`: the lessons played headless through the real keys (focus and Enter) — 1, 1–2
  and all 12 in one go — in both rooms. Each ends with three stars and opens the next lesson.
- In headless Chromium (desktop 1366×768 and 1920×969; a phone 390×844 at 3× with touch): keyboard, typing
  with the x-system, mouse, taps, the phone's prompt for typing, the progress kept over a reload.
- Both need `GAMEVIBER=/path/to/gameviber` and are skipped without it.

## Next

1. The rest of the A0–A2 material as further regions of the map (the A0 readings on the alphabet and
   greetings come before lesson 1; `gramatiko-mc` multiple choice; the phrasebook).
2. Audio with the esperanto-voice voice, and with it a listening exercise.
3. The account link above, once the hosting is settled.
4. Spaced review: words from finished lessons coming back as cards on the map.
