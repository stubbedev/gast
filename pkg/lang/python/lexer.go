package python

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/stubbedev/gast/pkg/gast"
)

// Lex modes. The bottom of the mode stack is always the code mode;
// f-string scanning pushes text, expression, and format-spec modes per
// PEP 701, so nested quotes and nested f-strings need no backtracking.
const (
	modeCode  byte = 0
	modeFText byte = 't'
	modeFExpr byte = 'e'
	modeFSpec byte = 's'
)

// lexMode is one entry of the lexer mode stack.
type lexMode struct {
	kind   byte
	quote  byte
	triple bool
	raw    bool
	depth  int // bracket depth saved while an f-string owns the scan
}

// indentLevel is one level of the indentation stack: its column and
// the leading whitespace that produced it.
type indentLevel struct {
	column int
	ws     string
}

// lexer tokenizes Python source, tracking indentation with an
// INDENT/DEDENT stack, bracket nesting for implicit line joining, and
// the f-string mode stack.
type lexer struct {
	*gast.Cursor
	indents        []indentLevel
	atLineStart    bool
	depth          int
	openers        []byte
	pendingDedents int
	modes          []lexMode
}

func newLexer(source string) *lexer {
	return &lexer{
		Cursor:      gast.NewCursor(source),
		indents:     []indentLevel{{}},
		atLineStart: true,
		modes:       []lexMode{{kind: modeCode}},
	}
}

// Tokenize scans source into tokens, ending with an EOF token.
func Tokenize(source string) ([]gast.Token, error) {
	l := newLexer(source)
	tokens := make([]gast.Token, 0, 64)
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

func (l *lexer) mode() lexMode { return l.modes[len(l.modes)-1] }

func (l *lexer) token(kind string) gast.Token {
	return gast.Token{Kind: kind, Text: l.MarkedText(), Span: l.Marked()}
}

// Next returns the next token. Lexical errors surface as
// *gast.LexError.
func (l *lexer) Next() (gast.Token, error) {
	switch l.mode().kind {
	case modeFText:
		return l.scanFStringText()
	case modeFSpec:
		return l.scanFStringSpec()
	default:
		return l.nextCode()
	}
}

func (l *lexer) nextCode() (gast.Token, error) {
	for {
		if l.pendingDedents > 0 {
			l.pendingDedents--
			l.indents = l.indents[:len(l.indents)-1]
			return l.zeroWidthToken(KindDedent), nil
		}
		if l.AtEnd() {
			return l.atEOF()
		}
		if l.atLineStart {
			token, err := l.handleLineStart()
			if err != nil {
				return gast.Token{}, err
			}
			if token.Kind != "" {
				return token, nil
			}
			continue
		}
		b, _ := l.Peek(0)
		switch {
		case b == ' ' || b == '\t' || b == '\f':
			l.Advance()
		case b == '\n' || b == '\r':
			l.StartToken()
			l.skipEndOfLine()
			if l.mode().kind == modeFExpr {
				// PEP 701 allows newlines anywhere inside braces.
				continue
			}
			if l.depth == 0 {
				l.atLineStart = true
				return l.token(KindNewline), nil
			}
		case b == '\\' && l.peekEndOfLine(1):
			l.Advance()
			l.skipEndOfLine()
		case b == '#':
			l.skipComment()
		default:
			l.StartToken()
			return l.scanToken(b)
		}
	}
}

// atEOF emits the trailing NEWLINE and DEDENT tokens, then EOF,
// reporting unclosed brackets and f-strings.
func (l *lexer) atEOF() (gast.Token, error) {
	if l.mode().kind != modeCode {
		l.StartToken()
		return gast.Token{}, l.Fail("unterminated f-string literal")
	}
	if len(l.openers) > 0 {
		l.StartToken()
		return gast.Token{}, l.Fail("'" + string(l.openers[len(l.openers)-1]) + "' was never closed")
	}
	if !l.atLineStart {
		l.atLineStart = true
		return l.zeroWidthToken(KindNewline), nil
	}
	if len(l.indents) > 1 {
		l.indents = l.indents[:len(l.indents)-1]
		return l.zeroWidthToken(KindDedent), nil
	}
	offset := l.Offset()
	return gast.Token{Kind: KindEOF, Span: gast.Span{Start: offset, End: offset}}, nil
}

func (l *lexer) zeroWidthToken(kind string) gast.Token {
	offset := l.Offset()
	return gast.Token{Kind: kind, Span: gast.Span{Start: offset, End: offset}}
}

// handleLineStart measures the indentation of a line and emits INDENT
// or queues DEDENT tokens. Blank and comment-only lines produce
// nothing; an empty kind marks that the caller should keep scanning.
func (l *lexer) handleLineStart() (gast.Token, error) {
	l.StartToken()
	column, sawSpace, sawTab := 0, false, false
	for {
		b, ok := l.Peek(0)
		if !ok {
			break
		}
		switch b {
		case ' ':
			column++
			sawSpace = true
			l.Advance()
		case '\t':
			column += 8 - column%8
			sawTab = true
			l.Advance()
		case '\f':
			column = 0
			l.Advance()
		default:
			goto measured
		}
	}
measured:
	b, ok := l.Peek(0)
	if !ok || b == '\n' || b == '\r' {
		l.skipEndOfLine()
		return gast.Token{}, nil
	}
	if b == '#' {
		l.skipComment()
		l.skipEndOfLine()
		return gast.Token{}, nil
	}
	if b == '\\' && l.peekEndOfLine(1) {
		l.Advance()
		l.skipEndOfLine()
		return gast.Token{}, nil
	}
	if sawTab && sawSpace {
		return gast.Token{}, l.Fail("inconsistent use of tabs and spaces in indentation")
	}
	l.atLineStart = false
	ws := l.MarkedText()
	top := l.indents[len(l.indents)-1]
	switch {
	case column > top.column:
		if inconsistentIndent(top.ws, ws) {
			return gast.Token{}, l.Fail("inconsistent use of tabs and spaces in indentation")
		}
		l.indents = append(l.indents, indentLevel{column: column, ws: ws})
		return l.token(KindIndent), nil
	case column < top.column:
		pops := 0
		for len(l.indents)-pops > 1 && l.indents[len(l.indents)-1-pops].column > column {
			pops++
		}
		if l.indents[len(l.indents)-1-pops].column != column {
			return gast.Token{}, l.Fail("unindent does not match any outer indentation level")
		}
		if inconsistentIndent(l.indents[len(l.indents)-1-pops].ws, ws) {
			return gast.Token{}, l.Fail("inconsistent use of tabs and spaces in indentation")
		}
		l.pendingDedents = pops
	}
	return gast.Token{}, nil
}

// inconsistentIndent reports whether two indentation prefixes mix tab
// and space styles; the empty root prefix matches anything.
func inconsistentIndent(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return strings.ContainsRune(a, '\t') != strings.ContainsRune(b, '\t')
}

func (l *lexer) skipComment() {
	for {
		b, ok := l.Peek(0)
		if !ok || b == '\n' {
			return
		}
		if b == '\r' && l.CheckAhead(1, '\n') {
			return
		}
		l.Advance()
	}
}

// skipEndOfLine consumes one newline, treating \r\n as a single
// newline.
func (l *lexer) skipEndOfLine() {
	if l.Match('\r') {
		l.Match('\n')
		return
	}
	l.Match('\n')
}

func (l *lexer) peekEndOfLine(ahead int) bool {
	b, ok := l.Peek(ahead)
	if !ok {
		return false
	}
	if b == '\r' {
		next, hasNext := l.Peek(ahead + 1)
		return !hasNext || next == '\n'
	}
	return b == '\n'
}

func (l *lexer) scanToken(b byte) (gast.Token, error) {
	switch {
	case isIdentStart(b):
		return l.scanName()
	case isDigit(b):
		return l.scanNumber()
	case b == '"' || b == '\'':
		return l.scanString("")
	case b == '.' && l.checkDigitAt(1):
		return l.scanNumber()
	}
	if l.mode().kind == modeFExpr && l.depth == 0 {
		switch b {
		case '}':
			l.Advance()
			l.depth = l.popMode().depth
			return l.token(RBrace), nil
		case ':':
			l.Advance()
			l.modes[len(l.modes)-1].kind = modeFSpec
			return l.token(Colon), nil
		case '!':
			if conv, ok := l.peekConversion(); ok {
				l.Advance()
				l.Advance()
				return l.token(conv), nil
			}
		}
	}
	return l.scanOperator(b)
}

// peekConversion reports the conversion kind when the bytes ahead are
// !s, !r, or !a followed by the field terminator.
func (l *lexer) peekConversion() (string, bool) {
	b, ok := l.Peek(1)
	if !ok || (b != 's' && b != 'r' && b != 'a') {
		return "", false
	}
	terminator, ok := l.Peek(2)
	if !ok || (terminator != ':' && terminator != '}') {
		return "", false
	}
	return KindConversion, true
}

func isIdentStart(b byte) bool {
	return b == '_' || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || b >= 0x80
}

func isDigit(b byte) bool { return '0' <= b && b <= '9' }

// scanName consumes an identifier and resolves keywords and string
// prefixes. Unicode identifiers are approximated: any rune outside
// ASCII must be a letter, digit, or underscore continuation.
func (l *lexer) scanName() (gast.Token, error) {
	for !l.AtEnd() {
		b, _ := l.Peek(0)
		if b < 0x80 {
			if b == '_' || isIdentStart(b) || isDigit(b) {
				l.Advance()
				continue
			}
			break
		}
		r, size := utf8.DecodeRuneInString(l.Source()[l.Offset():])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		l.SetOffset(l.Offset() + size)
	}
	word := l.MarkedText()
	if kind, ok := keywords[word]; ok {
		return l.token(kind), nil
	}
	if next, ok := l.Peek(0); ok && (next == '"' || next == '\'') {
		return l.scanString(word)
	}
	return l.token(KindName), nil
}

// stringPrefix decodes a string prefix into its raw, bytes, and f
// flags. The bool reports whether the word is a valid prefix.
func stringPrefix(word string) (raw, isBytes, isF bool, ok bool) {
	for _, r := range word {
		switch r {
		case 'r', 'R':
			if raw {
				return false, false, false, false
			}
			raw = true
		case 'b', 'B':
			if isBytes || isF {
				return false, false, false, false
			}
			isBytes = true
		case 'f', 'F':
			if isF || isBytes {
				return false, false, false, false
			}
			isF = true
		case 'u', 'U':
			if raw || isBytes || isF || len(word) > 1 {
				return false, false, false, false
			}
		default:
			return false, false, false, false
		}
	}
	return raw, isBytes, isF, len(word) <= 2
}

// scanString scans a quoted literal whose prefix was already consumed.
// f-strings push the text mode and return an FStringStart token.
func (l *lexer) scanString(prefix string) (gast.Token, error) {
	raw, isBytes, isF, ok := stringPrefix(prefix)
	if !ok {
		return gast.Token{}, l.Fail("invalid string prefix")
	}
	quote, _ := l.Advance()
	triple := l.Check(quote) && l.CheckAhead(1, quote)
	if triple {
		l.Advance()
		l.Advance()
	}
	if isF {
		l.modes = append(l.modes, lexMode{
			kind: modeFText, quote: quote, triple: triple, raw: raw, depth: l.depth,
		})
		return l.token(KindFStringStart), nil
	}
	for {
		if l.AtEnd() {
			return gast.Token{}, l.Fail("unterminated string literal")
		}
		b, _ := l.Peek(0)
		switch b {
		case '\\':
			if next, ok := l.Peek(1); ok && (next == '\n' || next == '\r') && !triple {
				return gast.Token{}, l.Fail("unterminated string literal")
			}
			l.Advance()
			if !l.AtEnd() {
				l.Advance()
			}
		case '\n', '\r':
			if !triple {
				return gast.Token{}, l.Fail("unterminated string literal")
			}
			l.Advance()
		case quote:
			if triple {
				if l.CheckAhead(1, quote) && l.CheckAhead(2, quote) {
					l.Advance()
					l.Advance()
					l.Advance()
					return l.token(literalKind(isBytes)), nil
				}
				l.Advance()
				continue
			}
			l.Advance()
			return l.token(literalKind(isBytes)), nil
		default:
			l.Advance()
		}
	}
}

func literalKind(isBytes bool) string {
	if isBytes {
		return KindBytes
	}
	return KindString
}

func (l *lexer) popMode() lexMode {
	top := l.modes[len(l.modes)-1]
	l.modes = l.modes[:len(l.modes)-1]
	return top
}

// scanFStringText scans literal text of an f-string until a nested
// field, the closing quote, or an error.
func (l *lexer) scanFStringText() (gast.Token, error) {
	m := l.mode()
	l.StartToken()
	for {
		if l.AtEnd() {
			return gast.Token{}, l.Fail("unterminated f-string literal")
		}
		b, _ := l.Peek(0)
		switch b {
		case '{':
			if l.CheckAhead(1, '{') {
				l.Advance()
				l.Advance()
				continue
			}
			if text := l.MarkedText(); text != "" {
				return l.token(KindFStringMiddle), nil
			}
			l.Advance()
			l.modes = append(l.modes, lexMode{kind: modeFExpr, depth: l.depth, quote: m.quote, triple: m.triple, raw: m.raw})
			l.depth = 0
			return l.token(LBrace), nil
		case '}':
			if l.CheckAhead(1, '}') {
				l.Advance()
				l.Advance()
				continue
			}
			return gast.Token{}, l.Fail("f-string: single '}' is not allowed")
		case '\\':
			next, hasNext := l.Peek(1)
			if !hasNext || ((next == '\n' || next == '\r') && !m.triple) {
				return gast.Token{}, l.Fail("unterminated f-string literal")
			}
			// In raw f-strings only a backslash before the closing
			// quote consumes it; \{ still opens a field.
			l.Advance()
			if !m.raw || next == m.quote {
				l.Advance()
			}
		case '\n', '\r':
			if !m.triple {
				return gast.Token{}, l.Fail("unterminated f-string literal")
			}
			l.Advance()
		case m.quote:
			closing := !m.triple || (l.CheckAhead(1, m.quote) && l.CheckAhead(2, m.quote))
			if !closing {
				l.Advance()
				continue
			}
			if text := l.MarkedText(); text != "" {
				return l.token(KindFStringMiddle), nil
			}
			if m.triple {
				l.Advance()
				l.Advance()
				l.Advance()
			} else {
				l.Advance()
			}
			l.depth = l.popMode().depth
			return l.token(KindFStringEnd), nil
		default:
			l.Advance()
		}
	}
}

// scanFStringSpec scans format-spec text until a nested field or the
// closing brace of the interpolation.
func (l *lexer) scanFStringSpec() (gast.Token, error) {
	m := l.mode()
	l.StartToken()
	for {
		if l.AtEnd() {
			return gast.Token{}, l.Fail("unterminated f-string literal")
		}
		b, _ := l.Peek(0)
		switch b {
		case '{':
			if text := l.MarkedText(); text != "" {
				return l.token(KindFStringMiddle), nil
			}
			l.Advance()
			l.modes = append(l.modes, lexMode{kind: modeFExpr, depth: l.depth, quote: m.quote, triple: m.triple, raw: m.raw})
			l.depth = 0
			return l.token(LBrace), nil
		case '}':
			if text := l.MarkedText(); text != "" {
				return l.token(KindFStringMiddle), nil
			}
			l.Advance()
			l.popMode()
			return l.token(RBrace), nil
		case '\\':
			next, hasNext := l.Peek(1)
			if !hasNext || ((next == '\n' || next == '\r') && !m.triple) {
				return gast.Token{}, l.Fail("unterminated f-string literal")
			}
			// In raw f-strings only a backslash before the closing
			// quote consumes it; \{ still opens a field.
			l.Advance()
			if !m.raw || next == m.quote {
				l.Advance()
			}
		case '\n', '\r':
			if !m.triple {
				return gast.Token{}, l.Fail("unterminated f-string literal")
			}
			l.Advance()
		default:
			l.Advance()
		}
	}
}

// scanNumber scans int, float, and complex literals, including 0x, 0o,
// and 0b prefixes, underscores between digits, exponents, and the
// imaginary j suffix.
func (l *lexer) scanNumber() (gast.Token, error) {
	if l.Check('0') {
		if radix, ok := l.Peek(1); ok {
			switch radix {
			case 'x', 'X':
				return l.scanRadix(isHexDigit)
			case 'o', 'O':
				return l.scanRadix(isOctalDigit)
			case 'b', 'B':
				return l.scanRadix(isBinaryDigit)
			}
		}
	}
	isFloat := false
	start := l.Offset()
	if err := l.scanDigits(isDigit); err != nil {
		return gast.Token{}, err
	}
	if l.Check('.') {
		isFloat = true
		l.Advance()
		if err := l.scanDigits(isDigit); err != nil {
			return gast.Token{}, err
		}
	}
	if next, ok := l.Peek(0); ok && (next == 'e' || next == 'E') {
		if exp, ok := l.Peek(1); ok && (isDigit(exp) || exp == '+' || exp == '-') {
			isFloat = true
			l.Advance()
			l.Match('+')
			l.Match('-')
			if !l.CheckDigit() {
				return gast.Token{}, l.Fail("expected digits in exponent")
			}
			if err := l.scanDigits(isDigit); err != nil {
				return gast.Token{}, err
			}
		}
	}
	text := l.Source()[start:l.Offset()]
	if next, ok := l.Peek(0); ok && (next == 'j' || next == 'J') {
		l.Advance()
		if err := l.checkNumberEnd(); err != nil {
			return gast.Token{}, err
		}
		return l.token(KindComplex), nil
	}
	if !isFloat {
		if err := l.checkLeadingZeros(text); err != nil {
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

// checkLeadingZeros rejects decimal integer literals with redundant
// leading zeros; 0 and 0_0 stay valid.
func (l *lexer) checkLeadingZeros(text string) error {
	if len(text) < 2 || text[0] != '0' {
		return nil
	}
	for i := 1; i < len(text); i++ {
		if text[i] == '_' {
			continue
		}
		if text[i] != '0' {
			return l.Fail("leading zeros in decimal integer literals are not permitted")
		}
	}
	return nil
}

// scanRadix consumes 0x, 0o, or 0b literals; a digit (or an
// underscore between digits) must follow the prefix.
func (l *lexer) scanRadix(digit func(byte) bool) (gast.Token, error) {
	l.Advance()
	l.Advance()
	if !l.takeRadixDigit(digit) {
		if l.Check('_') {
			if next, ok := l.Peek(1); !ok || !digit(next) {
				return gast.Token{}, l.Fail("unexpected '_' in number")
			}
			l.Advance()
		} else {
			return gast.Token{}, l.Fail("expected digits after number prefix")
		}
	}
	for {
		if l.takeRadixDigit(digit) {
			continue
		}
		if !l.Check('_') {
			break
		}
		if next, ok := l.Peek(1); !ok || !digit(next) {
			return gast.Token{}, l.Fail("unexpected '_' in number")
		}
		l.Advance()
	}
	if err := l.checkNumberEnd(); err != nil {
		return gast.Token{}, err
	}
	return l.token(KindInt), nil
}

func (l *lexer) takeRadixDigit(digit func(byte) bool) bool {
	if next, ok := l.Peek(0); ok && digit(next) {
		l.Advance()
		return true
	}
	return false
}

// scanDigits consumes a digit run where every underscore sits between
// digits.
func (l *lexer) scanDigits(digit func(byte) bool) error {
	for {
		if next, ok := l.Peek(0); ok && digit(next) {
			l.Advance()
			continue
		}
		if !l.Check('_') {
			return nil
		}
		if next, ok := l.Peek(1); !ok || !digit(next) {
			return l.Fail("unexpected '_' in number")
		}
		l.Advance()
	}
}

// CheckDigit reports whether the next byte is a decimal digit.
func (l *lexer) CheckDigit() bool {
	next, ok := l.Peek(0)
	return ok && isDigit(next)
}

func (l *lexer) checkDigitAt(ahead int) bool {
	next, ok := l.Peek(ahead)
	return ok && isDigit(next)
}

// checkNumberEnd rejects a trailing identifier start, unless it spells
// a keyword: 1abc is an error while 1if x else 2 lexes as tokens. A
// '.' after a number that did not end in one is also invalid: 1.2.3,
// but 1..real keeps working.
func (l *lexer) checkNumberEnd() error {
	next, ok := l.Peek(0)
	if ok && next == '.' && !strings.HasSuffix(l.MarkedText(), ".") {
		return l.Fail("invalid decimal literal")
	}
	if !ok || !isIdentStart(next) {
		return nil
	}
	identStart := l.Offset()
	l.SkipWhile(func(b byte) bool { return b == '_' || isIdentStart(b) || isDigit(b) })
	word := l.Source()[identStart:l.Offset()]
	l.SetOffset(identStart)
	if _, isKeyword := keywords[word]; isKeyword {
		return nil
	}
	return l.Fail("invalid decimal literal")
}

func isHexDigit(b byte) bool {
	return isDigit(b) || ('a' <= b && b <= 'f') || ('A' <= b && b <= 'F')
}

func isOctalDigit(b byte) bool { return '0' <= b && b <= '7' }

func isBinaryDigit(b byte) bool { return b == '0' || b == '1' }

// scanOperator scans punctuation and operators, maintaining bracket
// depth for implicit line joining.
func (l *lexer) scanOperator(b byte) (gast.Token, error) {
	switch b {
	case '(', '[', '{':
		l.openers = append(l.openers, b)
		l.depth++
		l.Advance()
		return l.token(openKind(b)), nil
	case ')', ']', '}':
		if len(l.openers) > 0 {
			l.openers = l.openers[:len(l.openers)-1]
			l.depth--
		}
		l.Advance()
		return l.token(closeKind(b)), nil
	case '+':
		return l.loneOrAssign(Plus, AddAssign), nil
	case '-':
		if l.CheckAhead(1, '=') {
			return l.multiByte(SubAssign, 2), nil
		}
		if l.CheckAhead(1, '>') {
			return l.multiByte(Arrow, 2), nil
		}
		l.Advance()
		return l.token(Minus), nil
	case '*':
		if l.CheckAhead(1, '*') {
			if l.CheckAhead(2, '=') {
				return l.multiByte(PowAssign, 3), nil
			}
			return l.multiByte(DoubleStar, 2), nil
		}
		if l.CheckAhead(1, '=') {
			return l.multiByte(MulAssign, 2), nil
		}
		l.Advance()
		return l.token(Star), nil
	case '/':
		if l.CheckAhead(1, '/') {
			if l.CheckAhead(2, '=') {
				return l.multiByte(FloorDivAssign, 3), nil
			}
			return l.multiByte(DoubleSlash, 2), nil
		}
		if l.CheckAhead(1, '=') {
			return l.multiByte(DivAssign, 2), nil
		}
		l.Advance()
		return l.token(Slash), nil
	case '%':
		return l.loneOrAssign(Percent, ModAssign), nil
	case '@':
		return l.loneOrAssign(At, MatMulAssign), nil
	case '&':
		return l.loneOrAssign(Ampersand, BitAndAssign), nil
	case '|':
		return l.loneOrAssign(Pipe, BitOrAssign), nil
	case '^':
		return l.loneOrAssign(Caret, BitXorAssign), nil
	case '~':
		l.Advance()
		return l.token(Tilde), nil
	case '<':
		if l.CheckAhead(1, '<') {
			if l.CheckAhead(2, '=') {
				return l.multiByte(LeftShiftAssign, 3), nil
			}
			return l.multiByte(LeftShift, 2), nil
		}
		if l.CheckAhead(1, '=') {
			return l.multiByte(LessEqual, 2), nil
		}
		l.Advance()
		return l.token(Less), nil
	case '>':
		if l.CheckAhead(1, '>') {
			if l.CheckAhead(2, '=') {
				return l.multiByte(RightShiftAssign, 3), nil
			}
			return l.multiByte(RightShift, 2), nil
		}
		if l.CheckAhead(1, '=') {
			return l.multiByte(GreaterEqual, 2), nil
		}
		l.Advance()
		return l.token(Greater), nil
	case '=':
		if l.CheckAhead(1, '=') {
			return l.multiByte(EqualEqual, 2), nil
		}
		l.Advance()
		return l.token(Assign), nil
	case '!':
		if l.CheckAhead(1, '=') {
			return l.multiByte(NotEqual, 2), nil
		}
		return gast.Token{}, l.Fail("unexpected '!'")
	case ':':
		if l.CheckAhead(1, '=') {
			return l.multiByte(Walrus, 2), nil
		}
		l.Advance()
		return l.token(Colon), nil
	case '.':
		if l.CheckAhead(1, '.') && l.CheckAhead(2, '.') {
			return l.multiByte(Ellipsis, 3), nil
		}
		l.Advance()
		return l.token(Dot), nil
	case ',':
		l.Advance()
		return l.token(Comma), nil
	case ';':
		l.Advance()
		return l.token(Semicolon), nil
	case '\\':
		return gast.Token{}, l.Fail("unexpected character after line continuation character")
	}
	return gast.Token{}, l.Fail("unexpected character '" + string(b) + "'")
}

// loneOrAssign scans a one-byte operator or its two-byte assignment
// form.
func (l *lexer) loneOrAssign(lone, withAssign string) gast.Token {
	if l.CheckAhead(1, '=') {
		return l.multiByte(withAssign, 2)
	}
	l.Advance()
	return l.token(lone)
}

func (l *lexer) multiByte(kind string, width int) gast.Token {
	for range width {
		l.Advance()
	}
	return l.token(kind)
}

func openKind(b byte) string {
	switch b {
	case '(':
		return LParen
	case '[':
		return LBracket
	default:
		return LBrace
	}
}

func closeKind(b byte) string {
	switch b {
	case ')':
		return RParen
	case ']':
		return RBracket
	default:
		return RBrace
	}
}
