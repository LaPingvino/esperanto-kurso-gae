# The course as a game (game/, a GameViber project) served at /ludo/.
#
#   make ludo      build it into static/ludo (needs Go, the gameviber command
#                  and GameViber's source: GAMEVIBER, GAMEVIBER_SRC)
#   make deploy    make ludo, then gcloud app deploy (Joop's credentials)
#
# static/ludo is git-ignored but not gcloud-ignored: the deploy uploads it,
# and app.yaml serves it as static files (the .wasm as application/wasm).

GAMEVIBER ?= gameviber
GAMEVIBER_SRC ?= $(HOME)/Projects/gameviber-webextras

.PHONY: ludo deploy

ludo:
	GAMEVIBER=$(GAMEVIBER) GAMEVIBER_SRC=$(GAMEVIBER_SRC) game/build.sh $(CURDIR)/static/ludo
	@size=$$(stat -c %s static/ludo/game.wasm); \
	  if [ $$size -gt 33554432 ]; then echo "game.wasm is $$size bytes: over App Engine's 32 MB per file" >&2; exit 1; fi; \
	  echo "static/ludo: game.wasm $$size bytes (App Engine's limit: 32 MB per file)"

deploy: ludo
	gcloud app deploy
