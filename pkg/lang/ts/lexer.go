package ts

import (
	"slices"

	"github.com/stubbedev/gast/pkg/gast"
	"github.com/stubbedev/gast/pkg/lang/js"
)

// KindJSXText is a raw JSX text run, produced only by the lazy TSX
// tokenizer in child position.
const KindJSXText = "JSXText"

// Tokenize scans TypeScript source into tokens, ending with an EOF
// token. TypeScript shares the JavaScript lexical grammar, so
// tokenization (including the regex-versus-division heuristic and
// template re-entry) is delegated to the JavaScript lexer; the only
// TypeScript-only lexeme, the decorator '@', is a token there.
// JSX-aware tokenization is parser-driven instead: see newTSXLexer.
func Tokenize(source string) ([]gast.Token, error) {
	return js.Tokenize(source)
}

// lexMode classifies where the lazy TSX tokenizer sits. The parser
// drives the mode; JavaScript and TypeScript lexing is delegated to
// the js lexer everywhere except JSX-specific positions, where it
// would misread the input: the '/' of '</' and '/>' starts a regex,
// and quotes, '#', and backticks in child text start literals.
type lexMode int

const (
	lexJS   lexMode = iota // ordinary JavaScript/TypeScript
	lexTag                 // inside '<name ...' or '</name'
	lexText                // JSX child text
)

// tsxLexer lazily tokenizes TSX source on demand for the parser. In
// lexTag mode the delimiters '<', '>', '/', '{', and '}' lex as single
// characters so '/>' and merged '>' runs cannot form, and names and
// strings come from the js lexer; in lexText mode a whole raw run up
// to a boundary '{' or tag-opening '<' is one KindJSXText token.
type tsxLexer struct {
	*js.Lexer
	mode lexMode
}

func newTSXLexer(source string) *tsxLexer {
	return &tsxLexer{Lexer: js.NewLexer(source)}
}

// next returns the next token in the current mode.
func (l *tsxLexer) next() (gast.Token, error) {
	switch l.mode {
	case lexText:
		return l.nextText()
	case lexTag:
		return l.nextTag()
	}
	return l.Next()
}

// nextText scans a raw JSX text run. Text is verbatim source: '<' and
// '>' are legal characters and entities like &amp; stay undecoded. A
// '<' only ends the run when it opens an element or closing tag.
func (l *tsxLexer) nextText() (gast.Token, error) {
	source, start := l.Source(), l.Offset()
	end := start
	for end < len(source) && !textBoundary(source, end) {
		end++
	}
	if end > start {
		l.SetOffset(end)
		return gast.Token{
			Kind: KindJSXText, Text: source[start:end],
			Span: gast.Span{Start: start, End: end},
		}, nil
	}
	// No text before the boundary: the '{' or '<' is a single-character
	// token, exactly what nextTag emits at a non-trivia byte.
	return l.nextTag()
}

// textBoundary reports whether source[at] ends a JSX text run: an
// expression container '{' or a '<' that opens an element, fragment,
// or closing tag (a lone '<' is plain text).
func textBoundary(source string, at int) bool {
	switch source[at] {
	case '{':
		return true
	case '<':
		if at+1 >= len(source) {
			return false
		}
		switch b := source[at+1]; {
		case b == '>' || b == '/':
			return true
		case b == '$' || b == '_', isAlphaByte(b), b >= 0x80:
			return true
		}
	}
	return false
}

// nextTag scans one token of an opening or closing tag. Delimiters lex
// as single characters; anything else (names, strings, ',', '=',
// '...') comes from the js lexer.
func (l *tsxLexer) nextTag() (gast.Token, error) {
	l.skipTagTrivia()
	if l.AtEnd() {
		return gast.Token{Kind: KindEOF, Span: gast.Span{Start: l.Offset(), End: l.Offset()}}, nil
	}
	b, _ := l.Peek(0)
	var kind string
	switch b {
	case '<':
		kind = Lt
	case '>':
		kind = Gt
	case '/':
		kind = Slash
	case '{':
		kind = LBrace
	case '}':
		kind = RBrace
	default:
		return l.Next()
	}
	l.StartToken()
	l.Advance()
	return gast.Token{Kind: kind, Text: string(b), Span: l.Marked()}, nil
}

// skipTagTrivia consumes whitespace and comments between attributes;
// the js lexer's own trivia skipping cannot be reused because a '>' or
// '/' after a comment must still lex as a single character.
func (l *tsxLexer) skipTagTrivia() {
	for !l.AtEnd() {
		b, _ := l.Peek(0)
		switch b {
		case ' ', '\t', '\n', '\r', '\f', '\v', 0xE2: // includes U+2028/2029 prefixes
			l.Advance()
		case '/':
			switch next, _ := l.Peek(1); next {
			case '/':
				l.SkipWhile(func(b byte) bool { return b != '\n' && b != '\r' })
			case '*':
				l.Advance()
				l.Advance()
				for !l.AtEnd() && (!l.Check('*') || !l.CheckAhead(1, '/')) {
					l.Advance()
				}
				l.Advance()
				l.Advance()
			default:
				return
			}
		default:
			return
		}
	}
}

// rewind resets the lexer to re-lex from offset, discarding the mode
// and every internal token of the js lexer. The parser rewinds after a
// failed speculative parse, which always starts and therefore ends in
// lexJS mode.
func (l *tsxLexer) rewind(source string, offset int) {
	l.Lexer = js.NewLexer(source)
	l.SetOffset(offset)
	l.mode = lexJS
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
