package main

import "testing"

func TestSanitizeDesc(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain description", "Dell Inc. DELL U3419W 5HJB6T2", "Dell Inc. DELL U3419W 5HJB6T2"},
		{"trims surrounding whitespace", "  Dell Inc. DELL U3419W  ", "Dell Inc. DELL U3419W"},
		{"rejects embedded comma", "Apple Computer Inc., Apple Studio Display", ""},
		{"rejects embedded double quote", `Dell "pro" U3419W`, ""},
		{"rejects newline", "Dell\nU3419W", ""},
		{"rejects control character", "Dell\x01U3419W", ""},
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeDesc(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeDesc(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
