package main

import "testing"

func TestCircumflex(t *testing.T) {
	for in, want := range map[string]string{
		"o": "ô", "do": "dô", "Do": "Dô", "pi": "pî", "lu": "lû", "ta": "tâ", "pha": "phâ",
		"ou": "ou", "eau": "eau", "oi": "oi", "qui": "qui", "beu": "beu", "on": "on",
		"tac": "tac", "Mor": "Mor", "é": "é", "zà": "zà", "": "",
	} {
		if got := circumflex(in); got != want {
			t.Errorf("circumflex(%q) = %q, want %q", in, got, want)
		}
	}
}
