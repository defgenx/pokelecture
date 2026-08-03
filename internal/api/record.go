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
// Recordings land in web/audio/recorded/<key>.webm (what MediaRecorder produces
// in Chrome) and are served under the existing /audio/<key>.m4a URL, so nothing
// else in the app needs to know they exist.

const recordedDir = "recorded"

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

func (s *Server) recordingPath(key string) string {
	return filepath.Join(s.webDir, "audio", recordedDir, key+".webm")
}

func (s *Server) hasRecording(key string) bool {
	st, err := os.Stat(s.recordingPath(key))
	return err == nil && st.Size() > 0
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

	dst := s.recordingPath(key)
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.Rename(tmp, dst); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "bytes": len(data), "recorded": true})
}

func (s *Server) handleRecordDelete(w http.ResponseWriter, r *http.Request) {
	key, ok := safeKey(r.PathValue("key"))
	if !ok {
		http.Error(w, "clé invalide", http.StatusBadRequest)
		return
	}
	if err := os.Remove(s.recordingPath(key)); err != nil && !os.IsNotExist(err) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "recorded": false})
}

// handleAudio serves a recording in place of the generated file when one exists.
// The game always asks for /audio/<key>.m4a and never has to care which it gets.
func (s *Server) handleAudio(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	stem := strings.TrimSuffix(name, filepath.Ext(name))

	if key, ok := safeKey(stem); ok {
		if p := s.recordingPath(key); s.hasRecording(key) {
			w.Header().Set("Content-Type", "audio/webm")
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
