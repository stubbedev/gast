package php

import (
	"strings"
	"unicode"
)

// Parse tokenizes and parses source and returns the file AST. A nil
// error means the source is syntactically valid PHP; lexical and
// grammar violations return *gast.LexError or *gast.ParseError.
func Parse(source string) (*File, error) {
	tokens, err := Tokenize(source)
	if err != nil {
		return nil, err
	}
	return newParser(tokens, source).parseFile()
}

// Validate reports whether source is syntactically valid PHP.
func Validate(source string) error {
	_, err := Parse(source)
	return err
}

// decodeString decodes the escapes of a string token into its value.
// Interpolation sequences in double-quoted text and heredocs are left
// as-is; heredoc bodies are returned without the <<< header and the
// closing identifier.
func decodeString(text string) string {
	switch {
	case strings.HasPrefix(text, "'"):
		return decodeSingleQuoted(text)
	case strings.HasPrefix(text, "<"):
		return decodeDoubleQuoted(heredocBody(text))
	default:
		return decodeDoubleQuoted(text)
	}
}

func heredocBody(text string) string {
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		if end := strings.LastIndexByte(text[index:], '\n'); end > 0 {
			return text[index+1 : index+end]
		}
		return text[index+1:]
	}
	return ""
}

func decodeSingleQuoted(text string) string {
	var builder strings.Builder
	body := text[1 : len(text)-1]
	for index := 0; index < len(body); index++ {
		if body[index] == '\\' && index+1 < len(body) && (body[index+1] == '\\' || body[index+1] == '\'') {
			builder.WriteByte(body[index+1])
			index++
			continue
		}
		builder.WriteByte(body[index])
	}
	return builder.String()
}

func decodeDoubleQuoted(text string) string {
	var builder strings.Builder
	body := text
	if len(body) >= 2 && (body[0] == '"' || body[0] == '`') {
		body = body[1 : len(body)-1]
	}
	for index := 0; index < len(body); index++ {
		if body[index] != '\\' || index+1 >= len(body) {
			builder.WriteByte(body[index])
			continue
		}
		index++
		switch body[index] {
		case 'n':
			builder.WriteByte('\n')
		case 't':
			builder.WriteByte('\t')
		case 'r':
			builder.WriteByte('\r')
		case 'v':
			builder.WriteByte('\v')
		case 'f':
			builder.WriteByte('\f')
		case 'e':
			builder.WriteByte(0x1b)
		case 'x', 'X':
			if value, width := hexAt(body, index+1, 2); width > 0 {
				builder.WriteRune(runeOf(value))
				index += width
			} else {
				builder.WriteByte('\\')
				builder.WriteByte(body[index])
			}
		case 'u':
			if index+1 < len(body) && body[index+1] == '{' {
				end := strings.IndexByte(body[index+2:], '}')
				if end >= 0 {
					digits := body[index+2 : index+2+end]
					if value, width := hexAt(digits, 0, len(digits)); width == len(digits) && width > 0 {
						builder.WriteRune(runeOf(value))
						index += 2 + end
						continue
					}
				}
			}
			builder.WriteByte('\\')
			builder.WriteByte('u')
		default:
			if body[index] >= '0' && body[index] <= '7' {
				value := int(body[index] - '0')
				width := 1
				for width < 3 && index+width < len(body) && body[index+width] >= '0' && body[index+width] <= '7' {
					value = value*8 + int(body[index+width]-'0')
					width++
				}
				builder.WriteRune(runeOf(value & 0xff))
				index += width - 1
				continue
			}
			builder.WriteByte('\\')
			builder.WriteByte(body[index])
		}
	}
	return builder.String()
}

// runeOf clamps a decoded escape value to the valid rune range;
// \u{...} admits more hex digits than a rune can hold.
func runeOf(value int) rune {
	if value > unicode.MaxRune {
		return unicode.ReplacementChar
	}
	return rune(value) //nolint:gosec // guarded by the MaxRune check above
}

// hexAt parses up to width hex digits at body[start:], returning the
// value and how many digits were read.
func hexAt(body string, start, width int) (int, int) {
	value := 0
	count := 0
	for count < width && start+count < len(body) {
		digit := hexValue(body[start+count])
		if digit < 0 {
			break
		}
		value = value*16 + digit
		count++
	}
	return value, count
}

func hexValue(b byte) int {
	switch {
	case '0' <= b && b <= '9':
		return int(b - '0')
	case 'a' <= b && b <= 'f':
		return int(b-'a') + 10
	case 'A' <= b && b <= 'F':
		return int(b-'A') + 10
	}
	return -1
}
