package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/adelvecchio/pokelecture/internal/speech"
)

// ElevenLabs: paid, needs ELEVENLABS_API_KEY and a voice ID (-voice). The
// audio keys depend on the text only, so switching engine means `audio -force`
// then bumping CACHE in web/sw.js. See README, « La voix ».
var elevenModel = "eleven_v4"

// elevenRev is part of each clip's signature in voices.json: bump it after
// changing the settings below and the next run remakes (and pays for) them.
const elevenRev = 1

// errQuota stops the whole run: once the credits are gone, every further
// request would fail too.
var errQuota = errors.New("quota ElevenLabs épuisé")

const elevenURL = "https://api.elevenlabs.io/v1/text-to-speech/%s?output_format=mp3_44100_128"

// elevenSpeed maps a style to ElevenLabs' speed setting (0.7–1.2): sounds and
// syllables as slow as it allows, sentences close to natural. Eleven v4 has no
// speed setting and rejects it.
func elevenSpeed(style speech.Style) float64 {
	switch style {
	case speech.StyleSound, speech.StyleSyllable:
		return 0.7
	case speech.StyleWord:
		return 0.85
	default:
		return 0.95
	}
}

func elevenSpeak(text, voice string, style speech.Style, out string) error {
	key := os.Getenv("ELEVENLABS_API_KEY")
	if key == "" {
		return fmt.Errorf("ELEVENLABS_API_KEY n'est pas défini")
	}
	settings := map[string]any{"stability": 0.6, "similarity_boost": 0.8}
	if !strings.HasPrefix(elevenModel, "eleven_v4") {
		settings["speed"] = elevenSpeed(style)
	}
	body := map[string]any{"text": text, "model_id": elevenModel, "voice_settings": settings}
	// A bare chunk must be said as written: normalization is what turns "dre"
	// into "Docteure" and "min" into "minute".
	if style == speech.StyleSound || style == speech.StyleSyllable {
		body["apply_text_normalization"] = "off"
	}
	// Forcing French matters for bare syllables ("ta", "pi"), which a model
	// left to detect the language may read in English. Only the v2.5+ models
	// accept the field; multilingual_v2 rejects it.
	if !strings.Contains(elevenModel, "multilingual_v2") {
		body["language_code"] = "fr"
	}
	payload, _ := json.Marshal(body)
	client := &http.Client{Timeout: 60 * time.Second}

	var lastErr error
	for attempt := 1; attempt <= 4; attempt++ {
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf(elevenURL, voice), bytes.NewReader(payload))
		req.Header.Set("xi-api-key", key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "audio/mpeg")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			switch {
			case resp.StatusCode == http.StatusOK:
				return os.WriteFile(out, b, 0o644)
			case bytes.Contains(b, []byte("quota_exceeded")) || resp.StatusCode == http.StatusPaymentRequired:
				return fmt.Errorf("%w (%s): %s", errQuota, resp.Status, bytes.TrimSpace(b))
			case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
				lastErr = fmt.Errorf("elevenlabs: %s", resp.Status)
			default: // a bad key, voice or quota will not fix itself: fail fast
				return fmt.Errorf("elevenlabs: %s: %s", resp.Status, bytes.TrimSpace(b))
			}
		}
		time.Sleep(time.Duration(attempt) * 3 * time.Second)
	}
	return lastErr
}
