#!/bin/sh
# build.sh [OUT]: the game's data from the site's seed (cmd/export-game), then
# the web build (gameviber web) into OUT (default: game/web, git-ignored).
# Needs Go and the gameviber command (GAMEVIBER, or on PATH).
set -e
here=$(dirname "$(readlink -f "$0")")
root=$(dirname "$here")
gv=${GAMEVIBER:-gameviber}
out=${1:-$here/web}
go -C "$root" run ./cmd/export-game \
  -seed "$root/seed/ekzercoj-zagr-restructured.json" \
  -locale "$root/internal/locale/eo.json" \
  -out "$here/scripts/course_data.gml"
"$gv" check "$here"
"$gv" web "$here" "$out"
