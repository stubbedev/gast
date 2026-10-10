package gast

import "strings"

// Cursor is a byte cursor over source text with token-span tracking.
// Hand-written lexers embed or wrap it and drive it from their scan loop.
type Cursor struct {
	source string
	pos    int
	start  int
}

// NewCursor returns a cursor at the start of source.
func NewCursor(source string) *Cursor {
	return &Cursor{source: source}
}

// Source returns the full input text.
func (c *Cursor) Source() string { return c.source }

// Offset returns the current byte offset.
func (c *Cursor) Offset() int { return c.pos }

// SetOffset rewinds or advances the cursor; used to backtrack after
// speculative lookahead.
func (c *Cursor) SetOffset(offset int) { c.pos = offset }

// AtEnd reports whether the cursor has consumed the input.
func (c *Cursor) AtEnd() bool { return c.pos >= len(c.source) }

// Peek returns the byte ahead positions from the cursor, or false past
// the end of input.
func (c *Cursor) Peek(ahead int) (byte, bool) {
	index := c.pos + ahead
	if index < 0 || index >= len(c.source) {
		return 0, false
	}
	return c.source[index], true
}

// Advance consumes and returns the next byte.
func (c *Cursor) Advance() (byte, bool) {
	b, ok := c.Peek(0)
	if ok {
		c.pos++
	}
	return b, ok
}

// Check reports whether the next byte is b.
func (c *Cursor) Check(b byte) bool {
	next, ok := c.Peek(0)
	return ok && next == b
}

// CheckAhead reports whether the byte ahead positions from the cursor
// is b.
func (c *Cursor) CheckAhead(ahead int, b byte) bool {
	next, ok := c.Peek(ahead)
	return ok && next == b
}

// Match consumes the next byte if it is b.
func (c *Cursor) Match(b byte) bool {
	if !c.Check(b) {
		return false
	}
	c.pos++
	return true
}

// MatchWord consumes word if it is at the cursor and is not followed by a
// word byte, so keyword matching never splits an identifier.
func (c *Cursor) MatchWord(word string) bool {
	if !strings.HasPrefix(c.source[c.pos:], word) {
		return false
	}
	after := c.pos + len(word)
	if after < len(c.source) && isWordByte(c.source[after]) {
		return false
	}
	c.pos = after
	return true
}

// SkipWhile consumes bytes while pred holds.
func (c *Cursor) SkipWhile(pred func(byte) bool) {
	for {
		b, ok := c.Peek(0)
		if !ok || !pred(b) {
			return
		}
		c.pos++
	}
}

// TakeWhile consumes bytes while pred holds and returns how many were
// consumed.
func (c *Cursor) TakeWhile(pred func(byte) bool) int {
	start := c.pos
	c.SkipWhile(pred)
	return c.pos - start
}

// SkipWhitespace consumes spaces, tabs, newlines, and carriage returns.
func (c *Cursor) SkipWhitespace() {
	c.SkipWhile(func(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' })
}

// StartToken marks the start of the next token.
func (c *Cursor) StartToken() { c.start = c.pos }

// Marked returns the span from the last StartToken to the cursor.
func (c *Cursor) Marked() Span { return Span{Start: c.start, End: c.pos} }

// MarkedText returns the source text covered by Marked.
func (c *Cursor) MarkedText() string { return c.source[c.start:c.pos] }

// Fail returns a LexError covering the marked span.
func (c *Cursor) Fail(message string) error {
	return &LexError{Message: message, Pos: c.Marked()}
}

func isWordByte(b byte) bool {
	return b == '_' || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9') || b >= 0x80
}
