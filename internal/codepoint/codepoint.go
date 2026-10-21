package codepoint

import "unicode"

func Hex(s string, width int) (value rune, count int, inRange bool) {
	inRange = true
	for count < width && count < len(s) {
		digit, ok := hexDigit(s[count])
		if !ok {
			break
		}
		if inRange {
			value = value<<4 | digit
			inRange = value <= unicode.MaxRune
		}
		count++
	}
	if !inRange {
		return unicode.ReplacementChar, count, false
	}
	return value, count, true
}

func hexDigit(b byte) (rune, bool) {
	switch {
	case '0' <= b && b <= '9':
		return rune(b - '0'), true
	case 'a' <= b && b <= 'f':
		return rune(b-'a') + 10, true
	case 'A' <= b && b <= 'F':
		return rune(b-'A') + 10, true
	}
	return 0, false
}
