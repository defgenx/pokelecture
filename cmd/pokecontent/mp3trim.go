package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// trimMP3 cuts the trailing silence off an MP3 without re-encoding: an MP3 is
// a run of ~26 ms frames, so dropping the frames after the voice keeps the
// file exactly as the voice service made it, only shorter. ElevenLabs leaves
// ~0.35 s of silence after every clip, heard as lag between two syllables.
// The leading Xing/Info frame is dropped too: its frame count would claim the
// old length, and the karaoke reads the clip's duration.
func trimMP3(path, scratchWAV string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	end, err := voiceEnd(path, scratchWAV)
	if err != nil {
		return err
	}
	keepUntil := end + float64(tailMs)/1000 + 0.03 // + one frame of decoder delay

	start := 0
	if len(b) > 10 && string(b[:3]) == "ID3" { // ID3v2: syncsafe size
		start = 10 + (int(b[6])<<21 | int(b[7])<<14 | int(b[8])<<7 | int(b[9]))
	}
	var out bytes.Buffer
	out.Write(b[:start])
	t := 0.0
	first := true
	for off := start; off+4 <= len(b); {
		n, dur, ok := mp3Frame(b[off:])
		if !ok || off+n > len(b) {
			break
		}
		frame := b[off : off+n]
		isInfo := first && (bytes.Contains(frame, []byte("Xing")) || bytes.Contains(frame, []byte("Info")))
		first = false
		if !isInfo {
			if t > keepUntil {
				break
			}
			out.Write(frame)
			t += dur
		}
		off += n
	}
	if out.Len() <= start {
		return errors.New("mp3: aucune trame lisible")
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

// voiceEnd decodes the clip (afconvert, macOS) and returns when the voice
// stops, in seconds.
func voiceEnd(path, wav string) (float64, error) {
	if b, err := exec.Command("afconvert", "-f", "WAVE", "-d", "LEI16", "-c", "1", path, wav).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("afconvert: %v: %s", err, b)
	}
	defer os.Remove(wav)
	b, err := os.ReadFile(wav)
	if err != nil {
		return 0, err
	}
	var rate int
	var data []byte
	for off := 12; off+8 <= len(b); {
		id, size := string(b[off:off+4]), int(binary.LittleEndian.Uint32(b[off+4:off+8]))
		body := b[off+8 : min(off+8+size, len(b))]
		switch id {
		case "fmt ":
			rate = int(binary.LittleEndian.Uint32(body[4:8]))
		case "data":
			data = body
		}
		off += 8 + size + size%2
	}
	if rate == 0 || data == nil {
		return 0, errors.New("wav illisible")
	}
	last := 0
	for i := 0; i+1 < len(data); i += 2 {
		if v := int16(binary.LittleEndian.Uint16(data[i:])); v > silence || v < -silence {
			last = i / 2
		}
	}
	return float64(last) / float64(rate), nil
}

var (
	mp3Bitrates = [2][16]int{ // kbps: MPEG-1 layer III, MPEG-2/2.5 layer III
		{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0},
		{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
	}
	mp3Rates = [4][3]int{{11025, 12000, 8000}, {0, 0, 0}, {22050, 24000, 16000}, {44100, 48000, 32000}}
)

// mp3Frame reads one layer III frame header: its length in bytes and its
// duration in seconds.
func mp3Frame(h []byte) (int, float64, bool) {
	if h[0] != 0xFF || h[1]&0xE0 != 0xE0 || (h[1]>>1)&3 != 1 { // sync + layer III
		return 0, 0, false
	}
	ver := (h[1] >> 3) & 3 // 3 = MPEG-1, 2 = MPEG-2, 0 = MPEG-2.5
	br, sr, pad := int(h[2]>>4), int((h[2]>>2)&3), int((h[2]>>1)&1)
	if ver == 1 || br == 0 || br == 15 || sr == 3 {
		return 0, 0, false
	}
	rate := mp3Rates[ver][sr]
	if ver == 3 {
		bitrate := mp3Bitrates[0][br] * 1000
		return 144*bitrate/rate + pad, 1152 / float64(rate), true
	}
	bitrate := mp3Bitrates[1][br] * 1000
	return 72*bitrate/rate + pad, 576 / float64(rate), true
}
