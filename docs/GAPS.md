# The course's gaps (2026-11-25)

Measured on the seed files in `seed/` (the export from March 2026: 2,295 items),
not on the live Datastore, which may have grown since. Once the builder API is
deployed, `scripts/builder.sh read` gives the live outline; re-check this page
against it.

The level of an item is its Glicko rating (`content.go cefrToRatingRange`):
A0 < 1000 ≤ A1 < 1200 ≤ A2 < 1400 ≤ B1 < 1600 ≤ B2 < 1800 ≤ C1 < 2000 ≤ C2. The
series browser files a series under the level of its first item.

**Esperanto first, then English.** esperanto-kurso.net is now step one of a
path: after it comes the English course for Esperanto speakers (angla.org).
That course explains English in Esperanto, so this course must teach every
grammar term and grammar point it relies on. The English course's agent will
write a PREREQS.md; until it arrives, the priority here is the core terminology
and grammar an Esperanto-language grammar of English needs: verb tenses and
moods, participles, the accusative, correlatives, word-building affixes, and
the grammar terms themselves.

## What there is

| Level | Readings | Fill-in | Multiple choice | Vocab | Video | Phrasebook |
|---|---:|---:|---:|---:|---:|---:|
| A0 | 6 | – | – | 272 | 4 | – |
| A1 | 10 | 46 | 5 | 783 | 30 | – |
| A2 | 9 | 40 | 7 | 14 | 32 | 2 |
| B1 | 48 | 40 | – | 693 | – | – |
| B2 | 20 | – | – | 34 | – | – |
| C1 | 12 | – | – | 188 | – | – |
| C2 | **0** | **0** | **0** | **0** | – | – |

Series: the Zagreb method (12 lessons, with 8–16 fill-ins each), vocab lists
(roots, correlatives, prepositions, affixes, numbers, days, months, colours),
the Fundamenta Ekzercaro (31 readings), PMEG excerpts (9), literature
excerpts (8, B1–C1), Mazi and Pasporto videos, two articles (C1).

## The gaps

1. **No C2 at all**, and nothing interactive above B1. B2, C1 and C2 have
   only readings and vocab cards: there is not one exercise with an answer to
   check. 108 interactive items in all (A1–B1), against 1,454 vocab cards.
2. **Grammar terminology is missing**, and the English course needs it most.
   Of the ~50 core terms (substantivo, verbo, akuzativo, participo,
   infinitivo, subjekto, objekto, tenso, modo, pasivo, komparativo,
   subfrazo…) only five are in the vocab: sufikso, radiko, finaĵo, silabo,
   ununombro.
3. **Grammar points are explained (PMEG readings) but never practised:**
   - correlatives: 90 vocab cards, no exercise that uses one in a sentence;
   - participles: one PMEG reading (C1), no exercise; nothing on -anta/-inta/
     -onta vs -ata/-ita/-ota, the e-participle (manĝinte), participle nouns
     (lernanto, arestito);
   - the accusative of direction (sur la tablon, hejmen, Parizon): one
     multiple choice (gramatiko-mc-05) touches it;
   - verb tenses and moods, and relative tense in reported speech ("li diris,
     ke li venos"), which an English course for Esperanto speakers leans on;
   - affixes: 54 vocab cards for the affixes, no exercise in forming words.
4. **Nuance and register (C1–C2):** si/lia, ja, de/da, ol/kiel, ju…des,
   kvankam/malgraŭ, aspect with ek- and -ad-, poetic elision — not taught.
5. **Exercise kinds:** no listening items (the type exists, no content), no
   image items, only 2 phrasebook items (and those ask in English).
6. **Vocab without meanings:** 335 vocab cards have no definition in any
   language (auto-made from literature readings: Zamenhof's speeches 97,
   Privat 87, PMEG 78, Grimm/Kabe 46, others 27); a learner can't answer them.
7. **Topics:** no everyday-situation series past A2 (health, work, travel,
   official letters, discussion and argument), no Esperanto culture/history
   series beyond a few readings.

## What the builder has prepared (content/builder/, 190 items, all pending)

Load after the deploy with `scripts/builder.sh load content/builder`; it all
goes in pending, nothing is live until published (see docs/BUILDER.md).

| Level | Readings | Fill-in | Multiple choice | Vocab |
|---|---:|---:|---:|---:|
| A2 | – | 8 | 3 | 18 |
| B1 | – | 36 | 16 | 36 |
| B2 | – | 8 | 2 | – |
| C1 | 2 | 2 | 19 | – |
| C2 | 5 | 8 | 27 | – |

- **C2: five authentic texts with 35 exercises** (comprehension, irony and
  register, rare roots, affixes in context, poetic style):
  - Zamenhof, *Antaŭparolo* of the Fundamento (1905), excerpt;
  - Zamenhof, *Letero pri la deveno de Esperanto* (to Borovko, c. 1895),
    excerpt;
  - Zamenhof, *Esenco kaj estonteco de la ideo de lingvo internacia* (1900),
    the opening;
  - Gogol, *La Revizoro*, transl. Zamenhof (1907), from act one;
  - Mickiewicz, *Sinjoro Tadeo*, transl. Antoni Grabowski (1918), the first
    22 lines.
- **C1:** the Bulonja Deklaracio (1905, excerpt) and Zamenhof's *La Espero*,
  with 11 exercises; 10 nuance questions (ja, ol, si/lia, de/da, kvankam,
  ek-, ne povas ne, tro…por, poetic elision).
- **Grammar series (A2–B2), self-written:** correlatives (14), participles
  (10), the accusative of direction (8), tenses and moods with relative tense
  (11), word-building affixes (18), grammar terms in use (12).
- **Vocab:** 54 grammar terms with en/nl/de/fr/es meanings.

### Sources and licences

All from Esperanto Wikisource (eo.wikisource.org), fetched 2026-11-25, as
public domain:

| Text | Author / translator (died) | Page | Revision |
|---|---|---|---|
| Fundamento de Esperanto, Antaŭparolo (1905) | Zamenhof (1917) | Fundamento_de_Esperanto/Antaŭparolo | 65022 |
| Letero pri la deveno de Esperanto | Zamenhof (1917); translated from Russian, translator not named in Lingvaj Respondoj (1925) | Lingvaj_Respondoj_(1925)/Letero_pri_la_deveno_de_Esperanto | 65766 |
| Esenco kaj estonteco… (1900), in Fundamenta Krestomatio (1903) | Zamenhof (1917) | Fundamenta_Krestomatio_(1903)/Esenco_kaj_estonteco_de_la_ideo_de_lingvo_internacia | 65280 |
| La Revizoro, akto unua (1907) | Gogol (1852), transl. Zamenhof (1917) | La_Revizoro/Akto_unua | 89777 |
| Sinjoro Tadeo, Libro I (1918) | Mickiewicz (1855), transl. Antoni Grabowski (1921) | Sinjoro_Tadeo/Libro_I | 58617 |
| Bulonja deklaracio (1905) | Zamenhof and the 1st World Congress | Bulonja_deklaracio | 68280 |
| La Espero, in Fundamenta Krestomatio (1903) | Zamenhof (1917) | Fundamenta_Krestomatio_(1903)/La_espero | 68501 |

(Revision: of the wrapper page; the text itself is transcluded from the scan's Paĝo: pages.)

Each reading's `content.source_url` and `content.license` hold its page and
status; `content.note` says what was cut or changed. Changes: typography
modernised (quotation marks, no space before : ; ! ?), line breaks of the
scan joined, footnote and verse numbers removed, two OCR errors in *Esenco*
fixed (samlempuloj → samtempuloj, "iras. kiel" → "iras, kiel"). The Revizoro
has a one-line introduction and the Letter a note on the two Russian shop
signs, both marked as the site's own. (Grabowski died in 1921, not 1930 as the
brief had it; PD either way. Hamlet, transl. Zamenhof, would have been a good
C2 text but Wikisource has act III untranscribed.)

## Copyright: already on the site, worth a look

These are on the live site now, from before the builder:

- *Vivo de Zamenhof*, **Edmond Privat** (died 1962): in copyright until
  2032, plus 87 vocab cards made from it.
- *La sep kapridoj*, Grimm, transl. **Kabe** (Kazimierz Bein, died 1959): in
  copyright until 2029.
- *1984*, Orwell (d. 1950), transl. **Donald Broadribb** (d. 2012), and *La
  eta princo*, Saint-Exupéry (d. 1944), in a modern translation: both in
  copyright.
- *Pinokjo* (Collodi) and *Reĝo Maĉjo Unua* (Korczak, d. 1942): it depends on
  the translator. Check who translated them.
- PMEG excerpts: PMEG (Bertilo Wennergren) has its own terms of use. Check
  them.

## drop?

Candidates only: nothing is deleted yet. A redesign pass comes after the
current branches land.

- **The 335 vocab cards without definitions** — they cannot be answered; give them meanings or drop them.
- **The Privat and Kabe excerpts and their auto-vocab** — in copyright (above); replace with Zamenhof/Grabowski PD texts.
- **The duplicate series `literatura` / `literaturo`** — one is enough (8 + 8 readings, overlapping).
- **Duplicate tags** (`grava-esprimo`/`grava_esprimo`, `tago-en-la-semajno`/`tago_en_la_semajno`) — one spelling.
- **The phrasebook items that ask in English** ("How do you say…") — they assume English, which this course comes *before*; ask in the learner's language or in Esperanto.
- **The legacy `question`/`answer` keys on readings** (17 readings, mostly empty) — dead data.
- **The Honorlisto (hall of fame)** — a rating leaderboard serves neither the course nor the path to English.
- **Video items without exercises** (Mazi 24, Pasporto 32, music 8) — they are passive; keep them only if they get comprehension questions, and check that the YouTube embeds still play.
- **`voc-auto-*` cards from literature excerpts in general** — even with meanings, a card per rare word of one text is noise next to the core vocabulary.
