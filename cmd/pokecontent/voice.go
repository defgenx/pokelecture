package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adelvecchio/pokelecture/internal/speech"
)

// The whole voice track is ElevenLabs, one voice (Jeanne by default, see
// .env.local), saved as the MP3 the service returns — trimmed of its trailing
// silence, never re-encoded. README, « La voix ».

// signature names the voice that made a clip, for voices.json. Bump elevenRev
// to deliberately remake every clip after a settings change.
func signature(voice string) string {
	return fmt.Sprintf("elevenlabs:%s:%s:r%d", voice, elevenModel, elevenRev)
}

// synth speaks one text into web/audio/<key>.mp3.
func synth(j job, voice, tmpDir, outDir string, worker int) error {
	raw := filepath.Join(tmpDir, fmt.Sprintf("w%d.mp3", worker))
	wav := filepath.Join(tmpDir, fmt.Sprintf("w%d.wav", worker))
	if err := elevenSpeak(j.text, voice, j.style, raw); err != nil {
		return err
	}
	if err := trimMP3(raw, wav); err != nil {
		return err
	}
	return os.Rename(raw, filepath.Join(outDir, speech.Key(j.text, j.style)+speech.Ext))
}

const (
	silence = 300 // |sample| at or below this is silence (≈ −40 dBFS)
	tailMs  = 150 // voice kept after the last loud sample
)
