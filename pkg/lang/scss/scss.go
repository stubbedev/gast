// Package scss implements an SCSS frontend for gast. It extends the
// shared CSS core with nesting, parent selectors, $variables, #{}
// interpolation, // comments, mixins and functions, and the Sass
// control at-rules. Nested properties (font: { family: ... }) and the
// legacy indented-dialect = and + mixin syntax are not supported.
package scss

import (
	"github.com/stubbedev/gast/internal/sasscore"
	"github.com/stubbedev/gast/pkg/gast"
)

// File is the root stylesheet node of a parsed source file.
type File = sasscore.Stylesheet

// Tokenize scans source into SCSS tokens, ending with an EOF token.
// Comments are trivia and never surface.
func Tokenize(source string) ([]gast.Token, error) {
	return sasscore.Tokenize(source, sasscore.DialectSCSS)
}

// Parse tokenizes and parses source and returns the stylesheet AST. A
// nil error means the source is syntactically valid SCSS; lexical and
// grammar violations return *gast.LexError or *gast.ParseError.
func Parse(source string) (*File, error) {
	return sasscore.Parse(source, sasscore.DialectSCSS)
}

// Validate reports whether source is syntactically valid SCSS.
func Validate(source string) error {
	_, err := Parse(source)
	return err
}
