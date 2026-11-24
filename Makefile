.PHONY: run check content audio sprites prune reset build audit voice-elevenlabs

# Everything you need after editing data/: validate, fill the gaps, drop the
# audio that no longer matches the content.
content: check audio sprites prune

prune:
	go run ./cmd/pokecontent prune

check:
	go run ./cmd/pokecontent check

# With .env.local (ElevenLabs key + voice), new lines get the ElevenLabs voice
# (Jeanne) — never Edge, which would mix two voices in the track.
audio:
	@if [ -f .env.local ]; then scripts/voice-elevenlabs.sh --go; else go run ./cmd/pokecontent audio; fi

sprites:
	go run ./cmd/pokecontent sprites

# NAME= renomme l'enfant et écrase la sauvegarde ; sans lui, elle garde son prénom.
run:
	go run ./cmd/pokelecture -name "$(NAME)"

build:
	go build -o bin/pokelecture ./cmd/pokelecture
	go build -o bin/pokecontent ./cmd/pokecontent

# Wipe the savegame and start the progression over.
reset:
	rm -f var/progress.json

# Clips sorted by length: a spelled-out syllable is twice as long as a spoken one.
audit:
	go run ./cmd/pokecontent audit

# Whole voice track with ElevenLabs (key + voice in .env.local, see the script).
voice-elevenlabs:
	scripts/voice-elevenlabs.sh
