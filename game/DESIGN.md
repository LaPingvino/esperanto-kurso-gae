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

## In the site: /ludo/, and progress on the account

**Served by the site.** `make ludo` builds the game into `static/ludo/` (git-ignored, not gcloud-ignored), and
app.yaml serves it at `/ludo/` as static files (the .wasm as `application/wasm`; 24.9 MB, under App Engine's
32 MB per file; 6.2 MB gzipped). The site links it from the menu (🎲, "Ludo: la kurso kiel ludo" in all 31
interface languages) and from every Zagreb lesson and its fill-ins ("Ludi ĉi tiun lecionon kiel ludon" →
`/ludo/?leciono=zagr-lNN-teksto`, which opens that lesson's text in the game). The game's way back: "Retejo"
and "Vidi en la retejo" open the site in the same tab. When the app runs by itself (`go run`), it serves
`/ludo/` from `static/ludo` too.

**Progress, both ways, in the background.** Every visitor of the site has an account (anonymous until they
sign in), carried by the site's `token` cookie; the game's requests are same-origin, so they carry it too.
GameViber's generic `http_post` / `http_get` (answers in the `other-62` event) do the talking; nothing in the
engine knows the site.

- *Typed answers → the site's own `POST /ekzerco/{slug}/provo`.* A fill-in typed in every gap is queued
  (kept in the progress file), sent one at a time, and taken off the queue only when the site has it.
  **Rule: only typed answers go to the site, so only they move the Glicko ratings; an answer built from
  tiles never does** (choosing is easier than typing, and the ratings are calibrated on typing). The game's
  own exercises (word cards, sentences) stay in the game.
- *Stars ↔ `/ludo-progreso`.* The game posts its stars per lesson; the site keeps them on the account
  (`User.GameStars`) and answers with the account's state: the stars kept there, and every exercise answered
  rightly on the site. **Merge rule: the larger per lesson, both on the account and in the game**, so a
  sync can be repeated, cross another device's, or arrive late, and nothing is ever lost. A lesson whose
  fill-ins were all answered rightly on the site counts in the game as done with one star.
- *When.* At the start, after each finished lesson and each delivered answer, and every 20 seconds, which
  covers coming back to the tab after working on the site (GameViber has no focus event yet).
- *Offline and failures.* No answer (status 0) or a server error: tried again after 1 s, then 2 s, 4 s … at
  most a minute. The queue lives in the progress file, so it survives a reload and goes the next time the
  game opens with the site reachable. A 4xx for an answer (no such exercise) drops it.
- *Signed out, signing in.* Playing without signing in fills the anonymous account; signing in merges
  accounts (`UserStore.MergeUsers`, the stars by the same larger-wins rule), and the game's own copy is
  pushed again on its next sync, so nothing is lost either way.
- *Outside the site* (the desktop, another host): `page_url()` is empty, so the game keeps its progress only
  in its own file (`esperanto-kurso-ludo.txt`, in the browser in localStorage).

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

- Asynchronous HTTP from GML: `http_get(url)`, `http_post(url, name, value, …)` return an id; the answer
  runs `other-62.gml` (Studio's Async HTTP event) with `http_id()`, `http_status()` (0: no answer),
  `http_result()`. In the browser a same-origin fetch; elsewhere net/http with a cookie jar.
  `page_url()`; `url_open_ext(url, target)`; `gameviber play -base URL` and `text=WORD` in key scripts, so a
  game that talks to its site can be played through headless.

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
- `internal/gamedata/sync_test.go`: the game headless against a stub of the site (the real `/ludo-progreso`
  handler over an in-memory account): the site's progress comes into the game; a lesson played with one
  fill-in by tiles and the others typed sends exactly the typed ones, all judged right, the first after one
  refusal (retry); the stars end on the account. And offline: the queue waits in the progress file and goes
  when the game is opened again with the site up.
- `internal/handler/game_test.go`: the merge rule, the endpoint's format and its handler, the lesson links.
- The whole site run locally (`go run` + the Datastore emulator, seeded), played in headless Chromium.
- The game-playing tests need `GAMEVIBER=/path/to/gameviber` and are skipped without it.

## Next

1. The rest of the A0–A2 material as further regions of the map (the A0 readings on the alphabet and
   greetings come before lesson 1; `gramatiko-mc` multiple choice; the phrasebook).
2. Audio with the esperanto-voice voice, and with it a listening exercise.
3. Spaced review: words from finished lessons coming back as cards on the map.
