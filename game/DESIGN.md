# Esperanto-kurso: la ludo — design note

A GameViber version of esperanto-kurso.net: **another way into the same course, beside the site, not a
replacement** — the whole course, loaded from the site while it plays.

## The whole course, loaded at run time

The game carries no copy of the course. The site serves it (`internal/handler/game_api.go`), built by
`internal/gamedata` from the same approved content the site shows (the content store's in-memory cache, so no
Datastore reads of its own; built again only when that cache reloads):

- `GET /ludo-api/kurso` — the map: levels, series, places;
- `GET /ludo-api/leciono/{slug}?lingvo=xx` — a place: its text and exercises, meanings in xx (else English).
- `GET /ludo-api/vortoj?lingvo=xx` — the vocabulary words for the bonus games (`gamedata/words.go`): each word
  with its level, meaning, masked Esperanto definition and picture, and the day's Vortludo word per level;
  and Frazotrajno's sentences, each with its level and place (`gamedata/trains.go`).

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
up. Typed answers get the x-system as the site's answer field does (`static/eo.js`), and the h-system (ch → ĉ,
sh → ŝ …; its ŭ is a plain u), both converted as you type (`xsys.gml`). The h-system's letters also occur in real
words (*flughaveno* shows as *fluĝaveno*), so a typed answer is judged by both readings (`hfold`, `gap_fix`) and
the site is sent the answer it matched, which `checkAnswer` then judges the same.

## The bonus games: the same words, learnt otherwise

As OpenTeacher does with one list, the course's vocabulary comes back as games, from `/ludo-api/vortoj`, kept in
the browser (`ludo-vortoj-<lang>.txt`) like the lessons. They are reached from the levels screen (*Ludoj*, above
the levels) and as the *Bonuso* after a finished place. **Neither sends anything to the site's progress:** a
Wordle guess answers no item's question and a gate is a choice, so the ratings still move only by typed course
answers (Vortludo only asks whether a guess is a word, below).

- **The learner's level:** the account's, from its rating once settled (RD < 200: `/ludo-progreso`'s `R` line),
  else the level the map is at.
- **Vortludo** (`vl_*.gml`): a Wordle on the course's words: in the A levels the five-letter ones (ĉ ĝ ĥ ĵ ŝ
  ŭ one letter each; no bare roots); from B1 longer ones, six to nine letters, each a root, one affix and the
  ending, read alike by ReVo and by the pieces alone (`legvortoj.OneAffix`: *komput·il·o*, *lern·ej·o*). The
  affix alone (never the ending) is the clue, shown **beside** the board, not in its place: in a long word it may
  sit anywhere, and the player works out where; its letters count as any others. The day's word is the site's
  (`T` lines: the same for every learner of a level, by the UTC date); *Nova vorto* plays a word of the level's
  pool at random (the level, then below, then above, to ten: `gamedata.Pool`, mirrored by `vl_pool.gml`), the
  board showing its length. A guess must be a word, but any word of the language, not only the course's: a ReVo
  headword or its inflection, or known roots and affixes with an ending (*domŝipo*, *malbonulejo*, *tiujn*;
  `legvortoj.Valid`). A word of the course's list goes in at once; another is asked of the site
  (`GET /ludo-api/vorto?v=…`, x-system, `1`/`0`, cached a day); a guess it knows as no word shakes the row and
  costs no try, as in Wordle. Offline, on an error or after two seconds without an answer, any letters count.
  (Asking was chosen over shipping the morphemes: 13 461 ReVo roots are only 38 KB deflated, but the headwords,
  the inflections and the splitter would be a second copy in GML of what `legvortoj` already does, sure to
  drift; one small cached GET per guess is quick enough, and no ReVo (GPL) data goes into the game.) Six rows; typed (x- and h-systems) or on the screen's keyboard of the 28 letters; the cells
  shrink to fit nine on a phone. In its place: the accent fill; elsewhere in the word: the note tint and a dot
  under the letter; not in it: sunk. After three misses it offers the word's picture (`spr_bildo`, the site's
  own word pictures) or its Esperanto definition with the word masked (~), else its meaning.
- **Vetkuro** (`rc_*.gml`): a road of three lanes; the question goes by the word, not the learner's language:
  its picture (the same as Vortludo's hint), else its Esperanto definition (masked), and only for a word with
  neither its meaning, marked with its language ("(en) car"); two words asking the same never share a race. A gate of three words comes
  down it; steer into the right one (arrow keys, a tap on a lane, a swipe). A wrong gate is a crash: a life of
  three, the right word shown with ✓. A gate takes 9 s down the road at a rating of 1000, 5 s at 2000, a little
  less with each point. The bonus after a place races with that place's words (four or more), else the
  level's. This is the one place with lives and a clock, on purpose and only on request: the course itself
  stays calm.
- **Konstrui** (`kb_*.gml`): a word built from its pieces, the Zagreb way. Only words whose split is beyond
  doubt (`legvortoj.Blocks`: one root, one or more affixes, a dictionary ending, ReVo's reading and the pieces'
  alone agreeing), shown as tiles *mal-*, *san*, *-ul-*, *-ej-*, *-o* (the `W` line's last field). Asked by the
  word itself, as in Vetkuro: its picture, else its masked definition, else its meaning under "Traduko". The
  bank: the word's pieces, another word's root, an affix of its first affix's kind and a suffix, another
  ending and -j or -n — ten tiles at most. Tap (or a number key, 0 the tenth) puts a piece on the line, tap
  on the line takes it back; the word they spell shows under the line. Kontroli judges by the spelling; after
  it each placed piece shows right or wrong (with its badge), and the dock gives the word in its pieces. The
  pool: the learner's level's words and, to ten, the levels below — never above (`gamedata.BuildPool`,
  `kb_pool.gml`); after a place, its own words when three or more have pieces. Nothing goes to the site.
- **Memoro** (`mm_*.gml`): pairs face down, the classic memory game. Six words, each twice: the word, and its
  picture, else its masked Esperanto definition — no translations, so the pair is Esperanto on both sides (and
  no typing: the x- and h-systems don't come in). Two words asking alike never share a board. Twelve cards,
  three across on a phone, six on a wide screen, the star on their backs; press two (a tap, or the arrows and
  Enter: every card stays pressable so the focus keeps its place). A pair stays up with ✓; two that aren't
  one show ✗ and turn back after 1.5 s or at the next press. The board done: its tries, and Denove. The
  pool (`gamedata.MemoPool`, `mm_pool.gml`): the course's own words (not those derived for Konstrui) of the
  learner's level and, to twelve, the levels below — never above; only those whose card fits a phone's
  (a definition of 66 letters at most, no word of it or the word itself over 12, `gamedata.MemoOK`).
  After a place, its own words when four or more can pair. Nothing goes to the site.
- **Frazotrajno** (`ft_*.gml`): the sentence train. A sentence of the course's readings, its words as wagons in
  the depot (shuffled, never in the text's order); press them behind the locomotive one by one (a tap, the
  arrows and Enter, or a number key; the depot's slots stay pressable so the focus keeps its place), press a
  coupled one to send it back. The last one coupled, the train is judged at once, no Kontroli: right (✓ on
  every wagon, the sentence written out with its stop) and it goes on after 1.5 s or at a press; wrong, the
  right start stays coupled with ✓, the wagons from the first one out of place show ✗ and go back after 1.5 s
  or at the next press. Five sentences a round; done, its tries and Denove. **Judged by the text's order
  only** (alike words may swap): Esperanto allows other orders, but which ones depends on stress and scope
  (*ne*, *nur*, *ankaŭ*, a fronted object) that the game can't check safely, so it plays only sentences where
  the text's order is plainly the one to find — the readings' plain sentences (`plainSentences`, as the
  sentences to build in a place) of three to seven words, with no comma (a wagon would lose it) and no *kaj*
  or *aŭ* (*patro kaj frato* is as right as *frato kaj patro*) (`gamedata.TrainOK`). The pool
  (`gamedata.TrainPool`, `ft_pool.gml`): the learner's level's sentences and, to ten, the levels below — never
  above; after a place, its own when it has two or more. The word list's format went to 2 with them
  (`WordsFormat`, in the ETag), and a kept copy without them waits for the site's. Nothing goes to the site.
- **Vortpluvo** (`pv_*.gml`): the word rain. After Komenci the course's words fall in the field; type one whole
  before it reaches the ground and it is caught (✓, rising), and the next comes at once when none is left
  falling; one that lands costs a life (✗ on the ground), the third ends the game with its words and the best of
  the session, and Denove plays again. The fall quickens with every word caught (a whole fall from nine seconds
  down to two and a half), a new word comes every 100 steps less three per word caught (36 at least), six at
  most at a time. Typed letters go through the x- and h-systems as in Vortludo (`pv_letters`: `xsys`, small
  letters, the alphabet's 28 — the x of *aux* caught at its u drops), or come from the screen's keyboard
  (`keys_screen`, Vortludo's, shared); a word is caught when the letters spell it by **either reading of the
  h-system** (`hfold`: *ankau* is *ankaŭ*, *flughaveno* shown *fluĝaveno* is still *flughaveno*), so no
  spelling is ever ambiguous at the moment of catching. To keep it so, a word never falls while one it starts
  or that starts it is falling (*la* beside *lago* would be caught before *lago* could be typed). The typed
  letters sit under the ground (✗ when no falling word starts so; Enter or a space clears them), the last word
  caught beside them with its meaning. The pool (`gamedata.RainPool`, `pv_pool.gml`): the learner's level's
  words and, to twenty, the levels below — never above; the course's own (not derived), one word of two to
  twelve small letters (`gamedata.RainOK`). After a place, its own words when four or more can fall. From the
  word list's W lines: no line of its own. Nothing goes to the site.
- **Bonvorto** (Vortludo's board, `vl_bona`; `bv_start.gml`): La Bona Lingvo's pairs (the *Bona lingvo* series,
  `cmd/bonalingvo`): the word with a root of its own is given (*Anstataŭ* **hobio**), the word of the same meaning
  built from basic roots is the one to find (*ŝatokupo*), six rows, typed in the x- and h-systems or on the screen's
  keyboard, a guess asked of the site as in Vortludo (the word itself always taken). Its clue, beside the board in
  the affix's place, is its **shape**: the project's own division with the roots hidden (*Formo:* …·…·o,
  …·ar·o, mal·…·a), so a compound's linking o is never a guess; at the end the word shows in its pieces
  (*ŝat·okup·o*) with its meaning, which is also the hint after three misses. The division is the project's
  (`gamedata/bonalingvo.tsv`, from `cmd/bonalingvo -dividoj` beside the series), not ReVo's splitter, which gets
  *profund·eg·o* or *kurac·at·o* wrong. The pairs are the course's Bona lingvo exercises asking for the built word
  (`gamedata.Pairs`, `B` lines, format 3) whose word has six to nine letters (the long board): 65 of 101. Every
  pair, whatever the learner's level (the series is C1–C2; the board with the shape is help enough); *Nova vorto*
  another at random. In the Ludoj row only (the series' four columns: four and three wide, rows of two on a phone),
  not the Bonuso: a place's words aren't its pairs. Nothing goes to the site.


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

### Sound

The site's own two sounds, at the same moments: `static/eo.js` plays a rising two-note chime on a right
answer and a short low buzz on a wrong one, made with Web Audio (no files). `cmd/game-sounds` renders the
same oscillators, gains and ramps into `sounds/snd_right.wav` and `sounds/snd_wrong.wav`; `ex_done` plays
them with the verdict (after Kontroli, or "Mi ne scias"). The site has no other sounds (the level-up has
confetti only), so neither does the game. The player's sound button (and M, which a typing game gives to the
keyboard) silences them, and the browser remembers it (GameViber, below).

The course itself has no audio yet (the seed's `audio_url`s are empty), so there is no listening exercise;
the natural source is the esperanto-voice Piper voice once it is released.

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

The shared kurso-design (`~/Projects/kurso-design`, DESIGN.md and tokens.css): the light green-teal base
and the Esperanto green accent, its tokens copied as the game's colours (`setup.gml`). Calm: no timers, no
lives, a wrong answer is a plain *Ne tute.* with the right one.

- **The stage.** An activity runs under a top bar with the site's sign (the green disc with the star), the
  progress through the place, *n/N* and close (×). The body is at most 640 wide: the kind (icon and word),
  the question, the answer. The **dock** is always at the bottom: *Kontroli* (disabled until there is an
  answer) with *Mi ne scias* (and *Tajpi*), then the verdict — *Ĝuste!* with a ✓ disc and a small gold
  star, or *Ne tute.* with a ✗ disc and the right answer — and *Daŭrigi*. A pick is only judged at
  Kontroli: a choice can be changed, a filled gap pressed to empty it, a placed word taken back.
- **One tile, five states** (`tilebox.gml`), each with a cue besides its colour: new (surface, edge, a
  4 px ledge), trying (accent border 3 px, accent ledge and text, no tint so it never looks "right"),
  right (tint, ✓ badge), wrong (tint, ✗ badge), done (sunk, dashed, sits lower); a word taken from the
  bank leaves a dashed slot, so nothing jumps; a built sentence gets one badge at its end.
- **Every text in a box wraps inside it** and the box grows to fit: options, word tiles, gaps, buttons,
  series tiles, the dock. There is no scrolling, so a multiple-choice screen steps its type down
  (`draw_choice`, measured by `choice_h`) until question and options fit above the dock; after the verdict only
  the right option and a wrong pick stay. Anything that still doesn't fit counts in `global.ovf` (the
  tests expect 0). Checked on every live place of the site (212, in English and German, both rooms, wrong
  answers first so every verdict dock shows), e.g. "„da“ diras kvanton (glaso plena de vino); „de“
  montras alian rilaton, ekz. glaso por vino (vinglaso)".
- **The levels** as KER chips (the current level filled, the others tinted), series as tiles with their
  picture; **a series' map** as a line with stations: done (filled), here (ringed, *Vi estas ĉi tie*),
  ahead (hollow). The folded-paper shading is gone (its panels read as rendering glitches).
- **Contrast**: every text 7:1 or more (tokens.css's checked pairs: ink 12.95:1 on the paper, muted 8.42:1,
  the accent text 8.34:1, white on the green 7.34:1, muted on sunk 7.44:1, the verdicts' texts 7.24:1 and
  up on their tints); edges, the line, progress and badges 3:1 or more; the focus ring in the accent, 3 px.
- Drawn with the engine's own font (Liberation Sans): kurso-design's faces (Source Sans 3, Charis SIL,
  Fraunces) would need GameViber to load a game's fonts. Light only for now.
- A wide room (1280×720) and a tall one (420×860), switched by the window's shape, as Sidenote's game does;
  the text is drawn at the screen's real resolution.
- Keyboard: arrows move the focus, Enter presses (or the screen's main button: Kontroli, Daŭrigi), 1–4
  pick a choice, typing fills a gap, Esc clears or goes back to the map. Mouse and touch: press what you see.

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

- The sound button is remembered in the browser (localStorage `gameviber/<id>/muted`): a game left muted
  opens muted.

## Checks

- `internal/handler/game_parity_test.go`: the parity variant (`game/parity.toml`) fetches the map and every
  place's lesson from the real `/ludo-api/` (over the seed) and answers every exercise through the game's own
  GML; the site's `checkAnswer` must judge alike — all 1200 site exercises (fill-in, multiple choice,
  vocabulary, typed answer), right and wrong; every gap offers its answer once among its tiles; 77 sentences
  and 304 word cards take their answer and refuse another.
- `internal/handler/game_play_test.go`: played headless through the real keys against a stub of the site (the
  real API and progress handlers, provo judged by `checkAnswer`): a place of every kind — a Zagreb lesson,
  another reading, vocabulary, multiple choice, the phrasebook, a video — each to its stars through Kontroli
  and Daŭrigi, every text fitting its box (`global.ovf == 0`), the Zagreb lesson in the tall room too. Sync: the site's progress comes in (a place done on the site, the account's stars);
  only the typed answers go out (one refused first: retried), all right by `checkAnswer`; the stars end on
  the account. Offline: from the built-in part, the queue waits and goes on the next opening; the old
  progress file is read into the new one.
- `internal/handler/game_ludoj_test.go`: Vortludo from the levels screen in both rooms — three misses typed in
  the x-system, the hint, the day's word typed in the h-system, found on the fourth row; a long word's board:
  a guess that is no word shakes and costs no try, tranĉiloj is asked of the site and taken; Vetkuro driven to the
  right lane (the `auto` variant, steering through the keys' `rc_steer`) scores and keeps its lives, held in one
  lane it crashes and the counts add up (both rooms), and the bonus after a place races with its words;
  Konstrui built right and wrong in both rooms, its pools the site's and every bank sound; Memoro's board played by the keys in both rooms (a pair, two that aren't and turn back, the board done in seven tries, Denove), its pools the site's, every word's cards fitting, the bonus after a place with its words; Frazotrajno's round played by the keys in both rooms (a train right and gone on, a wagon taken back, a train out of order with its right start kept, a number key, the round done in six tries, Denove), its pools the site's, every sentence's train drawn whole in every state with its depot shuffled, the bonus after a place with its sentences; Vortpluvo played in both rooms (a ŭ word typed in the h-system with a plain u and caught, one in the x-system, a hatted letter from the screen's keyboard, letters no word starts, three landed and the game done, Denove), its pools the site's, every word falling whole in the field typed or not, the bonus after a place with its words; Bonvorto over the seed and the Bona lingvo series in both rooms (the word to replace and the shape beside the board, a no-word shaking, two misses asked of the site, the meaning as the hint, the word typed in the h-system, Nova vorto), its pairs the site's, every pair's board drawn whole fresh, helped, found and lost; nothing posted. Every typed answer of the course, in the h- and x-systems, comes through the input as itself.
  `TestSyncWithSite` and `TestSyncOffline` type their answers in the h-system, and the site judges them right.
- `internal/gamedata/words_test.go`: the word list, the pools of every level (Vortludo's, Konstrui's, Memoro's, Frazotrajno's, Vortpluvo's), Bonvorto's pairs (each word's shape, the division table beside the series), a definition never giving its word.
- `internal/handler/game_api_test.go`: the API (text, ETag / 304, languages, no cookie, 404), the course built
  once per content set, the items' places, a course built from items without series fields.
- `internal/handler/game_test.go`: the merge rule, the progress format and handler.
- The whole site run locally (the Datastore emulator, seeded) and played in headless Chromium.
- The game-playing tests need `GAMEVIBER=/path/to/gameviber` and are skipped without it.

## Next

1. Audio with the esperanto-voice voice, and with it a listening exercise.
2. Spaced review: words from finished lessons coming back as cards on the map.
3. The design's faces and dark mode, once GameViber can load a game's fonts.
