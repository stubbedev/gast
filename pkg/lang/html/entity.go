package html

import (
	"strconv"
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

// windows1252 maps the C1 range 0x80-0x9F used in numeric character
// references to their windows-1252 code points per the spec's numeric
// character reference replacement table. Bytes with no windows-1252
// assignment map to themselves.
var windows1252 = [32]rune{
	0x20AC, 0x0081, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x008D, 0x017D, 0x008F,
	0x0090, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x009D, 0x017E, 0x0178,
}

// decodeEntities decodes character references in text per the WHATWG
// tokenizer rules: named references from namedEntities (semicolon form
// preferred, then the longest legacy match), decimal &#123; and hex
// &#x1F600; forms, and the spec's replacement rules for out-of-range
// numeric references (C1 range remapped through windows-1252; zero,
// surrogates, and values above U+10FFFF replaced with U+FFFD). A
// semicolon is optional for both forms; inAttribute applies the legacy
// attribute rule, where a named reference without a trailing semicolon
// followed by "=" or an ASCII alphanumeric stays literal. Text with no
// "&" is returned unchanged. A numeric reference with no digits
// ("&#;" or "&#x;") is a LexError positioned at its "&" via spanStart.
func decodeEntities(text string, spanStart int, inAttribute bool) (string, error) {
	if strings.IndexByte(text, '&') < 0 {
		return text, nil
	}
	var b strings.Builder
	b.Grow(len(text))
	i := 0
	for i < len(text) {
		if text[i] != '&' {
			j := strings.IndexByte(text[i:], '&')
			if j < 0 {
				b.WriteString(text[i:])
				return b.String(), nil
			}
			b.WriteString(text[i : i+j])
			i += j
		}
		consumed, value, ok, err := decodeReference(text[i:], spanStart+i, inAttribute)
		if err != nil {
			return "", err
		}
		if !ok {
			b.WriteByte('&')
			i++
			continue
		}
		b.WriteString(value)
		i += consumed
	}
	return b.String(), nil
}

// decodeReference decodes one character reference at the start of s,
// which begins with "&". It returns how many bytes were consumed, the
// decoded text, whether a reference was decoded at all, and a LexError
// for a numeric reference with no digits.
func decodeReference(s string, at int, inAttribute bool) (int, string, bool, error) {
	if len(s) < 2 {
		return 0, "", false, nil
	}
	if s[1] == '#' {
		return decodeNumericReference(s, at)
	}
	name := 1
	for name < len(s) && isASCIIAlphanumeric(s[name]) {
		name++
	}
	name-- // name is now the length of the candidate run after "&"
	if name == 0 {
		return 0, "", false, nil
	}
	hasSemi := 1+name < len(s) && s[1+name] == ';'
	// Longest match first; the semicolon form is only possible when it
	// spans the whole candidate run.
	for length := name; length > 0; length-- {
		value, ok := lookupEntity(s[1:1+length], length == name && hasSemi)
		if !ok {
			continue
		}
		consumed := 1 + length
		withSemi := length == name && hasSemi
		if withSemi {
			consumed++
		} else if inAttribute && guardedByLegacyRule(s[consumed:]) {
			// Legacy attribute rule: a semicolon-less reference
			// followed by "=" or an alphanumeric stays literal.
			return 0, "", false, nil
		}
		return consumed, value, true, nil
	}
	return 0, "", false, nil
}

// decodeNumericReference decodes "&#123;", "&#x1F600;", and their
// semicolon-less forms at the start of s. Digits are required: a
// reference with none is reported as a LexError at at.
func decodeNumericReference(s string, at int) (int, string, bool, error) {
	digits := 2
	base := 10
	if len(s) > 2 && (s[2] == 'x' || s[2] == 'X') {
		digits = 3
		base = 16
	}
	end := digits
	for end < len(s) && isDigit(s[end], base) {
		end++
	}
	if end == digits {
		return 0, "", false, &gast.LexError{
			Message: "numeric character reference with no digits",
			Pos:     gast.Span{Start: at, End: at + digits},
		}
	}
	consumed := end
	code, err := strconv.ParseUint(s[digits:end], base, 64)
	if err != nil {
		code = 0x110000 // beyond any valid code point; replaced below
	}
	if end < len(s) && s[end] == ';' {
		consumed++
	}
	return consumed, replacementCodePoint(code), true, nil
}

// lookupEntity looks up a named reference, preferring the semicolon
// form when one is possible.
func lookupEntity(name string, semi bool) (string, bool) {
	if semi {
		if value, ok := namedEntities[name+";"]; ok {
			return value, true
		}
	}
	value, ok := namedEntities[name]
	return value, ok
}

// guardedByLegacyRule reports whether rest begins with "=" or an ASCII
// alphanumeric byte.
func guardedByLegacyRule(rest string) bool {
	if rest == "" {
		return false
	}
	b := rest[0]
	return b == '=' || isASCIIAlphanumeric(b)
}

// replacementCodePoint renders a decoded numeric reference, applying
// the spec's replacement table for the C1 range and substituting
// U+FFFD for null, surrogates, and out-of-range values.
func replacementCodePoint(code uint64) string {
	switch {
	case code == 0, code >= 0xD800 && code <= 0xDFFF, code > 0x10FFFF:
		return "\uFFFD"
	case code >= 0x80 && code <= 0x9F:
		return string(windows1252[code-0x80])
	default:
		return string(rune(code))
	}
}

func isASCIIAlphanumeric(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9')
}

func isDigit(b byte, base int) bool {
	if '0' <= b && b <= '9' {
		return true
	}
	if base == 16 {
		return ('a' <= b && b <= 'f') || ('A' <= b && b <= 'F')
	}
	return false
}
