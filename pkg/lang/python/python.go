package python

// Parse tokenizes and parses source and returns the module AST. A nil
// error means the source is syntactically valid Python 3.12/3.13;
// lexical and grammar violations return *gast.LexError or
// *gast.ParseError.
func Parse(source string) (*File, error) {
	tokens, err := Tokenize(source)
	if err != nil {
		return nil, err
	}
	return newParser(tokens, source).parseModule()
}

// Validate reports whether source is syntactically valid Python.
func Validate(source string) error {
	_, err := Parse(source)
	return err
}

// splitStringLiteral splits a string literal's raw text into its raw
// flag and body (the text between the quotes).
func splitStringLiteral(text string) (raw bool, body string) {
	i := 0
	for ; i < len(text); i++ {
		switch text[i] {
		case 'r', 'R':
			raw = true
		case '"', '\'':
			quote := text[i : i+1]
			rest := text[i+1:]
			if len(rest) >= 6 && rest[:3] == quote+quote+quote {
				return raw, rest[3 : len(rest)-3]
			}
			return raw, rest[:len(rest)-1]
		}
	}
	return raw, ""
}

// decodeString decodes a string or bytes literal into its value.
func decodeString(text string) string {
	raw, body := splitStringLiteral(text)
	if raw {
		return body
	}
	return decodeEscapes(body)
}

// decodeFStringMiddle decodes one f-string literal chunk: doubled
// braces unescape and, outside raw strings, so do backslash escapes.
func decodeFStringMiddle(text string, raw bool) string {
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		b := text[i]
		switch {
		case b == '{' && i+1 < len(text) && text[i+1] == '{':
			out = append(out, '{')
			i++
		case b == '}' && i+1 < len(text) && text[i+1] == '}':
			out = append(out, '}')
			i++
		case b == '\\' && !raw && i+1 < len(text):
			i += decodeEscape(text, i, &out)
		default:
			out = append(out, b)
		}
	}
	return string(out)
}

// decodeEscapes decodes backslash escapes of a non-raw string body.
// Unknown escapes are kept verbatim, matching the lenient Python 3.12
// tokenizer; \N{...} names stay literal.
func decodeEscapes(body string) string {
	out := make([]byte, 0, len(body))
	for i := 0; i < len(body); i++ {
		if body[i] != '\\' || i+1 >= len(body) {
			out = append(out, body[i])
			continue
		}
		i += decodeEscape(body, i, &out)
	}
	return string(out)
}

// decodeEscape decodes the escape at body[start] into out and returns
// how many bytes past the backslash were consumed.
func decodeEscape(body string, start int, out *[]byte) int {
	b := body[start+1]
	switch b {
	case 'n':
		*out = append(*out, '\n')
	case 't':
		*out = append(*out, '\t')
	case 'r':
		*out = append(*out, '\r')
	case 'a':
		*out = append(*out, '\a')
	case 'b':
		*out = append(*out, '\b')
	case 'f':
		*out = append(*out, '\f')
	case 'v':
		*out = append(*out, '\v')
	case '\n':
	case '\r':
	case '\\', '\'', '"':
		*out = append(*out, b)
	case 'x', 'X':
		if value, width := hexAt(body, start+2, 2); width == 2 {
			*out = append(*out, byte(value&0xff))
			return width + 1
		}
		*out = append(*out, '\\', b)
	case 'u':
		if value, width := hexAt(body, start+2, 4); width == 4 {
			*out = appendRune(*out, runeOf(value))
			return width + 1
		}
		*out = append(*out, '\\', b)
	case 'U':
		if value, width := hexAt(body, start+2, 8); width == 8 {
			*out = appendRune(*out, runeOf(value))
			return width + 1
		}
		*out = append(*out, '\\', b)
	default:
		if '0' <= b && b <= '7' {
			value := int(b - '0')
			width := 1
			for width < 3 && start+1+width < len(body) && '0' <= body[start+1+width] && body[start+1+width] <= '7' {
				value = value*8 + int(body[start+1+width]-'0')
				width++
			}
			*out = append(*out, byte(value))
			return width
		}
		*out = append(*out, '\\', b)
	}
	return 1
}

// appendRune appends a decoded rune, mapping out-of-range values to
// the replacement character.
func appendRune(out []byte, r rune) []byte {
	if r > 0x10FFFF {
		r = 0xFFFD
	}
	return append(out, []byte(string(r))...)
}

// runeOf clamps a decoded escape value to the valid rune range.
func runeOf(value int) rune {
	if value > 0x10FFFF {
		return 0xFFFD
	}
	return rune(value) //nolint:gosec // guarded by the range check above
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
	if count < width {
		return 0, 0
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
