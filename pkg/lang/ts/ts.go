package ts

// Parse tokenizes and parses TypeScript source and returns the file
// AST. A nil error means the source is syntactically valid
// TypeScript; lexical and grammar violations return *gast.LexError or
// *gast.ParseError. No type checking is performed.
func Parse(source string) (*File, error) {
	tokens, err := Tokenize(source)
	if err != nil {
		return nil, err
	}
	return newParser(tokens, source).parseFile()
}

// Validate reports whether source is syntactically valid TypeScript.
func Validate(source string) error {
	_, err := Parse(source)
	return err
}
