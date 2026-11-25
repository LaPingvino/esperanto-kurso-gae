#!/usr/bin/env bash
# builder.sh — the builder's side of internal/handler/builder.go: read the
# course, add to it, publish and unpublish the builder's own items.
#   scripts/builder.sh read [QUERY]            # outline + counts; QUERY e.g. 'level=C2&origin=builder'
#   scripts/builder.sh get SLUG                # one item, whole
#   scripts/builder.sh add FILE.json [...]     # an item or an array; new items go in pending
#   scripts/builder.sh load DIR                # add every content/builder/*.json-like file in DIR, in name order
#   scripts/builder.sh publish SLUG...|--series S|--all     # → approved (live)
#   scripts/builder.sh unpublish SLUG...|--series S|--all   # → pending (off the site again: the undo)
#   scripts/builder.sh legvortoj               # the plan for the old voc-auto-* word cards (changes nothing)
#   scripts/builder.sh legvortoj-apply FP      # carry out the plan with that fingerprint → a run id
#   scripts/builder.sh legvortoj-undo RUN      # put back what that run changed
# The key stays in its file (never on the command line): $EK_BUILDER_KEY_FILE,
# default ~/.config/esperanto-kurso-builder/key. Site: $EK_SITE (default https://esperanto-kurso.net).
set -euo pipefail
site=${EK_SITE:-https://esperanto-kurso.net}
keyfile=${EK_BUILDER_KEY_FILE:-$HOME/.config/esperanto-kurso-builder/key}
hdr=$(mktemp); trap 'rm -f "$hdr"' EXIT; chmod 600 "$hdr"
printf 'Authorization: Bearer %s\n' "$(tr -d '\n' < "$keyfile")" > "$hdr"
ua=(-H "User-Agent: esperanto-kurso-builder/1 (+$site)")
post() { curl -sS --fail-with-body "${ua[@]}" -H @"$hdr" -H "Content-Type: application/json" --data-binary "$2" "$site/builder-api/$1"; echo; }
target() { # the target of publish/unpublish as JSON
  case "${1:-}" in
  --all) echo '{"all": true}' ;;
  --series) jq -n --arg s "${2:?a series slug}" '{series: $s}' ;;
  "") echo "name slugs, --series S or --all" >&2; exit 2 ;;
  *) jq -n '{slugs: $ARGS.positional}' --args "$@" ;;
  esac
}
cmd=${1:-read}; shift || true
case "$cmd" in
read) curl -sS --fail-with-body "${ua[@]}" -H @"$hdr" "$site/builder-api/content${1:+?$1}"; echo ;;
get) curl -sS --fail-with-body "${ua[@]}" -H @"$hdr" "$site/builder-api/content/${1:?a slug}"; echo ;;
add) for f in "${@:?a file}"; do echo "== $f"; post content @"$f"; done ;;
load) for f in "${1:?a directory}"/*.json; do echo "== $f"; post content @"$f"; done ;;
publish) post publish "$(target "$@")" ;;
unpublish) post unpublish "$(target "$@")" ;;
legvortoj) curl -sS --fail-with-body "${ua[@]}" -H @"$hdr" "$site/builder-api/legvortoj" | jq '{fingerprint, counts, actions: (.actions | group_by(.kind) | map({key: .[0].kind, value: map(.slug + (if .into then " → " + .into else "" end))}) | from_entries)}' ;;
legvortoj-apply) # in batches: the same run, the next fingerprint, until nothing remains
  fp=${1:?the fingerprint from: builder.sh legvortoj}; run=""
  while :; do
    res=$(post legvortoj/apply "$(jq -n --arg f "$fp" --arg r "$run" '{fingerprint: $f} + (if $r == "" then {} else {run: $r} end)')")
    echo "$res"
    run=$(jq -r .run <<<"$res"); left=$(jq -r '.remaining // 0' <<<"$res"); fp=$(jq -r '.fingerprint // empty' <<<"$res")
    [ "$left" -gt 0 ] && [ -n "$fp" ] || break
  done
  echo "run $run (undo: $0 legvortoj-undo $run)" ;;
legvortoj-undo) post legvortoj/undo "$(jq -n --arg r "${1:?a run id, lv-…}" '{run: $r}')" ;;
*) echo "usage: $0 read [QUERY] | get SLUG | add FILE... | load DIR | publish|unpublish SLUG...|--series S|--all | legvortoj | legvortoj-apply FP | legvortoj-undo RUN" >&2; exit 2 ;;
esac
