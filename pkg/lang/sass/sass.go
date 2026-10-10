// Package sass implements the indented SASS frontend for gast. It is
// the SCSS grammar without braces or semicolons: the shared scanner is
// wrapped in a newline- and indentation-sensitive layer that emits
// newline, indent, and dedent tokens. The legacy = and + mixin syntax
// and nested properties are not supported.
package sass

import (
	"github.com/stubbedev/gast/internal/sasscore"
	"github.com/stubbedev/gast/pkg/gast"
)

// File is the root stylesheet node of a parsed source file.
type File = sasscore.Stylesheet

// Tokenize scans source into SASS tokens, ending with an EOF token.
// Comments are trivia and never surface.
func Tokenize(source string) ([]gast.Token, error) {
	return sasscore.Tokenize(source, sasscore.DialectSass)
}

// Parse tokenizes and parses source and returns the stylesheet AST. A
// nil error means the source is syntactically valid SASS; lexical and
// grammar violations return *gast.LexError or *gast.ParseError.
func Parse(source string) (*File, error) {
	return sasscore.Parse(source, sasscore.DialectSass)
}

// Validate reports whether source is syntactically valid SASS.
func Validate(source string) error {
	_, err := Parse(source)
	return err
}
