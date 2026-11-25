# Esperanto-kurso: la ludo — design note

A GameViber version of esperanto-kurso.net: **another way into the same course, beside the site, not a
replacement** — the whole course, loaded from the site while it plays.

## The whole course, loaded at run time

The game carries no copy of the course. The site serves it (`internal/handler/game_api.go`), built by
`internal/gamedata` from the same approved content the site shows (the content store's in-memory cache, so no
Datastore reads of its own; built again only when that cache reloads):

- `GET /ludo-api/kurso` — the map: levels, series, places;
- `GET /ludo-api/leciono/{slug}?lingvo=xx` — a place: its text and exercises, meanings in xx (else English).

Plain text lines of tab-separated fields (`gamedata/text.go`; GML has no JSON reader), each ending in `FINO`
so a cut-off text is never taken for a whole one. Mounted outside AuthMiddleware: the same for everyone, no
user looked up or created, no cookie, `Cache-Control: public, max-age=300` and an ETag by the course's
version (304 when unchanged). **New or changed content on the site shows up in the game without a rebuild.**

In the game: the map is asked for at the start, a place's lesson when it opens; each is kept in the
browser (localStorage files `ludo-kurso.txt`, `ludo-L-<place>-<lang>.txt`) and the cached copy used at once
when its version is the current one. Offline: the cached copies; with nothing cached, a built-in part (the
Zagreb lessons, `scripts/course_data.gml`, made from the seed by `cmd/export-game`), else "try again".

**The shape is the site's:** levels (KER A0 …), each series in the level of its first item's rating, sorted
by label — as `/serioj` groups them. A reading is a place (its text, its child series' exercises, word cards
from its text, two sentences to build); a video is a place (watched on the site, one star); other exercises
come in places of ten (`<series>.<n>`); items in no series go, by level, into an "Aliaj" series (vocabulary
tagged with a reading is that reading's words). Zagreb places keep their reading slugs (`zagr-lNN-teksto`),
so the stars already on accounts stay theirs. The "play as a game" link on every exercise page leads to the
place showing that item.

**Inventory (the seed; production has the same kinds, with its own labels):** 1633 items — 1454
vocabulary items (14 `vortaro-*` series, 507; the Zagreb vocabulary, 597; words attached to readings, 329;
a few loose), 67 readings (Zagreb 12, Fundamenta Ekzercaro 31, PMEG 9, literature 8 + 4, articles 2, A0 2,
A2 1, loose 2), 64 fill-ins (Zagreb 62, loose 2), 7 multiple choice (`gramatiko-mc` 5, loose 2), 2
phrasebook, 33 videos (Mazi 12, Pasporto 16, music 4, the alphabet 1). In the game: 32 series, 226 places;
64 fill-ins, 7 multiple choice, 1127 vocabulary words, 2 typed answers, 304 word cards, 77 sentences.

| Encounter | From | Judged |
|---|---|---|
| fill-in: a tile per gap, or typed | the site's fill-ins | as `checkAnswer`: an answer per gap or one for all, no alternatives |
| multiple choice | the site's (`gramatiko-mc`, A0 basics) | the chosen option's text against the right one's, or an alternative |
| vocabulary: the meaning given, the word? tiles or typed | the site's vocabulary items | the word or an alternative |
| typed answer: the question, tiles or typed | the phrasebook (and any type with a question and an answer) | the answer or an alternative |
| word card: which meaning? | **derived**: words of a reading (`eo.ExtractWords`, matched by root) | the game's own |
| sentence to build | **derived**: a reading's plain sentences | the game's own |
| video | the site's videos | watched on the site |

Tiles are **derived** from the other answers of the same kind (same place, then series, then all), never made
up. Typed answers get the x-system as the site's answer field does (`static/eo.js`).

## How the course becomes a game

- **The levels:** the first screen shows the course by level, each series a tile with its places done.
- **A series' map:** its places on a path (two rows on a wide screen, a winding column on a phone; twelve to a
  page). The first is open; each finished place opens the next and keeps its stars.
- **A place** opens with its text when it has one, as on the site ("Vidi en la retejo" opens the same page
  there), then its exercises in a row (see the table above); a video place is watched on the site.
  A wrong answer shows the right one and comes back at the end of the row, as the site keeps a series open until
  each item is right. Stars count the first tries (3: 90 %, 2: 60 %).
- **The collection:** the words of the word cards of places done ("Vortoj N"), with their meaning in the
  language chosen then ("Signifoj: EN" cycles through the 20; the account's language is the default).
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
32 MB per file; 6.2 MB gzipped; the built-in part of the course, 33 KB, in project.tar — the rest comes from
/ludo-api/). The site links it from the menu (🎲, "Ludo: la kurso kiel ludo" in all 31 interface languages)
and from every exercise page the game shows ("Ludi ĉi tiun lecionon kiel ludon" → `/ludo/?leciono=<place>`,
which opens that place in the game). The game's way back: "Retejo"
and "Vidi en la retejo" open the site in the same tab. When the app runs by itself (`go run`), it serves
`/ludo/` from `static/ludo` too.

**Progress, both ways, in the background.** Every visitor of the site has an account (anonymous until they
sign in), carried by the site's `token` cookie; the game's requests are same-origin, so they carry it too.
GameViber's generic `http_post` / `http_get` (answers in the `other-62` event) do the talking; nothing in the
engine knows the site.

- *Typed answers → the site's own `POST /ekzerco/{slug}/provo`.* A fill-in, vocabulary word or typed answer
  typed in every gap is queued
  (kept in the progress file), sent one at a time, and taken off the queue only when the site has it.
  **Rule: only typed answers go to the site, so only they move the Glicko ratings; an answer built from
  tiles never does** (choosing is easier than typing, and the ratings are calibrated on typing). Multiple
  choice isn't sent either, though the site asks for a choice there too: a question for Joop. The game's own
  exercises (word cards, sentences) stay in the game.
- *Stars ↔ `/ludo-progreso`.* The game posts its stars per lesson; the site keeps them on the account
  (`User.GameStars`) and answers with the account's state: the stars kept there, and every exercise answered
  rightly on the site. **Merge rule: the larger per place, both on the account and in the game**, so a
  sync can be repeated, cross another device's, or arrive late, and nothing is ever lost. A place whose site
  exercises were all answered rightly on the site (the site works it out: "D" lines) counts in the game as
  done with one star.
- *The old progress file* (versions 1–2: stars by Zagreb lesson number, the language by its place in that
  game's list, the queue) is read into the new one (version 3: stars by place slug, the collection).
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

- `internal/handler/game_parity_test.go`: the parity variant (`game/parity.toml`) fetches the map and every
  place's lesson from the real `/ludo-api/` (over the seed) and answers every exercise through the game's own
  GML; the site's `checkAnswer` must judge alike — all 1200 site exercises (fill-in, multiple choice,
  vocabulary, typed answer), right and wrong; every gap offers its answer once among its tiles; 77 sentences
  and 304 word cards take their answer and refuse another.
- `internal/handler/game_play_test.go`: played headless through the real keys against a stub of the site (the
  real API and progress handlers, provo judged by `checkAnswer`): a place of every kind — a Zagreb lesson,
  another reading, vocabulary, multiple choice, the phrasebook, a video — each to its stars, the Zagreb lesson
  in the tall room too. Sync: the site's progress comes in (a place done on the site, the account's stars);
  only the typed answers go out (one refused first: retried), all right by `checkAnswer`; the stars end on
  the account. Offline: from the built-in part, the queue waits and goes on the next opening; the old
  progress file is read into the new one.
- `internal/handler/game_api_test.go`: the API (text, ETag / 304, languages, no cookie, 404), the course built
  once per content set, the items' places, a course built from items without series fields.
- `internal/handler/game_test.go`: the merge rule, the progress format and handler.
- The whole site run locally (the Datastore emulator, seeded) and played in headless Chromium.
- The game-playing tests need `GAMEVIBER=/path/to/gameviber` and are skipped without it.

## Next

1. Audio with the esperanto-voice voice, and with it a listening exercise.
2. Spaced review: words from finished lessons coming back as cards on the map.
