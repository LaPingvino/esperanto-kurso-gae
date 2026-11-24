#!/bin/sh
# build.sh [OUT]: the game's data from the site's seed (cmd/export-game), then
# the web build (gameviber web) into OUT (default: game/web, git-ignored).
# Needs Go, the gameviber command (GAMEVIBER, or on PATH) and GameViber's
# source (GAMEVIBER_SRC: gameviber web builds the player there).
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
env -C "${GAMEVIBER_SRC:?set GAMEVIBER_SRC to GameViber's source folder}" "$gv" web "$here" "$out"
# The page around the game in the course's colours and language (GameViber's
# page is dark and English); the loading line is the site's own (eo.json).
sed -i \
  -e 's/<html lang="en">/<html lang="eo">/' \
  -e 's/<title>.*<\/title>/<title>Esperanto-kurso: la ludo<\/title>/' \
  -e 's/background:#111;color:#ddd/background:#F6F1E7;color:#222D36/' \
  -e 's/>loading…</>Ŝarĝas…</' \
  "$out/index.html"
