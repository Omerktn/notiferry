package main

import (
	"strings"
	"testing"
)

func TestSplitPlainUnicodeAndBoundaries(t *testing.T) {
	s := strings.Repeat("界 ", 3000)
	parts := splitPlain(s)
	if len(parts) < 2 {
		t.Fatal("expected chunks")
	}
	for _, p := range parts {
		if len([]rune(p)) > telegramLimit {
			t.Fatal("chunk too large")
		}
	}
	if strings.Join(parts, "") != s {
		t.Fatal("text was not preserved")
	}
}

func TestSplitPlainDoesNotChooseDistantWhitespace(t *testing.T) {
	s := strings.Repeat("a", 100) + " " + strings.Repeat("界", telegramLimit)
	parts := splitPlain(s)
	if len(parts) != 2 || len([]rune(parts[0])) != telegramLimit {
		t.Fatalf("parts=%d first length=%d", len(parts), len([]rune(parts[0])))
	}
	if strings.Join(parts, "") != s {
		t.Fatal("text was not preserved")
	}
}
