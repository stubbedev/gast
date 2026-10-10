package ts

// Parse tokenizes and parses TypeScript source and returns the file
// AST. A nil error means the source is syntactically valid
// TypeScript; lexical and grammar violations return *gast.LexError or
// *gast.ParseError. No type checking is performed. JSX is not enabled;
// use ParseTSX for .tsx sources.
func Parse(source string) (*File, error) {
	return parse(source, false)
}

// ParseTSX tokenizes and parses TypeScript source with JSX enabled and
// returns the file AST, like Parse for .tsx sources. JSX elements and
// fragments are expressions; per tsc, legacy angle-bracket assertions
// are illegal in this mode and generic arrows must disambiguate
// themselves with a constraint, default, or trailing comma.
func ParseTSX(source string) (*File, error) {
	return parse(source, true)
}

// parse is the shared entry point behind Parse and ParseTSX. TSX mode
// does not pre-tokenize: JSX text is only lexable once the parser has
// decided where an element starts.
func parse(source string, jsx bool) (*File, error) {
	if !jsx {
		tokens, err := Tokenize(source)
		if err != nil {
			return nil, err
		}
		return newParser(tokens, source, false).parseFile()
	}
	return newParser(nil, source, true).parseFile()
}

// Validate reports whether source is syntactically valid TypeScript.
func Validate(source string) error {
	_, err := Parse(source)
	return err
}

// ValidateTSX reports whether source is syntactically valid TSX.
func ValidateTSX(source string) error {
	_, err := ParseTSX(source)
	return err
}
