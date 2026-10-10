package ts

import (
	"slices"

	"github.com/stubbedev/gast/pkg/gast"
	"github.com/stubbedev/gast/pkg/lang/js"
)

// Tokenize scans TypeScript source into tokens, ending with an EOF
// token. TypeScript shares the JavaScript lexical grammar, so
// tokenization (including the regex-versus-division heuristic and
// template re-entry) is delegated to the JavaScript lexer; the only
// TypeScript-only lexeme, the decorator '@', is a token there.
func Tokenize(source string) ([]gast.Token, error) {
	return js.Tokenize(source)
}

// gtOpens reports how many angle brackets a token opens in lookahead
// scans: '<' opens one and '<<' opens two, the latter appearing when
// nested type arguments are lexed as a shift operator.
func gtOpens(kind string) int {
	switch kind {
	case Lt:
		return 1
	case ShiftLeft:
		return 2
	}
	return 0
}

// gtCloses reports how many angle brackets a token closes in lookahead
// scans: the lexer greedily merges runs of '>' (with an optional '='),
// which nested type arguments must peel apart.
func gtCloses(kind string) int {
	switch kind {
	case Gt, GtEqual:
		return 1
	case ShiftRight, ShiftRightAssign:
		return 2
	case UnsignedShift, UnsignedShiftAssign:
		return 3
	}
	return 0
}

// splitGreater rewrites the '>'-run token at pos (">>", ">>>", ">=",
// ">>=", ">>>=") into single-character tokens, the first being '>'. It
// is the lexical half of generic-closing: the parser rewrites a merged
// token when a type-argument list closes, which the immutable
// gast.TokenCursor cannot do.
func splitGreater(tokens []gast.Token, pos int) ([]gast.Token, bool) {
	tok := tokens[pos]
	if gtCloses(tok.Kind) < 1 || tok.Text == "" || tok.Text[0] != '>' {
		return tokens, false
	}
	parts := make([]gast.Token, 0, len(tok.Text))
	for i := range len(tok.Text) {
		kind := Gt
		if tok.Text[i] == '=' {
			kind = Assign
		}
		parts = append(parts, gast.Token{
			Kind: kind, Text: tok.Text[i : i+1],
			Span: gast.Span{Start: tok.Span.Start + i, End: tok.Span.Start + i + 1},
		})
	}
	return slices.Concat(tokens[:pos], parts, tokens[pos+1:]), true
}
