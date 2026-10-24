package main

import (
	"log/slog"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{input: "debug", want: slog.LevelDebug},
		{input: "info", want: slog.LevelInfo},
		{input: "warn", want: slog.LevelWarn},
		{input: "warning", want: slog.LevelWarn},
		{input: "error", want: slog.LevelError},
		{input: " INFO ", want: slog.LevelInfo},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := parseLogLevel(test.input)
			if err != nil {
				t.Fatalf("parseLogLevel(%q) returned error: %v", test.input, err)
			}
			if got != test.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestParseLogLevelRejectsUnknownValue(t *testing.T) {
	if _, err := parseLogLevel("verbose"); err == nil {
		t.Fatal("parseLogLevel accepted an unsupported level")
	}
}
