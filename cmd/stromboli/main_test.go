package main

import (
	"log/slog"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	cases := []struct {
		in    string
		want  slog.Level
		ok    bool
	}{
		{"", slog.LevelInfo, true},
		{"info", slog.LevelInfo, true},
		{"INFO", slog.LevelInfo, true},
		{"  Info  ", slog.LevelInfo, true},
		{"debug", slog.LevelDebug, true},
		{"warn", slog.LevelWarn, true},
		{"warning", slog.LevelWarn, true},
		{"error", slog.LevelError, true},
		{"trace", slog.LevelInfo, false},
		{"verbose", slog.LevelInfo, false},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := parseLogLevel(tc.in)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("parseLogLevel(%q) = (%v, %v), want (%v, %v)",
					tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}
