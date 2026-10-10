package gast

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// LexError reports invalid input during tokenization.
type LexError struct {
	Message string
	Pos     Span
}

func (e *LexError) Error() string { return e.Message }

// Span implements Spanned.
func (e *LexError) Span() Span { return e.Pos }

// ParseError reports a token sequence that does not match the grammar.
type ParseError struct {
	Message string
	Pos     Span
}

func (e *ParseError) Error() string { return e.Message }

// Span implements Spanned.
func (e *ParseError) Span() Span { return e.Pos }

// Spanned is implemented by errors that point at a source range.
type Spanned interface {
	error
	Span() Span
}

// FormatError renders err as a position, message, and caret line.
func FormatError(err error, source string) string {
	var spanned Spanned
	if !errors.As(err, &spanned) {
		return err.Error()
	}
	m := NewSourceMap(source)
	pos := m.PositionAt(spanned.Span().Start)
	line := m.LineAt(spanned.Span().Start)
	name := strings.TrimPrefix(fmt.Sprintf("%T", err), "*")
	if index := strings.LastIndexByte(name, '.'); index >= 0 {
		name = name[index+1:]
	}
	gutter := strings.Repeat(" ", len(strconv.Itoa(line.Number)))
	return strings.Join([]string{
		name + " at " + strconv.Itoa(pos.Line) + ":" + strconv.Itoa(pos.Column) + ": " + err.Error(),
		strconv.Itoa(line.Number) + " | " + line.Text,
		gutter + " | " + strings.Repeat(" ", pos.Column-1) + "^",
	}, "\n")
}
