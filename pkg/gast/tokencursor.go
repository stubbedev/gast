package gast

// TokenCursor walks a token slice for hand-written parsers. Reads past
// the end yield a synthetic end-of-input token instead of failing, so
// parser code can peek freely.
type TokenCursor struct {
	tokens []Token
	pos    int
	eof    Token
}

// NewTokenCursor returns a cursor over tokens. EOFKind is the kind the
// synthetic end-of-input token reports.
func NewTokenCursor(tokens []Token, eofKind string) *TokenCursor {
	end := 0
	if len(tokens) > 0 {
		end = tokens[len(tokens)-1].Span.End
	}
	return &TokenCursor{tokens: tokens, eof: Token{Kind: eofKind, Span: Span{Start: end, End: end}}}
}

// Current returns the token at the cursor.
func (p *TokenCursor) Current() Token { return p.Peek(0) }

// Peek returns the token ahead positions from the cursor, or the
// end-of-input token past the end.
func (p *TokenCursor) Peek(ahead int) Token {
	if index := p.pos + ahead; index >= 0 && index < len(p.tokens) {
		return p.tokens[index]
	}
	return p.eof
}

// AtEnd reports whether every token has been consumed.
func (p *TokenCursor) AtEnd() bool { return p.pos >= len(p.tokens) }

// Check reports whether the current token has kind.
func (p *TokenCursor) Check(kind string) bool { return p.Current().Kind == kind }

// CheckAhead reports whether the token ahead positions from the cursor
// has kind.
func (p *TokenCursor) CheckAhead(ahead int, kind string) bool {
	return p.Peek(ahead).Kind == kind
}

// Advance consumes and returns the current token. At the end of input it
// keeps returning the end-of-input token without consuming anything.
func (p *TokenCursor) Advance() Token {
	token := p.Current()
	if !p.AtEnd() {
		p.pos++
	}
	return token
}

// Match consumes the current token if it has kind.
func (p *TokenCursor) Match(kind string) bool {
	if !p.Check(kind) {
		return false
	}
	p.pos++
	return true
}

// Expect consumes the current token if it has kind. Description replaces
// the default quoted kind in the error message.
func (p *TokenCursor) Expect(kind, description string) (Token, error) {
	if p.Check(kind) {
		return p.Advance(), nil
	}
	if description == "" {
		description = "'" + kind + "'"
	}
	return Token{}, p.Fail("expected " + description + ", found " + Describe(p.Current()))
}

// Fail returns a ParseError at the current token.
func (p *TokenCursor) Fail(message string) error {
	return &ParseError{Message: message, Pos: p.Current().Span}
}

// FailAt returns a ParseError at an explicit span.
func (p *TokenCursor) FailAt(span Span, message string) error {
	return &ParseError{Message: message, Pos: span}
}

// SpanFrom merges start with the span of the last consumed token.
func (p *TokenCursor) SpanFrom(start Span) Span {
	if p.pos == 0 {
		return start
	}
	return Merge(start, p.tokens[p.pos-1].Span)
}
