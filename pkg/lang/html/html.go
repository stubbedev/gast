package html

import "strings"

// Parse tokenizes and parses source and returns the document AST. A
// nil error means the source is valid for this package's WHATWG
// subset; violations return *gast.LexError or *gast.ParseError. When
// the source opens no html element, the html/head/body wrappers are
// synthesized without spans.
func Parse(source string) (*File, error) {
	tokens, err := Tokenize(source)
	if err != nil {
		return nil, err
	}
	return newParser(tokens, source).parseFile()
}

// Validate reports whether source is valid HTML for this package.
func Validate(source string) error {
	_, err := Parse(source)
	return err
}

// parseDoctypeText extracts the name and public/system identifiers
// from a doctype token's raw text. Internal structure is lenient:
// only the position of a doctype is validated.
func parseDoctypeText(text string) (name, publicID, systemID string) {
	fields := splitQuoted(strings.TrimSuffix(text[2:], gt))
	if len(fields) < 2 {
		return "", "", ""
	}
	name = fields[1]
	if len(fields) >= 3 && strings.EqualFold(fields[2], "PUBLIC") {
		if len(fields) >= 4 {
			publicID = fields[3]
		}
		if len(fields) >= 5 {
			systemID = fields[4]
		}
		return name, publicID, systemID
	}
	if len(fields) >= 4 && strings.EqualFold(fields[2], "SYSTEM") {
		systemID = fields[3]
	}
	return name, publicID, systemID
}

// splitQuoted splits fields on whitespace, keeping quoted runs (with
// their quotes) as single fields.
func splitQuoted(text string) []string {
	var fields []string
	start := -1
	var quote byte
	for i := range len(text) {
		b := text[i]
		switch {
		case quote != 0:
			if b == quote {
				quote = 0
			}
		case b == '"' || b == '\'':
			quote = b
			if start < 0 {
				start = i
			}
		case isWhitespace(b):
			if start >= 0 {
				fields = append(fields, text[start:i])
				start = -1
			}
		default:
			if start < 0 {
				start = i
			}
		}
	}
	if start >= 0 {
		fields = append(fields, text[start:])
	}
	return fields
}
