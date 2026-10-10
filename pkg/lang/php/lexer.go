package php

import (
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

// lexer tokenizes PHP source. Outside of php tags the input is inline
// HTML; <?php and <?= enter php mode and ?> leaves it. After a
// __halt_compiler() call the remainder of the file is raw data.
type lexer struct {
	*gast.Cursor
	inPHP  bool
	halted bool
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

func (l *lexer) fail(message string) error {
	return l.Fail(message)
}

// Next returns the next token. Whitespace and comments are skipped,
// inline HTML is one token per run, and lexical errors surface as
// *gast.LexError.
func (l *lexer) Next() (gast.Token, error) {
	if l.halted {
		return l.haltDataToken(), nil
	}
	if !l.inPHP {
		return l.scanInlineHTML()
	}
	for {
		l.SkipWhitespace()
		if l.AtEnd() {
			return l.eofToken(), nil
		}
		b, _ := l.Peek(0)
		if (b == '/' && l.CheckAhead(1, '/')) || (b == '#' && !l.CheckAhead(1, '[')) {
			l.atLineCommentEnd()
			continue
		}
		if b == '/' && l.CheckAhead(1, '*') {
			if err := l.skipBlockComment(); err != nil {
				return gast.Token{}, err
			}
			continue
		}
		l.StartToken()
		return l.scanToken(b)
	}
}

// haltDataToken returns the raw data token covering the rest of the
// file after __halt_compiler();. Its text is a zero-copy slice of the
// source. Once exhausted, the lexer reports EOF.
func (l *lexer) haltDataToken() gast.Token {
	start := l.Offset()
	end := len(l.Source())
	l.SetOffset(end)
	if start >= end {
		return l.eofToken()
	}
	return gast.Token{
		Kind: KindRawData,
		Text: l.Source()[start:end],
		Span: gast.Span{Start: start, End: end},
	}
}

// atLineCommentEnd consumes a // or # line comment. A close tag ends the
// comment without being consumed, leaving the ?> for the main loop.
func (l *lexer) atLineCommentEnd() {
	for {
		b, ok := l.Peek(0)
		if !ok || b == '\n' {
			return
		}
		if b == '?' && l.CheckAhead(1, '>') {
			return
		}
		l.Advance()
	}
}

func (l *lexer) skipBlockComment() error {
	l.StartToken()
	l.Advance()
	l.Advance()
	for {
		if l.AtEnd() {
			return l.fail("unterminated block comment")
		}
		if l.Check('*') && l.CheckAhead(1, '/') {
			l.Advance()
			l.Advance()
			return nil
		}
		l.Advance()
	}
}

// scanInlineHTML returns one inline HTML token per run, or an open tag.
// A <? that is not a php tag is inline HTML text.
func (l *lexer) scanInlineHTML() (gast.Token, error) {
	for {
		if l.AtEnd() {
			return l.eofToken(), nil
		}
		l.StartToken()
		for !l.AtEnd() && (!l.Check('<') || !l.CheckAhead(1, '?')) {
			l.Advance()
		}
		if l.MarkedText() != "" {
			return l.token(KindInlineHTML), nil
		}
		rest := l.Source()[l.Offset():]
		if strings.HasPrefix(rest, OpenTagText) && openTagFollows(rest) {
			l.SetOffset(l.Offset() + len(OpenTagText))
			l.inPHP = true
			return l.token(KindOpenTag), nil
		}
		if strings.HasPrefix(rest, OpenTagEchoText) {
			l.SetOffset(l.Offset() + len(OpenTagEchoText))
			l.inPHP = true
			return l.token(KindOpenTagEcho), nil
		}
		l.Advance()
		l.Advance()
	}
}

func openTagFollows(rest string) bool {
	after := rest[len(OpenTagText):]
	if after == "" {
		return true
	}
	switch after[0] {
	case ' ', '\t', '\n', '\r':
		return true
	}
	return strings.HasPrefix(after, CloseTagText)
}

func (l *lexer) scanToken(b byte) (gast.Token, error) {
	switch {
	case isIdentStart(b):
		return l.scanIdent()
	case isDigit(b):
		return l.scanNumber()
	case b == '$':
		return l.scanVariable()
	case b == '"':
		return l.scanQuoted('"', true)
	case b == '\'':
		return l.scanQuoted('\'', false)
	case b == '`':
		return l.scanQuoted('`', true)
	case b == '.' && l.checkDigitAt(1):
		return l.scanNumber()
	case b == '(':
		if token, ok := l.scanCast(); ok {
			return token, nil
		}
		l.Advance()
		return l.token(LParen), nil
	case b == '<' && l.checkOp(HeredocStart):
		return l.scanHeredoc()
	case b == '#' && l.CheckAhead(1, '['):
		l.Advance()
		l.Advance()
		return l.token(KindAttributeOpen), nil
	}
	return l.scanOperator()
}

func (l *lexer) scanIdent() (gast.Token, error) {
	l.SkipWhile(isIdentByte)
	if strings.EqualFold(l.MarkedText(), wordHaltCompiler) {
		// The word is reserved everywhere; when the empty call and
		// semicolon follow, the token covers them and the rest of the
		// file becomes raw data.
		if end, ok := l.haltCallEnd(); ok {
			l.SetOffset(end)
			l.halted = true
		}
		return l.token(KindHaltCompiler), nil
	}
	if kind, ok := keywords[strings.ToLower(l.MarkedText())]; ok {
		return l.token(kind), nil
	}
	return l.token(KindIdent), nil
}

// haltCallEnd reports whether the source after the __halt_compiler word
// spells the empty call followed by a semicolon, allowing whitespace
// and comments between the parts: __halt_compiler ( ) ;. It returns the
// offset just past the semicolon.
func (l *lexer) haltCallEnd() (int, bool) {
	end := l.Offset()
	for _, part := range []string{"(", ")", ";"} {
		end = l.skipHaltSpace(end)
		if !strings.HasPrefix(l.Source()[end:], part) {
			return 0, false
		}
		end += len(part)
	}
	return end, true
}

// skipHaltSpace advances over whitespace and comments from offset.
func (l *lexer) skipHaltSpace(offset int) int {
	source := l.Source()
	for offset < len(source) {
		switch source[offset] {
		case ' ', '\t', '\n', '\r':
			offset++
		case '/':
			if strings.HasPrefix(source[offset:], "/*") {
				if end := strings.Index(source[offset+2:], "*/"); end >= 0 {
					offset += 2 + end + 2
					continue
				}
				return len(source)
			}
			if strings.HasPrefix(source[offset:], "//") {
				if next := strings.IndexByte(source[offset:], '\n'); next >= 0 {
					offset += next
					continue
				}
				return len(source)
			}
			return offset
		case '#':
			if strings.HasPrefix(source[offset+1:], "[") {
				return offset
			}
			if next := strings.IndexByte(source[offset:], '\n'); next >= 0 {
				offset += next
				continue
			}
			return len(source)
		default:
			return offset
		}
	}
	return offset
}

// scanVariable scans $name. A $ not followed by a name yields a bare
// "$" token so the parser can build variable variables ($$x).
func (l *lexer) scanVariable() (gast.Token, error) {
	l.Advance()
	next, ok := l.Peek(0)
	if !ok || !isIdentByte(next) || isDigit(next) {
		return l.token(Dollar), nil
	}
	l.SkipWhile(isIdentByte)
	return l.token(KindVariable), nil
}

func (l *lexer) scanNumber() (gast.Token, error) {
	if !l.Check('.') {
		if radix, ok := l.Peek(1); ok && l.Check('0') {
			switch radix {
			case 'x', 'X':
				return l.scanRadix(isHexDigit)
			case 'b', 'B':
				return l.scanRadix(isBinaryDigit)
			case 'o', 'O':
				return l.scanRadix(isOctalDigit)
			}
		}
		if l.Check('0') {
			if next, ok := l.Peek(1); ok && (isDigit(next) || next == '_') {
				return l.scanLegacyOctal()
			}
		}
		l.Advance()
		if err := l.scanDigits(); err != nil {
			return gast.Token{}, err
		}
	}
	isFloat := l.Check('.')
	if isFloat {
		l.Advance()
		if l.Check('_') {
			return gast.Token{}, l.fail("unexpected '_' in number")
		}
		if l.CheckDigit() {
			l.Advance()
			if err := l.scanDigits(); err != nil {
				return gast.Token{}, err
			}
		}
	}
	if next, ok := l.Peek(0); ok && (next == 'e' || next == 'E') {
		isFloat = true
		l.Advance()
		if next, ok := l.Peek(0); ok && (next == '+' || next == '-') {
			l.Advance()
		}
		if !l.CheckDigit() {
			return gast.Token{}, l.fail("expected digits in exponent")
		}
		l.Advance()
		if err := l.scanDigits(); err != nil {
			return gast.Token{}, err
		}
	}
	if err := l.checkNumberEnd(); err != nil {
		return gast.Token{}, err
	}
	if isFloat {
		return l.token(KindFloat), nil
	}
	return l.token(KindInt), nil
}

// scanRadix consumes 0x, 0b, or 0o literals. The first digit must
// appear before any underscore.
func (l *lexer) scanRadix(digit func(byte) bool) (gast.Token, error) {
	l.Advance()
	l.Advance()
	if !l.takeRadixDigit(digit) {
		return gast.Token{}, l.fail("invalid number")
	}
	for {
		if l.takeRadixDigit(digit) {
			continue
		}
		if !l.Check('_') {
			return l.numberEnd()
		}
		if next, ok := l.Peek(1); !ok || !digit(next) {
			return gast.Token{}, l.fail("unexpected '_' in number")
		}
		l.Advance()
	}
}

func (l *lexer) takeRadixDigit(digit func(byte) bool) bool {
	if next, ok := l.Peek(0); ok && digit(next) {
		l.Advance()
		return true
	}
	return false
}

// scanLegacyOctal consumes a 0-prefixed digit run. When the run is
// followed by . or an exponent the literal is a float; otherwise an 8
// or 9 in the run is a malformed octal.
func (l *lexer) scanLegacyOctal() (gast.Token, error) {
	l.Advance()
	l.Advance()
	for {
		if next, ok := l.Peek(0); ok && isDigit(next) {
			l.Advance()
			continue
		}
		if !l.Check('_') {
			break
		}
		if next, ok := l.Peek(1); !ok || !isDigit(next) {
			return gast.Token{}, l.fail("unexpected '_' in number")
		}
		l.Advance()
	}
	if next, ok := l.Peek(0); !ok || (next != '.' && next != 'e' && next != 'E') {
		if strings.ContainsAny(l.MarkedText(), "89") {
			return gast.Token{}, l.fail("malformed octal literal")
		}
		return l.numberEnd()
	}
	if next, ok := l.Peek(0); ok && next == '.' {
		l.Advance()
		if l.CheckDigit() {
			l.Advance()
			if err := l.scanDigits(); err != nil {
				return gast.Token{}, err
			}
		}
	}
	if next, ok := l.Peek(0); ok && (next == 'e' || next == 'E') {
		l.Advance()
		if next, ok := l.Peek(0); ok && (next == '+' || next == '-') {
			l.Advance()
		}
		if !l.CheckDigit() {
			return gast.Token{}, l.fail("expected digits in exponent")
		}
		l.Advance()
		if err := l.scanDigits(); err != nil {
			return gast.Token{}, err
		}
	}
	if err := l.checkNumberEnd(); err != nil {
		return gast.Token{}, err
	}
	return l.token(KindFloat), nil
}

// scanDigits consumes a digit run where every underscore sits between
// digits. The first digit is already consumed by the caller.
func (l *lexer) scanDigits() error {
	for {
		if next, ok := l.Peek(0); ok && isDigit(next) {
			l.Advance()
			continue
		}
		if !l.Check('_') {
			return nil
		}
		if next, ok := l.Peek(1); !ok || !isDigit(next) {
			return l.fail("unexpected '_' in number")
		}
		l.Advance()
	}
}

// checkNumberEnd rejects a trailing identifier start, unless it spells
// a keyword: 1abc is an error while 1and 2 lexes as two tokens.
func (l *lexer) checkNumberEnd() error {
	next, ok := l.Peek(0)
	if !ok || !isIdentStart(next) {
		return nil
	}
	identStart := l.Offset()
	l.Advance()
	l.SkipWhile(isIdentByte)
	word := strings.ToLower(l.Source()[identStart:l.Offset()])
	l.SetOffset(identStart)
	if _, isKeyword := keywords[word]; isKeyword {
		return nil
	}
	return l.fail("unexpected character in number")
}

func (l *lexer) numberEnd() (gast.Token, error) {
	if err := l.checkNumberEnd(); err != nil {
		return gast.Token{}, err
	}
	return l.token(KindInt), nil
}

// CheckDigit reports whether the next byte is a decimal digit.
func (l *lexer) CheckDigit() bool {
	next, ok := l.Peek(0)
	return ok && isDigit(next)
}

// scanCast recognizes (int)-style casts. Anything else resets so the
// caller emits a plain "(" token.
func (l *lexer) scanCast() (gast.Token, bool) {
	saved := l.Offset()
	l.Advance()
	start := l.Offset()
	l.SkipWhile(isIdentByte)
	word := l.Source()[start:l.Offset()]
	if word != "" && l.Check(')') {
		if _, ok := casts[strings.ToLower(word)]; ok {
			l.Advance()
			return l.token(KindCast), true
		}
	}
	l.SetOffset(saved)
	return gast.Token{}, false
}

// scanQuoted scans single-quoted, double-quoted, and shell-exec
// literals. In double-quoted text only \u{...} escapes are validated;
// unknown escapes are literal, matching PHP.
func (l *lexer) scanQuoted(quote byte, escapes bool) (gast.Token, error) {
	l.Advance()
	for {
		b, ok := l.Advance()
		if !ok {
			return gast.Token{}, l.fail("unterminated string")
		}
		switch b {
		case quote:
			return l.token(KindString), nil
		case '\\':
			next, ok := l.Advance()
			if !ok {
				return gast.Token{}, l.fail("unterminated string")
			}
			if escapes && next == 'u' && l.Check('{') {
				if err := l.scanUnicodeEscape(); err != nil {
					return gast.Token{}, err
				}
			}
		}
	}
}

func (l *lexer) scanUnicodeEscape() error {
	l.Advance()
	digits := 0
	for {
		b, ok := l.Advance()
		if !ok {
			return l.fail("unterminated unicode escape")
		}
		if b == '}' {
			if digits == 0 {
				return l.fail("invalid unicode escape")
			}
			return nil
		}
		if !isHexDigit(b) {
			return l.fail("invalid unicode escape")
		}
		digits++
	}
}

func (l *lexer) scanHeredoc() (gast.Token, error) {
	if !l.matchOp(HeredocStart) {
		return gast.Token{}, l.fail("expected '<<<'")
	}
	l.SkipWhile(func(b byte) bool { return b == ' ' || b == '\t' })
	quote := byte(0)
	if l.Check('"') || l.Check('\'') {
		quote, _ = l.Peek(0)
		l.Advance()
	}
	start := l.Offset()
	l.SkipWhile(isIdentByte)
	identifier := l.Source()[start:l.Offset()]
	if identifier == "" {
		return gast.Token{}, l.fail("expected heredoc identifier")
	}
	if quote != 0 && !l.Match(quote) {
		return gast.Token{}, l.fail("expected closing quote after heredoc identifier")
	}
	l.SkipWhile(func(b byte) bool { return b == ' ' || b == '\t' })
	l.Match('\r')
	if !l.Match('\n') {
		return gast.Token{}, l.fail("expected newline after heredoc identifier")
	}
	for {
		if l.AtEnd() {
			return gast.Token{}, l.fail("unterminated heredoc")
		}
		saved := l.Offset()
		l.SkipWhile(func(b byte) bool { return b == ' ' || b == '\t' })
		if l.MatchWord(identifier) {
			if next, ok := l.Peek(0); !ok || !isIdentByte(next) {
				return l.token(KindString), nil
			}
		}
		l.SetOffset(saved)
		for !l.AtEnd() && !l.Check('\n') {
			l.Advance()
		}
		if !l.AtEnd() {
			l.Advance()
		}
	}
}

// operators lists every operator longest-first; the scan walks it in
// order and emits the first match. The table is derived from the kind
// constants so spellings cannot drift.
var operators = []string{
	Identical, NotIdentical, PowAssign, ShiftLeftAssign, ShiftRightAssign,
	CoalesceAssign, NullsafeArrow, HeredocStart, Spaceship,
	ShiftLeft, ShiftRight, Equal, NotEqual, AngleNotEqual, LtEqual, GtEqual,
	LogicalAnd, LogicalOr, Incr, Decr, Arrow, DoubleArrow, DoubleColon,
	AddAssign, SubAssign, MulAssign, DivAssign, ModAssign, ConcatAssign,
	BitAndAssign, BitOrAssign, BitXorAssign, Pow, Coalesce, Ellipsis,
	CloseTagText,
	Plus, Minus, Asterisk, Slash, Percent, Dot, Assign, Lt, Gt, Bang,
	Question, Colon, Semicolon, Comma, LParen, RParen, LBracket, RBracket,
	LBrace, RBrace, Ampersand, Pipe, Caret, Tilde, At, Backslash,
}

// scanOperator matches the longest operator at the cursor. The close
// tag leaves php mode and reports the close_tag kind.
func (l *lexer) scanOperator() (gast.Token, error) {
	for _, op := range operators {
		if l.matchOp(op) {
			if op == CloseTagText {
				l.inPHP = false
				return gast.Token{Kind: KindCloseTag, Text: l.MarkedText(), Span: l.Marked()}, nil
			}
			return l.token(op), nil
		}
	}
	l.Advance()
	return gast.Token{}, l.fail("unexpected character")
}

// tokenizeInterpolation lexes source from offset in php mode, stopping
// before the first unmatched '}', which is left unconsumed. It returns
// the tokens (EOF excluded) and the offset of that '}', or -1 when the
// input ends first. Used to parse the expression inside {$...}
// interpolation, where braces, strings, and comments nest.
func tokenizeInterpolation(source string, offset int) ([]gast.Token, int, error) {
	l := newLexer(source)
	l.inPHP = true
	l.SetOffset(offset)
	var tokens []gast.Token
	depth := 0
	for {
		token, err := l.Next()
		if err != nil {
			return nil, 0, err
		}
		switch token.Kind {
		case KindEOF:
			return nil, -1, nil
		case LBrace:
			depth++
		case RBrace:
			if depth == 0 {
				return tokens, token.Span.Start, nil
			}
			depth--
		}
		tokens = append(tokens, token)
	}
}

func (l *lexer) matchOp(op string) bool {
	if !strings.HasPrefix(l.Source()[l.Offset():], op) {
		return false
	}
	l.SetOffset(l.Offset() + len(op))
	return true
}

func (l *lexer) checkOp(op string) bool {
	return strings.HasPrefix(l.Source()[l.Offset():], op)
}

func isIdentStart(b byte) bool {
	return b == '_' || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || b >= 0x80
}

func isIdentByte(b byte) bool {
	return isIdentStart(b) || isDigit(b)
}

func isDigit(b byte) bool {
	return '0' <= b && b <= '9'
}

func isHexDigit(b byte) bool {
	return isDigit(b) || ('a' <= b && b <= 'f') || ('A' <= b && b <= 'F')
}

func isBinaryDigit(b byte) bool {
	return b == '0' || b == '1'
}

func isOctalDigit(b byte) bool {
	return '0' <= b && b <= '7'
}

func (l *lexer) checkDigitAt(ahead int) bool {
	next, ok := l.Peek(ahead)
	return ok && isDigit(next)
}
