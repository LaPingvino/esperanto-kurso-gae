# The builder API

Adding to the course from the VPS, by key — modelled on WEquilibrium's
builder API. Code: `internal/handler/builder.go`; client: `scripts/builder.sh`.

## The key

- The key is on the VPS in `~/.config/esperanto-kurso-builder/key` (mode 600).
  It is never in the repo, on a command line or in a log.
- The site knows only its SHA-256: `EK_BUILDER_KEY_SHA256` in `app.yaml`.
  Empty or missing: the API is off (every call 403).
- A new key: write a new random key to that file, put
  `tr -d '\n' < ~/.config/esperanto-kurso-builder/key | sha256sum` in
  `app.yaml`, deploy.
- No key or a wrong one: 401; after 20 wrong keys from one address within
  15 minutes: 429.

## Endpoints

| | |
|---|---|
| `GET /builder-api/content[?status=&type=&series=&level=&origin=builder]` | the outline (slug, type, level, status, series, title) and counts by level, type, status, series |
| `GET /builder-api/content/{slug}` | one item, whole |
| `POST /builder-api/content` | one item or an array (≤ 500): add, or update the builder's own |
| `POST /builder-api/publish` | `{"slugs": […]}`, `{"series": "…"}` or `{"all": true}` → approved |
| `POST /builder-api/unpublish` | the same → pending |
| `POST /builder-api/review` | the same as unpublish ("mark for review") |

An item: `{"slug", "type", "content", "tags", "source", "series_slug",
"series_order", "series_label", "series_parent", "level" | "rating", "trusted"}`.
`level` (A0…C2) sets a new item's starting rating at the middle of that level;
an update keeps the rating the item has learned from attempts. A batch is
checked whole first, with the site's answer rules (multiple-choice options
differ, `correct_index` is among them; a fill-in has one answer per `___`);
one bad item and nothing is written.

## The approval flow, and undo

- Everything the builder writes has `AuthorID: "builder"`. The admin's
  single-item editor keeps that mark; the admin's *series* editor
  (`/admin/serio/…/redakti`) rewrites a series' items as the editing admin's
  (and resets their ratings), so after a save there they are no longer the
  builder's: the builder can't change or unpublish them any more.
- New items go in **pending**: on the admin's list (`/admin/enhavo?status=pending`)
  and the dashboard count, in no public list, series, search or
  recommendation, until an admin approves them there, or the builder
  publishes them (`trusted: true` on add, or `/publish`).
- The builder changes **only its own items**. Adding over a slug that isn't
  the builder's is refused (400), and publishing or unpublishing one is
  refused (409). So it can never overwrite seed, admin or community content,
  or publish someone else's pending item.
- `unpublish --all` also sends back builder items that an admin approved by
  hand; approve them again afterwards if that wasn't meant.
- **Undo in one call:** `scripts/builder.sh unpublish --all` puts every
  builder item back to pending. Use `--series S` or slugs to undo some.
- The limit: like every unapproved item on the site, a pending item still
  opens by its direct URL `/ekzerco/{slug}`, though nothing links to it.

## The script

```sh
scripts/builder.sh read 'level=C2&origin=builder'
scripts/builder.sh get c2-letero-deveno
scripts/builder.sh load content/builder      # the prepared content, all pending
scripts/builder.sh publish c2-letero-deveno           # a reading (its series holds all C2 readings)
scripts/builder.sh publish --series c2-letero-ekz     # and its exercises
scripts/builder.sh unpublish --all           # the undo
```

`EK_SITE` (default https://esperanto-kurso.net) and `EK_BUILDER_KEY_FILE`
override the site and key file. The script needs `curl` and `jq`.

## Prepared content

`content/builder/*.json` is in the API's format, made by
`content/builder/make.py` from the Wikisource texts (`content/builder/sources/fetch.sh DIR`
fetches and converts them; then `python content/builder/make.py DIR content/builder`) (sources and licences in
`docs/GAPS.md`). `TestBuilderContentFiles` runs the files through the API's own
checks and makes sure no slug collides with the seed.
