#!/usr/bin/env bash
# Regenerate the whole voice track with ElevenLabs. See README, « La voix ».
#
#   scripts/voice-elevenlabs.sh --voices          # list the account's voices
#   scripts/voice-elevenlabs.sh --try "Pikachu"   # hear one line (costs its characters)
#   scripts/voice-elevenlabs.sh                   # the plan: what is left, what it costs — sends nothing
#   scripts/voice-elevenlabs.sh --go              # spend this month's credits on the most useful lines
#
# Every clip is paid once: web/audio/voices.json (committed) records which voice
# made it, and a rerun skips them. Sounds and syllables go first, sentences
# last; rerun --go next month to continue where the credits ran out.
#
# Settings come from the environment or from .env.local (git-ignored):
#   ELEVENLABS_API_KEY=…   ELEVENLABS_VOICE_ID=…   ELEVENLABS_MODEL=eleven_v4
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ -f .env.local ]]; then set -a; source .env.local; set +a; fi
: "${ELEVENLABS_MODEL:=eleven_v4}"

die() { echo "✗ $*" >&2; exit 1; }
[[ -n "${ELEVENLABS_API_KEY:-}" ]] || die "ELEVENLABS_API_KEY manquant : ajoute-le dans .env.local (voir l'en-tête de ce script)"

api() { curl -fsS -H "xi-api-key: $ELEVENLABS_API_KEY" "$@"; }

case "${1:-}" in
--voices)
	api https://api.elevenlabs.io/v1/voices | python3 -c '
import json, sys
for v in json.load(sys.stdin)["voices"]:
    l = v.get("labels") or {}
    print(f"{v[\"voice_id\"]}  {v[\"name\"]:<24} {l.get(\"language\", \"\")} {l.get(\"accent\", \"\")} {l.get(\"gender\", \"\")}")'
	exit 0 ;;
--try)
	[[ -n "${2:-}" ]] || die "usage : $0 --try \"texte\""
	[[ -n "${ELEVENLABS_VOICE_ID:-}" ]] || die "ELEVENLABS_VOICE_ID manquant (lance --voices)"
	out=$(mktemp -t pokevoice).mp3
	body=$(python3 -c 'import json,sys; print(json.dumps({"text": sys.argv[1], "model_id": sys.argv[2], "language_code": "fr", "voice_settings": {"stability": 0.6, "similarity_boost": 0.8}}))' "$2" "$ELEVENLABS_MODEL")
	api -X POST -H "Content-Type: application/json" -d "$body" -o "$out" \
		"https://api.elevenlabs.io/v1/text-to-speech/$ELEVENLABS_VOICE_ID?output_format=mp3_44100_128"
	afplay "$out"
	exit 0 ;;
--go) go=1 ;;
"") go=0 ;;
*) die "option inconnue : $1" ;;
esac

[[ -n "${ELEVENLABS_VOICE_ID:-}" ]] || die "ELEVENLABS_VOICE_ID manquant (lance --voices)"
go run ./cmd/pokecontent check >/dev/null || die "contenu invalide : corrige data/ d'abord (go run ./cmd/pokecontent check)"

left=$(api https://api.elevenlabs.io/v1/user/subscription | python3 -c '
import json, sys; s = json.load(sys.stdin); print(max(0, s["character_limit"] - s["character_count"]))')
echo "  Crédits ElevenLabs restants ce mois-ci : $left"
args=(-force -engine elevenlabs -voice "$ELEVENLABS_VOICE_ID" -model "$ELEVENLABS_MODEL" -budget "$left")

if [[ $go -eq 0 ]]; then
	go run ./cmd/pokecontent audio -dry-run "${args[@]}"
	echo "  Rien n'a été envoyé. Pour générer : $0 --go"
	exit 0
fi
[[ $left -gt 0 ]] || die "plus de crédits ce mois-ci"

before=$(shasum web/audio/voices.json 2>/dev/null || true)
go run ./cmd/pokecontent audio "${args[@]}" -workers 3
go run ./cmd/pokecontent prune

# Same URLs, new voice: the tablet must drop its cached clips — only when
# something was actually remade.
if [[ "$(shasum web/audio/voices.json)" != "$before" ]]; then
	n=$(sed -nE "s/.*const CACHE = 'pokelecture-v([0-9]+)'.*/\1/p" web/sw.js)
	sed -E -i '' "s/const CACHE = 'pokelecture-v[0-9]+'/const CACHE = 'pokelecture-v$((n + 1))'/" web/sw.js
	echo "  ✓ $(grep -o "pokelecture-v[0-9]*" web/sw.js) : la tablette rechargera la voix"
fi

echo
echo "  Clips les plus longs (un clip épelé dure deux fois plus : écoute ceux du haut) :"
go run ./cmd/pokecontent audit | head -15
echo
echo "  Puis : git add web/audio web/sw.js && git commit -m 'voix ElevenLabs'"
