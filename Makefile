# The course as a game (game/, a GameViber project) served at /ludo/.
#
#   make ludo      build it into static/ludo (needs Go, the gameviber command
#                  and GameViber's source: GAMEVIBER, GAMEVIBER_SRC)
#   make deploy    make ludo, then gcloud app deploy (Joop's credentials)
#   make revo      rebuild internal/revo/revo.json.gz from fresh clones of Reta
#                  Vortaro's sources (SRC: where to keep the clones)
#
# static/ludo is git-ignored but not gcloud-ignored: the deploy uploads it,
# and app.yaml serves it as static files (the .wasm as application/wasm).

GAMEVIBER ?= gameviber
GAMEVIBER_SRC ?= $(HOME)/Projects/gameviber-webextras

.PHONY: ludo deploy revo

ludo:
	GAMEVIBER=$(GAMEVIBER) GAMEVIBER_SRC=$(GAMEVIBER_SRC) game/build.sh $(CURDIR)/static/ludo
	@size=$$(stat -c %s static/ludo/game.wasm); \
	  if [ $$size -gt 33554432 ]; then echo "game.wasm is $$size bytes: over App Engine's 32 MB per file" >&2; exit 1; fi; \
	  echo "static/ludo: game.wasm $$size bytes (App Engine's limit: 32 MB per file)"

deploy: ludo
	gcloud app deploy

SRC ?= $(HOME)/Projects/.cache/revo

revo:
	@mkdir -p $(SRC)
	@for r in revo-fonto voko-grundo; do \
	  if [ -d $(SRC)/$$r/.git ]; then git -C $(SRC)/$$r pull -q --ff-only; \
	  else git clone -q --depth 1 https://github.com/revuloj/$$r.git $(SRC)/$$r; fi; done
	go run ./cmd/revo-import -fonto $(SRC)/revo-fonto -grundo $(SRC)/voko-grundo -o internal/revo/revo.json.gz
