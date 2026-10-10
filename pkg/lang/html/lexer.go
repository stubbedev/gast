package html

import (
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

// lexer tokenizes HTML source in a single pass. A tag is emitted as a
// token sequence - the tag-open token, one token per attribute, and a
// closing token (">" or "/>") - so attribute scanning needs no
// per-attribute allocations beyond the tokens themselves. Extra tokens
// of a sequence are buffered and flushed before new input is scanned.
type lexer struct {
	*gast.Cursor
	raw     string // open raw-text/rcdata element name, "" in normal mode
	pending []gast.Token
}

func newLexer(source string) *lexer {
	return &lexer{Cursor: gast.NewCursor(source)}
}

// Tokenize scans source into tokens, ending with an EOF token.
func Tokenize(source string) ([]gast.Token, error) {
	l := newLexer(source)
	tokens := []gast.Token{}
	for {
		token, err := l.Next()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
		if token.Kind == KindEOF {
			return tokens, nil
		}
	}
}

func (l *lexer) eofToken() gast.Token {
	offset := l.Offset()
	return gast.Token{Kind: KindEOF, Span: gast.Span{Start: offset, End: offset}}
}

func (l *lexer) token(kind string) gast.Token {
	return gast.Token{Kind: kind, Text: l.MarkedText(), Span: l.Marked()}
}

// emit records a token of the current tag sequence in the pending
// buffer.
func (l *lexer) emit(kind string) {
	l.pending = append(l.pending, l.token(kind))
}

// Next returns the next token. Character data is one token per run,
// raw "<" that starts no markup stays in the text, and lexical errors
// surface as *gast.LexError.
func (l *lexer) Next() (gast.Token, error) {
	if len(l.pending) > 0 {
		token := l.pending[0]
		l.pending = l.pending[1:]
		return token, nil
	}
	if l.raw != "" {
		return l.scanRaw()
	}
	if l.AtEnd() {
		return l.eofToken(), nil
	}
	if l.markupStarts() {
		return l.scanMarkup()
	}
	return l.scanText(), nil
}

// scanText consumes character data up to the next markup start.
func (l *lexer) scanText() gast.Token {
	l.StartToken()
	for !l.AtEnd() && !l.markupStarts() {
		l.Advance()
	}
	return l.token(KindText)
}

// markupStarts reports whether the cursor sits on a "<" that begins
// markup: a declaration, a comment, or a tag with a name.
func (l *lexer) markupStarts() bool {
	if !l.Check('<') {
		return false
	}
	next, ok := l.Peek(1)
	if !ok {
		return false
	}
	switch {
	case next == '!':
		return true
	case next == '/':
		after, ok := l.Peek(2)
		return ok && isASCIILetter(after)
	case isASCIILetter(next):
		return true
	}
	return false
}

func (l *lexer) scanMarkup() (gast.Token, error) {
	l.StartToken()
	next, _ := l.Peek(1)
	if next == '!' {
		return l.scanBang()
	}
	return l.scanTag(next == '/')
}

// scanBang consumes a "<!" sequence: a comment, a CDATA section, a
// doctype, or a bogus comment ending at the first ">".
func (l *lexer) scanBang() (gast.Token, error) {
	rest := l.Source()[l.Offset():]
	switch {
	case strings.HasPrefix(rest, commentStart):
		for range len(commentStart) {
			l.Advance()
		}
		return l.scanUntil(KindComment, commentEnd, "unterminated comment")
	case hasPrefixFold(rest, cdataStart):
		for range len(cdataStart) {
			l.Advance()
		}
		return l.scanUntil(KindCDATA, cdataEnd, "unterminated CDATA section")
	case hasPrefixFold(rest, doctypeStart):
		for range len(doctypeStart) {
			l.Advance()
		}
		return l.scanUntil(KindDoctype, tagCloseText, "unterminated doctype")
	default:
		l.Advance()
		l.Advance()
		return l.scanUntil(KindComment, tagCloseText, "unterminated comment")
	}
}

// scanUntil consumes input through the next occurrence of terminator
// and returns the token marked before it.
func (l *lexer) scanUntil(kind, terminator, message string) (gast.Token, error) {
	for {
		if l.AtEnd() {
			return gast.Token{}, l.fail(message)
		}
		if strings.HasPrefix(l.Source()[l.Offset():], terminator) {
			for range len(terminator) {
				l.Advance()
			}
			return l.token(kind), nil
		}
		l.Advance()
	}
}

// scanTag consumes a start or end tag with its attributes and buffers
// every token after the first. A start tag of a raw-text or rcdata
// element (not self-closed) switches the lexer into raw mode.
func (l *lexer) scanTag(end bool) (gast.Token, error) {
	l.Advance() // <
	l.Advance() // '/' or the first name byte
	l.SkipWhile(isTagNameByte)
	nameStart := l.Marked().Start + 1
	if end {
		nameStart++
	}
	name := strings.ToLower(l.Source()[nameStart:l.Offset()])
	kind := KindTagOpen
	if end {
		kind = KindEndTagOpen
	}
	openToken := l.token(kind)
	selfClosed := false
	for {
		l.SkipWhitespace()
		if l.AtEnd() {
			return gast.Token{}, l.fail("unterminated tag")
		}
		if l.Check('>') {
			l.StartToken()
			l.Advance()
			l.emit(KindTagClose)
			break
		}
		if l.Check('/') && l.CheckAhead(1, '>') {
			l.Advance()
			l.Advance()
			l.emit(KindSelfClose)
			selfClosed = true
			break
		}
		if err := l.scanAttribute(); err != nil {
			return gast.Token{}, err
		}
	}
	if !end && !selfClosed && (rawTextElements[name] || rcdataElements[name]) {
		l.raw = name
	}
	return openToken, nil
}

// scanAttribute consumes one attribute: a name, optionally followed by
// "=" and a double-quoted, single-quoted, or unquoted value. A stray
// byte that forms no attribute is skipped.
func (l *lexer) scanAttribute() error {
	if l.Check('=') {
		l.Advance() // stray '=' before a name: dropped
	}
	l.StartToken()
	l.SkipWhile(isAttrNameByte)
	nameEnd := l.Offset()
	l.SkipWhitespace()
	if !l.Match('=') {
		l.SetOffset(nameEnd)
		if l.Offset() == l.Marked().Start {
			l.Advance() // forms no attribute; skip to make progress
			return nil
		}
		l.emit(KindAttr)
		return nil
	}
	l.SkipWhitespace()
	if err := l.scanAttributeValue(); err != nil {
		return err
	}
	l.emit(KindAttr)
	return nil
}

// scanAttributeValue consumes a quoted or unquoted value. A quote with
// no closing partner is an error pointing at the opening quote.
func (l *lexer) scanAttributeValue() error {
	quote, quoted := l.Peek(0)
	if !quoted || (quote != '"' && quote != '\'') {
		l.SkipWhile(func(b byte) bool { return !isWhitespace(b) && b != '>' })
		return nil
	}
	start := l.Offset()
	l.Advance()
	for {
		if l.AtEnd() {
			return &gast.LexError{
				Message: "unclosed quote in attribute value",
				Pos:     gast.Span{Start: start, End: start + 1},
			}
		}
		if l.Match(quote) {
			return nil
		}
		l.Advance()
	}
}

// scanRaw consumes the content of a raw-text or rcdata element as one
// verbatim token, ending only at the proper case-insensitive end tag.
func (l *lexer) scanRaw() (gast.Token, error) {
	kind := KindRawText
	if rcdataElements[l.raw] {
		kind = KindRCDATA
	}
	l.StartToken()
	for {
		if l.AtEnd() {
			return gast.Token{}, l.fail("unterminated " + l.raw + " element")
		}
		if l.Check('<') && l.CheckAhead(1, '/') && l.closesRawElement() {
			l.raw = ""
			if l.MarkedText() == "" {
				return l.Next()
			}
			return l.token(kind), nil
		}
		l.Advance()
	}
}

// closesRawElement reports whether the cursor sits on the end tag of
// the open raw-text element: "</" plus its name, case-insensitively,
// followed by whitespace, "/", or ">".
func (l *lexer) closesRawElement() bool {
	for i := range len(l.raw) {
		b, ok := l.Peek(2 + i)
		if !ok || !asciiEqualFold(b, l.raw[i]) {
			return false
		}
	}
	after, ok := l.Peek(2 + len(l.raw))
	return ok && (isWhitespace(after) || after == '/' || after == '>')
}

func (l *lexer) fail(message string) error {
	return l.Fail(message)
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

func asciiEqualFold(a, b byte) bool {
	if 'A' <= a && a <= 'Z' {
		a += 'a' - 'A'
	}
	if 'A' <= b && b <= 'Z' {
		b += 'a' - 'A'
	}
	return a == b
}

func isASCIILetter(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

func isTagNameByte(b byte) bool {
	return !isWhitespace(b) && b != '/' && b != '>'
}

func isAttrNameByte(b byte) bool {
	return !isWhitespace(b) && b != '=' && b != '/' && b != '>'
}
