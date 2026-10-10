// Package css implements a CSS frontend for gast: a lexer, a
// recursive-descent parser producing a spanned AST, and syntax
// validation. It targets the CSS Syntax Level 3 subset shared with the
// SCSS and SASS frontends, which are built on the same core.
package css

import (
	"github.com/stubbedev/gast/internal/sasscore"
	"github.com/stubbedev/gast/pkg/gast"
)

// File is the root stylesheet node of a parsed source file.
type File = sasscore.Stylesheet

// Tokenize scans source into CSS tokens, ending with an EOF token.
// Comments are trivia and never surface.
func Tokenize(source string) ([]gast.Token, error) {
	return sasscore.Tokenize(source, sasscore.DialectCSS)
}

// Parse tokenizes and parses source and returns the stylesheet AST. A
// nil error means the source is syntactically valid CSS; lexical and
// grammar violations return *gast.LexError or *gast.ParseError.
func Parse(source string) (*File, error) {
	return sasscore.Parse(source, sasscore.DialectCSS)
}

// Validate reports whether source is syntactically valid CSS.
func Validate(source string) error {
	_, err := Parse(source)
	return err
}
