package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// manifest records, for every clip in web/audio, which voice produced it
// ("elevenlabs:<voice>:<model>:<rev>"). It is committed with the audio: a
// paid clip is paid once, and a rerun — on any machine — skips every clip the
// same voice already made.
type manifest struct {
	mu   sync.Mutex
	path string
	by   map[string]string // clip key → voice signature
}

func loadManifest(audioDir string) *manifest {
	m := &manifest{path: filepath.Join(audioDir, "voices.json"), by: map[string]string{}}
	if b, err := os.ReadFile(m.path); err == nil {
		_ = json.Unmarshal(b, &m.by)
	}
	return m
}

func (m *manifest) get(key string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.by[key]
}

func (m *manifest) set(key, sig string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.by[key] = sig
}

// keep drops every entry whose clip is no longer part of the track.
func (m *manifest) keep(keys map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.by {
		if !keys[k] {
			delete(m.by, k)
		}
	}
}

// save writes the manifest with sorted keys, so its git diff stays readable.
func (m *manifest) save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.by))
	for k := range m.by {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := make([]struct{ k, v string }, len(keys))
	for i, k := range keys {
		ordered[i] = struct{ k, v string }{k, m.by[k]}
	}
	var buf []byte
	buf = append(buf, "{\n"...)
	for i, kv := range ordered {
		kb, _ := json.Marshal(kv.k)
		vb, _ := json.Marshal(kv.v)
		buf = append(buf, "  "...)
		buf = append(buf, kb...)
		buf = append(buf, ": "...)
		buf = append(buf, vb...)
		if i < len(ordered)-1 {
			buf = append(buf, ',')
		}
		buf = append(buf, '\n')
	}
	buf = append(buf, "}\n"...)
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}
