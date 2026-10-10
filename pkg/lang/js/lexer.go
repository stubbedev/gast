package js

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/stubbedev/gast/pkg/gast"
)

// Lexer tokenizes JavaScript source in a single pass. It is exported so
// sibling frontends (TypeScript) can embed and extend it.
type Lexer struct {
	*gast.Cursor
	prev          gast.Token // last significant token, for regex-vs-division
	templateStack []int      // brace depth per open template substitution
	scanned       bool       // whether the first token was produced
}

// NewLexer returns a lexer at the start of source.
func NewLexer(source string) *Lexer {
	return &Lexer{Cursor: gast.NewCursor(source)}
}

// Tokenize scans source into tokens, ending with an EOF token.
func Tokenize(source string) ([]gast.Token, error) {
	l := NewLexer(source)
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

func (l *Lexer) eofToken() gast.Token {
	offset := l.Offset()
	return gast.Token{Kind: KindEOF, Span: gast.Span{Start: offset, End: offset}}
}

func (l *Lexer) token(kind string) gast.Token {
	return gast.Token{Kind: kind, Text: l.MarkedText(), Span: l.Marked()}
}

func (l *Lexer) fail(message string) error {
	return l.Fail(message)
}

// Next returns the next significant token. Whitespace and comments are
// skipped; lexical errors surface as *gast.LexError.
func (l *Lexer) Next() (gast.Token, error) {
	token, err := l.next()
	if err == nil && token.Kind != KindEOF {
		l.prev = token
	}
	return token, err
}

func (l *Lexer) next() (gast.Token, error) {
	if !l.scanned {
		l.scanned = true
		if l.Check('#') && l.CheckAhead(1, '!') {
			return l.scanHashbang(), nil
		}
	}
	if err := l.skipTrivia(); err != nil {
		return gast.Token{}, err
	}
	if l.AtEnd() {
		if len(l.templateStack) > 0 {
			return gast.Token{}, l.fail("unterminated template literal")
		}
		return l.eofToken(), nil
	}
	// A "}" closing a template substitution resumes template
	// scanning instead of emitting a brace token.
	if l.Check('}') && len(l.templateStack) > 0 && l.templateStack[len(l.templateStack)-1] == 0 {
		l.templateStack = l.templateStack[:len(l.templateStack)-1]
		l.StartToken()
		return l.scanTemplate(false)
	}
	l.StartToken()
	b, _ := l.Peek(0)
	return l.scanToken(b)
}

// skipTrivia consumes whitespace and comments. Line and block comments
// may contain any bytes except their terminators.
func (l *Lexer) skipTrivia() error {
	for {
		b, ok := l.Peek(0)
		if !ok {
			return nil
		}
		switch b {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			l.Advance()
		case '/':
			if l.CheckAhead(1, '/') {
				l.SkipWhile(func(b byte) bool { return b != '\n' && b != '\r' })
				continue
			}
			if l.CheckAhead(1, '*') {
				if err := l.skipBlockComment(); err != nil {
					return err
				}
				continue
			}
			return nil
		case 0xE2: // U+2028 LINE SEPARATOR and U+2029 PARAGRAPH SEPARATOR
			if l.CheckAhead(1, 0x80) {
				if next, ok := l.Peek(2); ok && (next == 0xA8 || next == 0xA9) {
					l.Advance()
					l.Advance()
					l.Advance()
					continue
				}
			}
			return nil
		default:
			return nil
		}
	}
}

func (l *Lexer) skipBlockComment() error {
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

func (l *Lexer) scanHashbang() gast.Token {
	l.SkipWhile(func(b byte) bool { return b != '\n' && b != '\r' })
	return l.token(KindHashbang)
}

func (l *Lexer) scanToken(b byte) (gast.Token, error) {
	switch {
	case isIdentStart(b):
		return l.scanIdent()
	case isDigit(b) || (b == '.' && l.checkDigitAt(1)):
		return l.scanNumber()
	case b == '\'' || b == '"':
		return l.scanString(b)
	case b == '`':
		return l.scanTemplate(true)
	case b == '#':
		return l.scanPrivateName()
	case b == '/' && !endsExpression(l.prev.Kind, l.prev.Text):
		return l.scanRegex()
	}
	return l.scanOperator()
}

// scanIdent consumes an identifier; multi-byte sequences are decoded as
// runes and may be letters, digits, or combining marks.
func (l *Lexer) scanIdent() (gast.Token, error) {
	for !l.AtEnd() {
		b, _ := l.Peek(0)
		if b < 0x80 {
			if !isASCIIIdentByte(b) {
				break
			}
			l.Advance()
			continue
		}
		r, size := utf8.DecodeRuneInString(l.Source()[l.Offset():])
		if !identRune(r) {
			break
		}
		l.SetOffset(l.Offset() + size)
	}
	if kind, ok := keywords[l.MarkedText()]; ok {
		return l.token(kind), nil
	}
	return l.token(KindIdent), nil
}

// identRune reports whether a non-ASCII rune may appear in an
// identifier.
func identRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r)
}

// scanPrivateName consumes #name. A "#" not followed by a name start is
// an error.
func (l *Lexer) scanPrivateName() (gast.Token, error) {
	l.Advance()
	if next, ok := l.Peek(0); !ok || !isIdentStart(next) {
		return gast.Token{}, l.fail("expected name after '#'")
	}
	l.SkipWhile(isIdentByte)
	return l.token(KindPrivateName), nil
}

// scanNumber consumes decimal literals with separators, exponents, and
// the BigInt n suffix. Legacy octal 0NNN is accepted (sloppy-mode
// policy); a run containing 8 or 9 is a decimal number.
func (l *Lexer) scanNumber() (gast.Token, error) {
	if l.Check('0') {
		radix, _ := l.Peek(1)
		switch radix {
		case 'x', 'X':
			return l.scanRadix(isHexDigit)
		case 'o', 'O':
			return l.scanRadix(isOctalDigit)
		case 'b', 'B':
			return l.scanRadix(isBinaryDigit)
		case '.', 'e', 'E', 'n':
			// 0.5, 0e1, and 0n continue below
		default:
			if isDigit(radix) || radix == '_' {
				return l.scanLegacyOctal()
			}
		}
	}
	l.Advance()
	if err := l.scanDigits(); err != nil {
		return gast.Token{}, err
	}
	if l.Check('n') {
		l.Advance()
		return l.numberEnd(KindBigInt)
	}
	if l.Match('.') {
		if l.CheckDigitByte() {
			l.Advance()
			if err := l.scanDigits(); err != nil {
				return gast.Token{}, err
			}
		}
	}
	if l.Check('e') || l.Check('E') {
		l.Advance()
		if l.Check('+') || l.Check('-') {
			l.Advance()
		}
		if !l.CheckDigitByte() {
			return gast.Token{}, l.fail("expected digits in exponent")
		}
		l.Advance()
		if err := l.scanDigits(); err != nil {
			return gast.Token{}, err
		}
	}
	return l.numberEnd(KindNum)
}

// scanRadix consumes 0x, 0o, and 0b literals, with separators and the
// BigInt suffix. The first digit must appear before any underscore.
func (l *Lexer) scanRadix(digit func(byte) bool) (gast.Token, error) {
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
			break
		}
		if next, ok := l.Peek(1); !ok || !digit(next) {
			return gast.Token{}, l.fail("unexpected '_' in number")
		}
		l.Advance()
	}
	if l.Match('n') {
		return l.numberEnd(KindBigInt)
	}
	return l.numberEnd(KindNum)
}

func (l *Lexer) takeRadixDigit(digit func(byte) bool) bool {
	if next, ok := l.Peek(0); ok && digit(next) {
		l.Advance()
		return true
	}
	return false
}

// scanLegacyOctal consumes a 0-prefixed digit run: digits 0-7 form a
// legacy octal (allowed, sloppy-mode policy), an 8 or 9 turns the
// literal decimal, separators are rejected, and a trailing fraction or
// exponent makes it a float.
func (l *Lexer) scanLegacyOctal() (gast.Token, error) {
	l.Advance()
	for {
		next, ok := l.Peek(0)
		if !ok || !isDigit(next) {
			break
		}
		l.Advance()
	}
	if l.Check('_') {
		return gast.Token{}, l.fail("numeric separator not allowed after leading 0")
	}
	if l.Check('n') {
		return gast.Token{}, l.fail("invalid BigInt literal")
	}
	if l.Match('.') {
		if l.CheckDigitByte() {
			l.Advance()
			if err := l.scanDigits(); err != nil {
				return gast.Token{}, err
			}
		}
	}
	if l.Check('e') || l.Check('E') {
		l.Advance()
		if l.Check('+') || l.Check('-') {
			l.Advance()
		}
		if !l.CheckDigitByte() {
			return gast.Token{}, l.fail("expected digits in exponent")
		}
		l.Advance()
		if err := l.scanDigits(); err != nil {
			return gast.Token{}, err
		}
	}
	return l.numberEnd(KindNum)
}

// scanDigits consumes a digit run where every underscore sits between
// digits. The first digit is already consumed by the caller.
func (l *Lexer) scanDigits() error {
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

// numberEnd rejects an identifier glued to a number (1abc) and emits
// the token.
func (l *Lexer) numberEnd(kind string) (gast.Token, error) {
	if next, ok := l.Peek(0); ok && isIdentStart(next) {
		return gast.Token{}, l.fail("unexpected character in number")
	}
	return l.token(kind), nil
}

// scanString consumes a quoted literal. Every escape is consumed; \x,
// \u, and \u{...} escapes are validated. A raw line terminator inside
// the literal is an error.
func (l *Lexer) scanString(quote byte) (gast.Token, error) {
	l.Advance()
	for {
		b, ok := l.Advance()
		if !ok {
			return gast.Token{}, l.fail("unterminated string")
		}
		switch b {
		case quote:
			return l.token(KindString), nil
		case '\n', '\r':
			return gast.Token{}, l.fail("unterminated string")
		case '\\':
			if err := l.scanEscape(); err != nil {
				return gast.Token{}, err
			}
		}
	}
}

func (l *Lexer) scanEscape() error {
	b, ok := l.Advance()
	if !ok {
		return l.fail("unterminated string")
	}
	switch b {
	case '\n', '\r':
		return nil // line continuation
	case 'x':
		for range 2 {
			next, ok := l.Peek(0)
			if !ok || !isHexDigit(next) {
				return l.fail("invalid \\x escape")
			}
			l.Advance()
		}
	case 'u':
		if l.Match('{') {
			digits, value := 0, 0
			for {
				next, ok := l.Advance()
				if !ok {
					return l.fail("unterminated \\u escape")
				}
				if next == '}' {
					if digits == 0 {
						return l.fail("invalid \\u escape")
					}
					return nil
				}
				if !isHexDigit(next) {
					return l.fail("invalid \\u escape")
				}
				value = value*16 + hexValue(next)
				digits++
				if value > 0x10FFFF {
					return l.fail("invalid \\u escape")
				}
			}
		}
		for range 4 {
			next, ok := l.Peek(0)
			if !ok || !isHexDigit(next) {
				return l.fail("invalid \\u escape")
			}
			l.Advance()
		}
	}
	return nil
}

// scanTemplate consumes a template literal or substitution tail. When
// fromBacktick is set the token starts at a backtick; otherwise it
// starts at the "}" that closes a substitution. Head and middle tokens
// push a brace counter that the main loop tracks.
func (l *Lexer) scanTemplate(fromBacktick bool) (gast.Token, error) {
	l.Advance()
	for {
		b, ok := l.Advance()
		if !ok {
			return gast.Token{}, l.fail("unterminated template literal")
		}
		switch b {
		case '`':
			if fromBacktick {
				return l.token(KindNoSubTemplate), nil
			}
			return l.token(KindTemplateTail), nil
		case '\\':
			if _, ok := l.Advance(); !ok {
				return gast.Token{}, l.fail("unterminated template literal")
			}
		case '$':
			if l.Check('{') {
				l.Advance()
				l.templateStack = append(l.templateStack, 0)
				if fromBacktick {
					return l.token(KindTemplateHead), nil
				}
				return l.token(KindTemplateMiddle), nil
			}
		}
	}
}

// scanRegex consumes a regex literal. A line terminator or the end of
// input before the closing slash is an error; "/" inside a character
// class does not close the literal.
func (l *Lexer) scanRegex() (gast.Token, error) {
	l.Advance()
	inClass := false
	for {
		b, ok := l.Advance()
		if !ok {
			return gast.Token{}, l.fail("unterminated regular expression")
		}
		switch b {
		case '\n', '\r':
			return gast.Token{}, l.fail("unterminated regular expression")
		case '\\':
			next, ok := l.Advance()
			if !ok || next == '\n' || next == '\r' {
				return gast.Token{}, l.fail("unterminated regular expression")
			}
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '/':
			if !inClass {
				l.SkipWhile(isASCIIIdentByte)
				return l.token(KindRegex), nil
			}
		}
	}
}

// operators lists every operator longest-first; the scan walks it in
// order and emits the first match. The table is derived from the kind
// constants so spellings cannot drift.
var operators = []string{
	UnsignedShiftAssign, Ellipsis, StrictEqual, StrictNotEqual, PowAssign,
	ShiftLeftAssign, ShiftRightAssign, UnsignedShift, LogicalAndAssign,
	LogicalOrAssign, NullishAssign,
	Arrow, Incr, Decr, ShiftLeft, ShiftRight, LtEqual, GtEqual,
	LogicalAnd, LogicalOr, Nullish, Equal, NotEqual, AddAssign, SubAssign,
	MulAssign, DivAssign, ModAssign, BitAndAssign, BitOrAssign, BitXorAssign,
	Pow,
	LParen, RParen, LBrace, RBrace, LBracket, RBracket, Semicolon, Colon,
	Comma, Dot, Question, Bang, Tilde, Plus, Minus, Asterisk, Slash, Percent,
	Lt, Gt, Assign, Ampersand, Pipe, Caret, At,
}

// scanOperator matches the longest operator at the cursor. A "{"
// inside a template substitution is counted; anything unmatched is an
// error.
func (l *Lexer) scanOperator() (gast.Token, error) {
	if l.Check('?') && l.CheckAhead(1, '.') && !l.checkDigitAt(2) {
		l.Advance()
		l.Advance()
		return l.token(OptChain), nil
	}
	for _, op := range operators {
		if l.matchOp(op) {
			switch op {
			case LBrace:
				if len(l.templateStack) > 0 {
					l.templateStack[len(l.templateStack)-1]++
				}
			case RBrace:
				if len(l.templateStack) > 0 {
					l.templateStack[len(l.templateStack)-1]--
				}
			}
			return l.token(op), nil
		}
	}
	l.Advance()
	return gast.Token{}, l.fail("unexpected character")
}

func (l *Lexer) matchOp(op string) bool {
	if !strings.HasPrefix(l.Source()[l.Offset():], op) {
		return false
	}
	l.SetOffset(l.Offset() + len(op))
	return true
}

func isIdentStart(b byte) bool {
	return b == '$' || b == '_' || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || b >= 0x80
}

// isIdentByte extends isIdentStart with ASCII digits; the multi-byte
// part is approximate and refined by identRune in scanIdent.
func isIdentByte(b byte) bool {
	return isIdentStart(b) || isDigit(b)
}

func isASCIIIdentByte(b byte) bool {
	return b == '$' || b == '_' || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || isDigit(b)
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

// hexValue returns the value of a hex digit byte, or -1.
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

func isOctalDigit(b byte) bool {
	return '0' <= b && b <= '7'
}

func (l *Lexer) CheckDigitByte() bool {
	next, ok := l.Peek(0)
	return ok && isDigit(next)
}

func (l *Lexer) checkDigitAt(ahead int) bool {
	next, ok := l.Peek(ahead)
	return ok && isDigit(next)
}
