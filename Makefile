.PHONY: run check content audio sprites prune reset build

# Everything you need after editing data/: validate, fill the gaps, drop the
# audio that no longer matches the content.
content: check audio sprites prune

prune:
	go run ./cmd/pokecontent prune

check:
	go run ./cmd/pokecontent check

audio:
	go run ./cmd/pokecontent audio -voice Thomas

sprites:
	go run ./cmd/pokecontent sprites

run:
	go run ./cmd/pokelecture -name $(or $(NAME),Dresseur)

build:
	go build -o bin/pokelecture ./cmd/pokelecture
	go build -o bin/pokecontent ./cmd/pokecontent

# Wipe the savegame and start the progression over.
reset:
	rm -f var/progress.json
