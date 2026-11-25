# Word pictures: the format

`vortbildoj.json` maps Esperanto words to pictures. It is embedded in the
binary (package `bildoj`), so a change ships with the next deploy; the content
in Datastore needs no change. Every vocab item, reading text and picture
exercise with a word in it shows that word's picture.

Each row:

```json
{"word": "hundo", "topic": "bestoj", "file": "vortoj/1f415.svg",
 "source": "OpenMoji 17.0.0: 1F415 dog", "author": "…", "license": "CC BY-SA 4.0",
 "group": "…", "exact": true}
```

| Field | Meaning |
|---|---|
| `word` | the word in its dictionary form: `hundo`, `ruĝa`, `manĝi`. One row per word. |
| `topic` | the A0 topic: `objektoj`, `bestoj`, `mangxajxoj`, `familio`, `koloroj`, `nombroj`, `agoj`, `korpo`, `naturo`. The A0 picture series `bildoj-<topic>` (seed/bildvortaro.json) takes its card picture from its topic's first row. |
| `file` | the file under `static/bildoj/`. Name it by the picture's code point or id, never by the word: a picture exercise must not give its answer away in the picture's URL (the test checks this). |
| `source`, `author`, `license` | where the picture comes from; shown on the credits page (`/fontoj`). |
| `exact` | optional: the word matches only as written. For words with no grammatical ending (numbers): without it `tri` would also match `tre`. |
| `group` | optional: pictures too alike to tell apart (man / boy, the bowls and cups). Two words of one group are never options of the same picture exercise. |

How a word finds its picture (`ForToken`, `ForVocab`):

1. the word as written (`du`, `hundo`);
2. else its root, when it carries a grammatical ending (`hundojn` → `hund` → `hundo`;
   `manĝis` → `manĝ` → `manĝi`); a word with no ending is never cut, so
   `vin` (you) does not show wine;
3. a vocab item's bare root (the Zagreb vocabulary keeps roots: `akv`) matches
   the root of a row. Affixes and correlatives (tags `sufikso`, `prefikso`,
   `tabelvorto`) get no picture.

To give a new word a picture: add the file under `static/bildoj/vortoj/`, add
its row, add its credit to `static/bildoj/ASSETS.md` if it comes from a new
source, and run `go test ./internal/bildoj/`.
