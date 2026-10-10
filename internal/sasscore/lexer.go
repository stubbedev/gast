package sasscore

import (
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

// lexer tokenizes stylesheet source for any dialect. In the indented
// dialect it additionally tracks line structure and emits newline,
// indent, and dedent tokens between statements.
type lexer struct {
	*gast.Cursor
	dialect Dialect
	depth   int // open (, [, {, and #{ groups; newlines inside are whitespace
	interp  int // open #{ groups; their } closes the interpolation
	// Indented-dialect line tracking.
	atLineStart    bool
	indents        []int
	pendingDedents int
}

// Tokenize scans source into tokens for the dialect, ending with an
// EOF token. Comments are trivia and never surface; unterminated
// strings, comments, and urls are lexical errors.
func Tokenize(source string, dialect Dialect) ([]gast.Token, error) {
	l := &lexer{
		Cursor:      gast.NewCursor(source),
		dialect:     dialect,
		atLineStart: dialect.Indented,
		indents:     []int{0},
	}
	var tokens []gast.Token
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

// Next returns the next token, skipping whitespace and comments.
func (l *lexer) Next() (gast.Token, error) {
	if l.pendingDedents > 0 {
		l.pendingDedents--
		offset := l.Offset()
		return gast.Token{Kind: KindDedent, Span: gast.Span{Start: offset, End: offset}}, nil
	}
	for {
		if l.dialect.Indented && l.atLineStart {
			token, err := l.handleLineStart()
			if err != nil {
				return gast.Token{}, err
			}
			if token.Kind != "" {
				return token, nil
			}
			if l.atLineStart {
				continue
			}
		}
		if l.AtEnd() {
			return l.eofToken(), nil
		}
		b, _ := l.Peek(0)
		switch {
		case b == ' ' || b == '\t' || b == '\r':
			l.Advance()
			continue
		case b == '\n':
			l.StartToken()
			l.Advance()
			if l.dialect.Indented && l.depth == 0 {
				l.atLineStart = true
				return l.token(KindNewline), nil
			}
			continue
		case b == '/' && l.CheckAhead(1, '*'):
			if err := l.skipBlockComment(); err != nil {
				return gast.Token{}, err
			}
			continue
		case b == '/' && l.CheckAhead(1, '/') && l.dialect.SCSS:
			l.skipLineComment()
			continue
		}
		l.StartToken()
		return l.scanToken(b)
	}
}

func (l *lexer) eofToken() gast.Token {
	offset := l.Offset()
	return gast.Token{Kind: KindEOF, Span: gast.Span{Start: offset, End: offset}}
}

func (l *lexer) token(kind string) gast.Token {
	return gast.Token{Kind: kind, Text: l.MarkedText(), Span: l.Marked()}
}

func (l *lexer) delim(b byte) gast.Token {
	return gast.Token{Kind: string(b), Text: string(b), Span: l.Marked()}
}

// handleLineStart processes the start of a content line in the
// indented dialect: it swallows blank and comment-only lines, measures
// the indentation, and emits indent or dedent tokens. The returned
// token has an empty kind when token scanning should simply continue
// on the current line.
func (l *lexer) handleLineStart() (gast.Token, error) {
	for {
		runStart := l.Offset()
		spaces, tabs := 0, 0
		for {
			b, ok := l.Peek(0)
			if !ok || (b != ' ' && b != '\t') {
				break
			}
			if b == ' ' {
				spaces++
			} else {
				tabs++
			}
			l.Advance()
		}
		if spaces > 0 && tabs > 0 {
			return gast.Token{}, &gast.LexError{
				Message: "mixed tabs and spaces in indentation",
				Pos:     gast.Span{Start: runStart, End: l.Offset()},
			}
		}
		width := spaces + tabs
		b, ok := l.Peek(0)
		if !ok {
			l.atLineStart = false
			return gast.Token{}, nil
		}
		if b == '\r' || b == '\n' { // blank line
			l.Advance()
			continue
		}
		if l.lineIsCommentOnly() {
			l.swallowCommentLine()
			continue
		}
		l.atLineStart = false
		top := l.indents[len(l.indents)-1]
		span := gast.Span{Start: runStart, End: l.Offset()}
		switch {
		case width > top:
			l.indents = append(l.indents, width)
			return gast.Token{Kind: KindIndent, Span: span}, nil
		case width == top:
			return gast.Token{}, nil
		}
		dedents := 0
		for len(l.indents) > 1 && l.indents[len(l.indents)-1] > width {
			l.indents = l.indents[:len(l.indents)-1]
			dedents++
		}
		if l.indents[len(l.indents)-1] != width {
			return gast.Token{}, &gast.LexError{
				Message: "inconsistent dedent",
				Pos:     gast.Span{Start: runStart, End: l.Offset()},
			}
		}
		l.pendingDedents = dedents - 1
		return gast.Token{Kind: KindDedent, Span: span}, nil
	}
}

// lineIsCommentOnly reports whether the current line holds nothing but
// a // or /* */ comment; such lines never affect indentation.
func (l *lexer) lineIsCommentOnly() bool {
	rest := l.Source()[l.Offset():]
	if l.dialect.SCSS && strings.HasPrefix(rest, "//") {
		return true
	}
	if strings.HasPrefix(rest, "/*") {
		return strings.Contains(rest, "*/")
	}
	return false
}

// swallowCommentLine consumes a comment-only line up to but excluding
// its newline. Multi-line block comments are consumed through the end
// of the line that closes them.
func (l *lexer) swallowCommentLine() {
	rest := l.Source()[l.Offset():]
	stop := 0
	if strings.HasPrefix(rest, "/*") {
		if end := strings.Index(rest, "*/"); end >= 0 {
			stop = end + 2
		}
	}
	if newline := strings.IndexByte(rest[stop:], '\n'); newline >= 0 {
		stop += newline
	}
	l.SetOffset(l.Offset() + stop)
}

func (l *lexer) skipBlockComment() error {
	l.StartToken()
	l.Advance()
	l.Advance()
	for {
		if l.AtEnd() {
			return l.Fail("unclosed comment")
		}
		if l.Check(Asterisk[0]) && l.CheckAhead(1, '/') {
			l.Advance()
			l.Advance()
			return nil
		}
		l.Advance()
	}
}

func (l *lexer) skipLineComment() {
	l.SkipWhile(func(b byte) bool { return b != '\n' })
}

func (l *lexer) scanToken(b byte) (gast.Token, error) {
	switch {
	case b == Lt[0] && strings.HasPrefix(l.Source()[l.Offset():], CDOText):
		l.SetOffset(l.Offset() + len(CDOText))
		return l.token(KindCDO), nil
	case b == Minus[0] && strings.HasPrefix(l.Source()[l.Offset():], CDCText):
		l.SetOffset(l.Offset() + len(CDCText))
		return l.token(KindCDC), nil
	case (b == 'u' || b == 'U') && l.matchesURLOpen():
		return l.scanURL()
	case (b == 'u' || b == 'U') && l.CheckAhead(1, Plus[0]):
		if token, ok := l.scanUnicodeRange(); ok {
			return token, nil
		}
		return l.scanName()
	case l.startsNumber(b):
		return l.scanNumber()
	case l.startsNameAt(0):
		return l.scanName()
	case b == '"' || b == '\'':
		return l.scanString(b)
	case b == HashMark[0]:
		return l.scanHash()
	case b == At[0] && l.nameStartsAt(1):
		l.Advance()
		l.scanNameBytes()
		return l.token(KindAtKeyword), nil
	case b == Dollar[0] && l.dialect.SCSS && l.nameStartsAt(1):
		l.Advance()
		l.scanNameBytes()
		return l.token(KindVariable), nil
	case b == LParen[0] || b == LBracket[0] || b == LBrace[0]:
		l.depth++
		l.Advance()
		return l.delim(b), nil
	case b == RParen[0] || b == RBracket[0] || b == RBrace[0]:
		if l.depth > 0 {
			l.depth--
		}
		if b == RBrace[0] && l.interp > 0 {
			l.interp--
		}
		l.Advance()
		return l.delim(b), nil
	}
	l.Advance()
	return l.delim(b), nil
}

// scanName scans an identifier; an identifier immediately followed by
// "(" becomes a function token whose text includes the parenthesis.
func (l *lexer) scanName() (gast.Token, error) {
	l.scanNameBytes()
	if l.Check(LParen[0]) {
		l.depth++
		l.Advance()
		return l.token(KindFunction), nil
	}
	return l.token(KindIdent), nil
}

// scanNameBytes consumes identifier bytes, including \ escapes.
func (l *lexer) scanNameBytes() {
	for {
		b, ok := l.Peek(0)
		if !ok {
			return
		}
		if isNameByte(b) {
			l.Advance()
			continue
		}
		if b != '\\' {
			return
		}
		l.Advance()
		next, ok := l.Peek(0)
		if !ok {
			return
		}
		if hexValue(next) >= 0 {
			l.Advance()
			for range 5 {
				after, ok := l.Peek(0)
				if !ok || hexValue(after) < 0 {
					break
				}
				l.Advance()
			}
			if l.Check(' ') || l.Check('\t') {
				l.Advance()
			}
			continue
		}
		l.Advance()
	}
}

func (l *lexer) scanHash() (gast.Token, error) {
	if l.dialect.SCSS && l.CheckAhead(1, LBrace[0]) {
		l.Advance()
		l.Advance()
		l.depth++
		l.interp++
		return l.token(KindInterpOpen), nil
	}
	if l.nameStartsAt(1) {
		l.Advance()
		l.scanNameBytes()
		return l.token(KindHashID), nil
	}
	if b, ok := l.Peek(1); ok && isNameByte(b) {
		l.Advance()
		l.scanNameBytes()
		return l.token(KindHash), nil
	}
	l.Advance()
	return l.delim(HashMark[0]), nil
}

func (l *lexer) scanNumber() (gast.Token, error) {
	if l.Check(Plus[0]) || l.Check(Minus[0]) {
		l.Advance()
	}
	l.TakeWhile(isDigit)
	if l.Match(Dot[0]) {
		l.TakeWhile(isDigit)
	}
	if l.Check('e') || l.Check('E') {
		mark := l.Offset()
		l.Advance()
		if l.Check(Plus[0]) || l.Check(Minus[0]) {
			l.Advance()
		}
		if l.TakeWhile(isDigit) == 0 {
			l.SetOffset(mark)
		}
	}
	if l.Match(Percent[0]) {
		return l.token(KindPercentage), nil
	}
	if l.startsNameAt(0) {
		l.scanNameBytes()
		return l.token(KindDimension), nil
	}
	return l.token(KindNumber), nil
}

func (l *lexer) scanString(quote byte) (gast.Token, error) {
	l.Advance()
	if err := l.scanStringBody(quote); err != nil {
		return gast.Token{}, err
	}
	l.Advance()
	return l.token(KindString), nil
}

// scanStringBody consumes string content up to the closing quote,
// handling escapes and escaped newlines.
func (l *lexer) scanStringBody(quote byte) error {
	for {
		b, ok := l.Peek(0)
		if !ok || b == '\n' {
			return l.Fail("unclosed string")
		}
		if b == quote {
			return nil
		}
		if b != '\\' {
			l.Advance()
			continue
		}
		l.Advance()
		escaped, ok := l.Peek(0)
		if !ok {
			return l.Fail("unclosed string")
		}
		switch {
		case escaped == '\n':
			l.Advance()
		case hexValue(escaped) >= 0:
			l.Advance()
			for range 5 {
				after, ok := l.Peek(0)
				if !ok || hexValue(after) < 0 {
					break
				}
				l.Advance()
			}
			if l.Match('\r') {
				l.Match('\n')
			} else if !l.Match(' ') {
				l.Match('\t') // one whitespace consumed as part of the escape
			}
		default:
			l.Advance()
		}
	}
}

// matchesURLOpen reports whether a url( token starts at the cursor.
func (l *lexer) matchesURLOpen() bool {
	rest := l.Source()[l.Offset():]
	return len(rest) >= 4 && strings.EqualFold(rest[:3], "url") && rest[3] == LParen[0]
}

// scanURL scans a url(...) token in one piece, quoted or raw.
func (l *lexer) scanURL() (gast.Token, error) {
	l.SetOffset(l.Offset() + 4) // url(
	for {
		b, ok := l.Peek(0)
		if !ok {
			return gast.Token{}, l.Fail("unclosed url")
		}
		switch b {
		case RParen[0]:
			l.Advance()
			return l.token(KindURL), nil
		case '"', '\'':
			l.Advance()
			if err := l.scanStringBody(b); err != nil {
				return gast.Token{}, err
			}
			l.Advance()
		case '\n', '(':
			return gast.Token{}, l.Fail("unclosed url")
		default:
			l.Advance()
		}
	}
}

// scanUnicodeRange scans a U+... token; ok is false when the bytes do
// not form a unicode-range, leaving the cursor untouched.
func (l *lexer) scanUnicodeRange() (gast.Token, bool) {
	mark := l.Offset()
	l.Advance() // u
	l.Advance() // +
	hexRun := l.TakeWhile(func(b byte) bool { return hexValue(b) >= 0 || b == '?' })
	if hexRun == 0 {
		l.SetOffset(mark)
		return gast.Token{}, false
	}
	if l.Match(Minus[0]) {
		l.TakeWhile(func(b byte) bool { return hexValue(b) >= 0 })
	}
	return l.token(KindUnicodeRange), true
}

// startsNumber reports whether a number token starts at the cursor.
func (l *lexer) startsNumber(b byte) bool {
	if isDigit(b) {
		return true
	}
	if b == Dot[0] && l.digitAt(1) {
		return true
	}
	if b != Plus[0] && b != Minus[0] {
		return false
	}
	return l.digitAt(1) || (l.CheckAhead(1, Dot[0]) && l.digitAt(2))
}

func (l *lexer) startsNameAt(ahead int) bool {
	b, ok := l.Peek(ahead)
	if !ok {
		return false
	}
	if isNameStart(b) || b == '\\' {
		return true
	}
	if b != Minus[0] {
		return false
	}
	next, ok := l.Peek(ahead + 1)
	return ok && (next == Minus[0] || isNameStart(next))
}

func (l *lexer) nameStartsAt(ahead int) bool {
	b, ok := l.Peek(ahead)
	return ok && (isNameStart(b) || b == '\\')
}

func (l *lexer) digitAt(ahead int) bool {
	b, ok := l.Peek(ahead)
	return ok && isDigit(b)
}

func isDigit(b byte) bool { return '0' <= b && b <= '9' }

func isNameStart(b byte) bool {
	return b == '_' || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || b >= 0x80
}

func isNameByte(b byte) bool {
	return isNameStart(b) || isDigit(b) || b == Minus[0]
}

func hexValue(b byte) int {
	switch {
	case '0' <= b && b <= '9':
		return int(b - '0')
	case 'a' <= b && b <= 'f':
		return int(b-'a') + 10
	case 'A' <= b && b <= 'F':
		return int(b-'A') + 10
	}
	return -1
}
