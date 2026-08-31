package api

import (
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/speech"
)

// A recording made by a parent beats the synthetic voice every time — a
// five-year-old learns sounds far better from a voice he knows, and no TTS
// pronounces invented Pokémon names correctly.
//
// Recordings land in web/audio/recorded/<key>.<ext> and are served under the
// existing /audio/<key>.m4a URL, so nothing else in the app needs to know they
// exist. The extension follows what MediaRecorder produced: .m4a for AAC
// (recent Chrome, Safari — plays everywhere including iPads), .webm for Opus
// (older Chrome — iPads refuse it and the game falls back to the synthetic
// voice there, so AAC is preferred at capture time in parent.js).

const recordedDir = "recorded"

// recordingExts, best first: an .m4a shadows a leftover .webm of the same line.
var recordingExts = []struct{ ext, ctype string }{
	{".m4a", "audio/mp4"},
	{".webm", "audio/webm"},
}

// maxRecording caps an upload; a spoken sentence is a few tens of kilobytes.
const maxRecording = 8 << 20

type textView struct {
	Text     string `json:"text"`
	Style    string `json:"style"`
	Key      string `json:"key"`
	URL      string `json:"url"`
	Group    string `json:"group"`
	Recorded bool   `json:"recorded"`
}

// handleTexts lists every line the voice has to say, so the recorder page can
// show what is synthetic and what is already in the parent's own voice.
func (s *Server) handleTexts(w http.ResponseWriter, r *http.Request) {
	items := s.cur.SpeechTexts()
	out := make([]textView, 0, len(items))
	for _, it := range items {
		key := speech.Key(it.Text, it.Style)
		out = append(out, textView{
			Text:     it.Text,
			Style:    string(it.Style),
			Key:      key,
			URL:      speech.URL(it.Text, it.Style),
			Group:    groupOf(it),
			Recorded: s.hasRecording(key),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// groupOf labels a line for the recorder UI. Sounds come first because they are
// the ones a TTS gets worst and the ones worth recording first.
func groupOf(it curriculum.SpeechItem) string {
	switch it.Style {
	case speech.StyleSound:
		return "sons"
	case speech.StyleSyllable:
		return "syllabes"
	case speech.StyleWord:
		return "mots"
	default:
		return "phrases"
	}
}

// recordingFile finds the stored recording for a key, if any, and its MIME type.
func (s *Server) recordingFile(key string) (path, ctype string, ok bool) {
	for _, e := range recordingExts {
		p := filepath.Join(s.webDir, "audio", recordedDir, key+e.ext)
		if st, err := os.Stat(p); err == nil && st.Size() > 0 {
			return p, e.ctype, true
		}
	}
	return "", "", false
}

func (s *Server) hasRecording(key string) bool {
	_, _, ok := s.recordingFile(key)
	return ok
}

// safeKey rejects anything that could escape the audio directory.
func safeKey(key string) (string, bool) {
	if key == "" || len(key) > 80 || key != path.Base(key) {
		return "", false
	}
	for _, r := range key {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
		if !ok {
			return "", false
		}
	}
	return key, true
}

func (s *Server) handleRecordPut(w http.ResponseWriter, r *http.Request) {
	key, ok := safeKey(r.PathValue("key"))
	if !ok {
		http.Error(w, "clé invalide", http.StatusBadRequest)
		return
	}

	dir := filepath.Join(s.webDir, "audio", recordedDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	body := http.MaxBytesReader(w, r.Body, maxRecording)
	data, err := io.ReadAll(body)
	if err != nil {
		http.Error(w, "enregistrement trop gros ou illisible", http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "enregistrement vide", http.StatusBadRequest)
		return
	}

	// Extension by what the browser recorded; a new take replaces any previous
	// take in the *other* container too, or the stale one would keep shadowing.
	ext := ".webm"
	if ct := r.Header.Get("Content-Type"); strings.HasPrefix(ct, "audio/mp4") || strings.HasPrefix(ct, "audio/aac") {
		ext = ".m4a"
	}
	dst := filepath.Join(dir, key+ext)
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.Rename(tmp, dst); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, e := range recordingExts {
		if e.ext != ext {
			_ = os.Remove(filepath.Join(dir, key+e.ext))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "bytes": len(data), "recorded": true})
}

func (s *Server) handleRecordDelete(w http.ResponseWriter, r *http.Request) {
	key, ok := safeKey(r.PathValue("key"))
	if !ok {
		http.Error(w, "clé invalide", http.StatusBadRequest)
		return
	}
	for _, e := range recordingExts {
		p := filepath.Join(s.webDir, "audio", recordedDir, key+e.ext)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "recorded": false})
}

// handleAudio serves a recording in place of the generated file when one exists.
// The game always asks for /audio/<key>.m4a and never has to care which it gets.
func (s *Server) handleAudio(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	stem := strings.TrimSuffix(name, filepath.Ext(name))

	if key, ok := safeKey(stem); ok {
		if p, ctype, found := s.recordingFile(key); found {
			w.Header().Set("Content-Type", ctype)
			// Recordings change while the parent is iterating, so never cache.
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, p)
			return
		}
	}

	generated := filepath.Join(s.webDir, "audio", filepath.Base(name))
	if _, err := os.Stat(generated); err != nil {
		http.NotFound(w, r)
		return
	}
	// Explicit, because the runtime image carries no /etc/mime.types: Go knows
	// nothing about .m4a there and would sniff the container down to video/mp4.
	w.Header().Set("Content-Type", "audio/mp4")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	http.ServeFile(w, r, generated)
}
