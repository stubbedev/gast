package js

import (
	"strconv"
	"strings"
	"unicode"
)

// Parse tokenizes and parses source and returns the file AST. A nil
// error means the source is syntactically valid JavaScript; lexical
// and grammar violations return *gast.LexError or *gast.ParseError.
func Parse(source string) (*File, error) {
	tokens, err := Tokenize(source)
	if err != nil {
		return nil, err
	}
	return newParser(tokens, source).parseFile()
}

// Validate reports whether source is syntactically valid JavaScript.
func Validate(source string) error {
	_, err := Parse(source)
	return err
}

// decodeString decodes the escapes of a string token into its value;
// unknown escapes decode to the escaped character itself, matching
// JavaScript.
func decodeString(text string) string {
	body := text[1 : len(text)-1]
	if !strings.ContainsRune(body, '\\') {
		return body
	}
	var builder strings.Builder
	builder.Grow(len(body))
	for index := 0; index < len(body); index++ {
		if body[index] != '\\' || index+1 >= len(body) {
			builder.WriteByte(body[index])
			continue
		}
		index++
		escape := body[index]
		if escape == '\n' || escape == '\r' {
			// line continuation contributes nothing
			if escape == '\r' && index+1 < len(body) && body[index+1] == '\n' {
				index++
			}
			continue
		}
		switch escape {
		case 'n':
			builder.WriteByte('\n')
		case 't':
			builder.WriteByte('\t')
		case 'r':
			builder.WriteByte('\r')
		case 'b':
			builder.WriteByte('\b')
		case 'f':
			builder.WriteByte('\f')
		case 'v':
			builder.WriteByte('\v')
		case '0':
			if index+1 < len(body) && body[index+1] >= '0' && body[index+1] <= '9' {
				// octal escape
				value, width := octalAt(body, index)
				builder.WriteRune(runeOf(value))
				index += width - 1
				continue
			}
			builder.WriteByte(0)
		case '1', '2', '3', '4', '5', '6', '7':
			value, width := octalAt(body, index)
			builder.WriteRune(runeOf(value))
			index += width - 1
		case 'x':
			if value, width := hexAt(body, index+1, 2); width == 2 {
				builder.WriteRune(runeOf(value))
				index += width
			} else {
				builder.WriteByte('x')
			}
		case 'u':
			if index+1 < len(body) && body[index+1] == '{' {
				end := strings.IndexByte(body[index+2:], '}')
				if end >= 0 {
					digits := body[index+2 : index+2+end]
					if value, width := hexAt(digits, 0, len(digits)); width == len(digits) && width > 0 && value <= 0x10FFFF {
						builder.WriteRune(runeOf(value))
						index += 2 + end
						continue
					}
				}
			} else if value, width := hexAt(body, index+1, 4); width == 4 {
				builder.WriteRune(runeOf(value))
				index += width
				continue
			}
			builder.WriteByte('u')
		default:
			builder.WriteByte(escape)
		}
	}
	return builder.String()
}

// octalAt parses up to three octal digits at body[start:], returning
// the value and how many digits were read.
func octalAt(body string, start int) (int, int) {
	value := 0
	count := 0
	for count < 3 && start+count < len(body) && body[start+count] >= '0' && body[start+count] <= '7' {
		value = value*8 + int(body[start+count]-'0')
		count++
	}
	return value, count
}

// hexAt parses up to width hex digits at body[start:], returning the
// value and how many digits were read.
// runeOf clamps a decoded escape value to the valid rune range; hex and
// \u{...} escapes admit more digits than a rune can hold.
func runeOf(value int) rune {
	if value > unicode.MaxRune {
		return unicode.ReplacementChar
	}
	return rune(value) //nolint:gosec // guarded by the MaxRune check above
}

func hexAt(body string, start, width int) (int, int) {
	value := 0
	count := 0
	for count < width && start+count < len(body) {
		digit, err := strconv.ParseUint(string(body[start+count]), 16, 32)
		if err != nil {
			break
		}
		value = value*16 + int(digit)
		count++
	}
	return value, count
}
