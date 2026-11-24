.PHONY: run test check content audio sprites prune reset build audit voice-plan

# Everything you need after editing data/: validate, fill the gaps, drop the
# audio that no longer matches the content.
content: check audio sprites prune

prune:
	go run ./cmd/pokecontent prune

check:
	go run ./cmd/pokecontent check

# The voice track is ElevenLabs only (key + voice in .env.local): new lines
# are generated with the same voice, and nothing already made is paid again.
audio:
	scripts/voice-elevenlabs.sh --go

sprites:
	go run ./cmd/pokecontent sprites

# NAME= renomme l'enfant et écrase la sauvegarde ; sans lui, elle garde son prénom.
# .env.local (git-ignored) carries the secrets: ElevenLabs, and the Google
# sign-in for the parent pages (GOOGLE_CLIENT_ID, …, see README).
run:
	@set -a; if [ -f .env.local ]; then . ./.env.local; fi; set +a; go run -tags dev ./cmd/pokelecture -local-admin -name "$(NAME)"

# Both builds: the production one (no sign-in bypass) and the dev one.
test:
	go vet ./... && go vet -tags dev ./...
	go test ./... && go test -tags dev ./internal/api

build:
	go build -o bin/pokelecture ./cmd/pokelecture
	go build -o bin/pokecontent ./cmd/pokecontent

# Wipe the savegame and start the progression over.
reset:
	rm -f var/progress.json

# Clips sorted by length: a spelled-out syllable is twice as long as a spoken one.
audit:
	go run ./cmd/pokecontent audit

# The plan only: what is left to generate and what it costs. Sends nothing.
voice-plan:
	scripts/voice-elevenlabs.sh
