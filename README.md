# Esperanto-kurso

An interactive language-learning platform for Esperanto, running at **[esperanto-kurso.net](https://esperanto-kurso.net)**.

## What it is

A self-hosted, adaptive learning platform built with Go on Google App Engine. It features:

- **Adaptive difficulty** — Glicko-2 ratings calibrate both exercises and learners over time
- **No account required** — anonymous users get a magic link to preserve progress; optional passkey registration for cross-device sync
- **Multiple exercise types** — fill-in, multiple choice, reading, vocab flashcards (Anki-style), image, listening, phrasebook
- **Community contributions** — translations, comments, error reports, voting
- **Moderation queue** — auto-trust for established users, manual review otherwise
- **Series browser** — exercises grouped by CEFR level (A0–C2) and topic
- **Localized UI** — interface strings in 30+ languages; learners see exercises in Esperanto but definitions in their own language

## Tech stack

- **Backend**: Go 1.22+ on GAE Standard Environment
- **Database**: Google Cloud Datastore
- **Frontend**: Go templates + HTMX (no JS framework)
- **CSS**: Pico CSS with custom overrides
- **Auth**: magic links (crypto/rand) + WebAuthn passkeys

## Adapting for another language

This codebase is generic enough to run a course for any language. To adapt it:

1. Replace seed data in `seed/` with exercises for your target language
2. Update the locale strings in `internal/locale/` (the Esperanto UI strings are in `eo.json`; pick the closest existing language or add a new one)
3. Update `app.yaml` with your own GAE project ID
4. Build the game (below), then deploy to GAE with `gcloud app deploy` (or both at once: `make deploy`)

The only Esperanto-specific logic is in the seed data and locale strings — the engine itself is language-agnostic.

## Running locally

```bash
# Requires a Google Cloud project with Datastore enabled (or use the emulator)
export GOOGLE_CLOUD_PROJECT=your-project-id
go run main.go
```

With the Datastore emulator:

```bash
gcloud beta emulators datastore start &
$(gcloud beta emulators datastore env-init)
go run main.go
```

## The course as a game (/ludo/)

`game/` is the same course played as a game (a [GameViber](https://github.com/LaPingvino/gameviber) project,
built to WebAssembly). It loads the course from the site while it plays (`/ludo-api/`, built from the approved
content), so new content shows up without a rebuild, and it answers by the site's own rules. The site serves it
at `/ludo/` and links it from the menu (🎲) and from every exercise page it shows. Progress
syncs with the player's account in the background (see `game/DESIGN.md`).

```bash
make ludo     # builds into static/ludo (git-ignored, deployed); needs GAMEVIBER and GAMEVIBER_SRC
make deploy   # make ludo, then gcloud app deploy
```

## Spaced repetition (/ripetoj)

As on [angla.org](https://angla.org) (whose code it reuses): FSRS-6 scheduling in the browser, the state in
localStorage, the learner's Glicko rating on the server as before.

- **Cards** are the site's items, all but texts and videos (`/ripetoj-api/kartoj`, public and cached like
  `/ludo-api/`). New cards: 10 a day ("+5 novaj" at the end); in a series a card opens once one of the two
  before it has been seen; of the open cards the three nearest the learner's rating form the window.
- **Grades:** a right answer is Good, a wrong one Again; a self-judgement ne sciis / malfacile / bone /
  facile is Again / Hard / Good / Easy. Every verdict on the site grades its card, in a session or not.
- **Choices** of a multiple-choice item are shown in a fixed, fair shuffle per item (`miksi`, the same
  permutation as angla.org's), so authors need not vary the answer's place.
- **The server's trace** (`internal/ripetoj`): per item answered, the last day, the answer count and the
  last grade, 7 bytes an item (a 4-byte slug hash, 2 bytes day, 1 byte count and grade) on the User
  entity, written in the transaction that already records the streak; items never seen take nothing.
  A browser that lacks items the trace holds (a new device, a cleared browser) gets them from
  `/ripetoj/spuro` and rebuilds each by replaying a synthetic history through FSRS.
- **The page** `/ripetoj` starts a session (each card on its own exercise page, `?ripeto=1`), saves and
  restores the state as a file, and forgets it (here and the trace on the server). The header shows the
  count due today.

Code: `static/fsrs.js`, `static/ripetoj.js` (pure; tested by `node tools/test-fsrs.mjs` and
`tools/test-ripetoj.mjs`, which `go test` runs too), `static/eo-ripetoj.js` (the browser side),
`internal/handler/ripetoj.go`, `internal/ripetoj` (the trace, the shuffle).

## Pictures

Words, Zagreb lessons and exercise kinds have pictures, mapped through data
(`internal/bildoj/vortbildoj.json`; the format in `internal/bildoj/README.md`).
The A0 picture series are in `seed/bildvortaro.json`: seed it from the admin
page like the other seed files. Sources and licences: `static/bildoj/ASSETS.md`
and the site's `/fontoj` page.

## Dictionaries

A vocab card shows its meaning in the learner's meaning language (the 💬 picker, 101 languages) by a
fallback chain: the course's own definition → a community translation → [Reta Vortaro](https://reta-vortaro.de)'s
translation → English → ReVo's Esperanto definition. ReVo's text is an extract embedded in the app
(`internal/revo/revo.json.gz`): nothing is fetched from ReVo at run time. Every card also links to its ReVo
article and to [PIV](https://vortaro.net) (Esperanto only; linked, never copied).

Refresh the extract from ReVo's sources (git clones, no scraping):

```bash
make revo SRC=~/Projects/.cache/revo   # clones or pulls revo-fonto and voko-grundo there, rebuilds the extract
go test -v -run MeaningCoverage ./internal/handler   # cards with a meaning, per language
```

## License

MIT — see [LICENSE](LICENSE), with two exceptions that keep their own licences:

- the pictures under `static/bildoj/`: the word pictures are OpenMoji's (CC BY-SA 4.0, attribution on
  `/fontoj`), the lesson and kind icons Kenney's (CC0 1.0); see `static/bildoj/ASSETS.md`;
- `internal/revo/revo.json.gz`, an extract of Reta Vortaro (github.com/revuloj/revo-fonto), licensed under
  the GNU GPL 2.0 (`internal/revo/LICENSE-REVO`). The site credits ReVo wherever it shows its text.

Contributions welcome. If you build a course for another language on top of this, please let us know!
