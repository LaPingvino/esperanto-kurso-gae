#!/usr/bin/env bash
# builder.sh — the builder's side of internal/handler/builder.go: read the
# course, add to it, publish and unpublish the builder's own items.
#   scripts/builder.sh read [QUERY]            # outline + counts; QUERY e.g. 'level=C2&origin=builder'
#   scripts/builder.sh get SLUG                # one item, whole
#   scripts/builder.sh add FILE.json [...]     # an item or an array; new items go in pending
#   scripts/builder.sh load DIR                # add every content/builder/*.json-like file in DIR, in name order
#   scripts/builder.sh publish SLUG...|--series S|--all     # → approved (live)
#   scripts/builder.sh unpublish SLUG...|--series S|--all   # → pending (off the site again: the undo)
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
*) echo "usage: $0 read [QUERY] | get SLUG | add FILE... | load DIR | publish|unpublish SLUG...|--series S|--all" >&2; exit 2 ;;
esac
