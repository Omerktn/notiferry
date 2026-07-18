package main

import (
	"unicode"
)

const telegramLimit = 4096

const splitBoundaryFloor = telegramLimit * 3 / 4

func splitPlain(s string) []string {
	r := []rune(s)
	if len(r) == 0 {
		return nil
	}
	var out []string
	for len(r) > telegramLimit {
		cut := telegramLimit
		for i := telegramLimit; i >= splitBoundaryFloor; i-- {
			if unicode.IsSpace(r[i-1]) {
				cut = i
				break
			}
		}
		// Keep the boundary whitespace in the preceding chunk. This preserves
		// the input while still making the next chunk start at a useful place.
		out = append(out, string(r[:cut]))
		r = r[cut:]
	}
	if len(r) > 0 {
		out = append(out, string(r))
	}
	return out
}
