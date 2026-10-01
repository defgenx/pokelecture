package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/adelvecchio/pokelecture/internal/speech"
)

// An engine turns one text into a raw audio file that afconvert can read.
type engine struct {
	name  string
	bin   string
	ext   string
	voice string // default voice
	hint  string // how to install bin
	speak func(text, voice string, style speech.Style, out string) error
}

var engines = map[string]engine{
	// Microsoft Edge's neural voices: free, no account, far more natural than
	// `say`. Needs the network while generating, never at play time.
	"edge": {
		name: "edge", bin: "edge-tts", ext: ".mp3", voice: "fr-FR-DeniseNeural",
		hint: "installe-le : `uv tool install edge-tts` (ou `pipx install edge-tts`)",
		speak: func(text, voice string, style speech.Style, out string) error {
			// --text= and --rate= glued: a value starting with "-" would be read as a flag.
			return retry("edge-tts", "--voice", voice, "--rate="+edgeRate(style), "--text="+text, "--write-media", out)
		},
	},
	"say": {
		name: "say", bin: "say", ext: ".aiff", voice: "Thomas",
		hint: "la voix `say` nécessite macOS",
		speak: func(text, voice string, style speech.Style, out string) error {
			cmd := exec.Command("say", "-v", voice, "-r", fmt.Sprint(style.Rate()), "-o", out, text)
			if b, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("say: %v: %s", err, b)
			}
			return nil
		},
	},
}

// edgeRate expresses the style's `say` words-per-minute as the relative rate
// edge-tts takes, against `say`'s ~175 wpm default.
func edgeRate(style speech.Style) string {
	return fmt.Sprintf("%+d%%", style.Rate()*100/175-100)
}

// retry runs an edge-tts call up to four times: the free service drops the odd
// request when several workers hit it at once.
func retry(name string, args ...string) error {
	var err error
	for attempt := 1; attempt <= 4; attempt++ {
		var b []byte
		if b, err = exec.Command(name, args...).CombinedOutput(); err == nil {
			return nil
		}
		err = fmt.Errorf("edge-tts: %v: %s", err, bytes.TrimSpace(b))
		time.Sleep(time.Duration(attempt) * 2 * time.Second)
	}
	return err
}

// synth speaks one text, trims the silence around it, then transcodes to
// AAC/m4a, which Chrome on Android and Safari on iPad both play.
func synth(j job, eng engine, voice, tmpDir, outDir string, worker int) error {
	raw := filepath.Join(tmpDir, fmt.Sprintf("w%d%s", worker, eng.ext))
	wav := filepath.Join(tmpDir, fmt.Sprintf("w%d.wav", worker))
	out := filepath.Join(outDir, speech.Key(j.text, j.style)+".m4a")

	if err := eng.speak(j.text, voice, j.style, raw); err != nil {
		return err
	}
	if b, err := exec.Command("afconvert", "-f", "WAVE", "-d", "LEI16", raw, wav).CombinedOutput(); err != nil {
		return fmt.Errorf("afconvert: %v: %s", err, b)
	}
	if err := trimWAV(wav); err != nil {
		return err
	}
	if b, err := exec.Command("afconvert", "-f", "m4af", "-d", "aac", "-b", "48000", wav, out).CombinedOutput(); err != nil {
		return fmt.Errorf("afconvert: %v: %s", err, b)
	}
	return nil
}

// trimWAV cuts the silence before and after the voice in a 16-bit PCM WAV, in
// place. Edge pads every clip with ~1 s of trailing silence, which the child
// hears as lag between two syllables.
func trimWAV(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return errors.New("trim: pas un fichier WAVE")
	}
	var fmtChunk, data []byte
	for off := 12; off+8 <= len(b); {
		id, size := string(b[off:off+4]), int(binary.LittleEndian.Uint32(b[off+4:off+8]))
		body := b[off+8 : min(off+8+size, len(b))]
		switch id {
		case "fmt ":
			fmtChunk = body
		case "data":
			data = body
		}
		off += 8 + size + size%2
	}
	if len(fmtChunk) < 16 || data == nil {
		return errors.New("trim: WAVE sans fmt/data")
	}
	channels := int(binary.LittleEndian.Uint16(fmtChunk[2:4]))
	rate := int(binary.LittleEndian.Uint32(fmtChunk[4:8]))
	if bits := binary.LittleEndian.Uint16(fmtChunk[14:16]); bits != 16 || channels < 1 {
		return fmt.Errorf("trim: %d bits / %d canaux non gérés", bits, channels)
	}

	frame := 2 * channels
	frames := len(data) / frame
	loud := func(i int) bool {
		for c := 0; c < channels; c++ {
			v := int16(binary.LittleEndian.Uint16(data[i*frame+2*c:]))
			if v > silence || v < -silence {
				return true
			}
		}
		return false
	}
	first, last := 0, frames-1
	for first < frames && !loud(first) {
		first++
	}
	for last > first && !loud(last) {
		last--
	}
	if first >= frames {
		return nil // all silence: leave it, the check would rather hear nothing than crash
	}
	start := max(0, first-rate*leadMs/1000)
	end := min(frames, last+1+rate*tailMs/1000)
	pcm := data[start*frame : end*frame]

	var w bytes.Buffer
	w.WriteString("RIFF")
	binary.Write(&w, binary.LittleEndian, uint32(4+8+16+8+len(pcm)))
	w.WriteString("WAVEfmt ")
	binary.Write(&w, binary.LittleEndian, uint32(16))
	w.Write(fmtChunk[:16])
	w.WriteString("data")
	binary.Write(&w, binary.LittleEndian, uint32(len(pcm)))
	w.Write(pcm)
	return os.WriteFile(path, w.Bytes(), 0o644)
}

const (
	silence = 300 // |sample| at or below this is silence (≈ −40 dBFS)
	leadMs  = 40
	tailMs  = 150
)
