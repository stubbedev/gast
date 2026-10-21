package codepoint

import (
	"testing"
	"unicode"
)

func TestHex(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input   string
		width   int
		value   rune
		count   int
		inRange bool
	}{
		{"41", 2, 'A', 2, true},
		{"4g", 2, 4, 1, true},
		{"", 4, 0, 0, true},
		{"zz", 2, 0, 0, true},
		{"00e9rest", 4, 'é', 4, true},
		{"aBcD", 4, 0xABCD, 4, true},
		{"1F600", 5, 0x1F600, 5, true},
		{"10FFFF", 6, unicode.MaxRune, 6, true},
		{"110000", 6, unicode.ReplacementChar, 6, false},
		{"FFFFFFFF", 8, unicode.ReplacementChar, 8, false},
		{"10000000000000041", 17, unicode.ReplacementChar, 17, false},
		{"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF", 32, unicode.ReplacementChar, 32, false},
		{"123456", 3, 0x123, 3, true},
	}
	for _, tc := range tests {
		value, count, inRange := Hex(tc.input, tc.width)
		if value != tc.value || count != tc.count || inRange != tc.inRange {
			t.Errorf("Hex(%q, %d) = (%U, %d, %t), want (%U, %d, %t)",
				tc.input, tc.width, value, count, inRange, tc.value, tc.count, tc.inRange)
		}
	}
}
