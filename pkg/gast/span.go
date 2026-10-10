// Package gast provides the shared primitives for language frontends:
// spans, source positions, tokens, and the lexer and parser cursors that
// hand-written frontends are built on.
package gast

import "strings"

// Span is a half-open range of byte offsets into a source string.
type Span struct {
	Start int
	End   int
}

// Pos returns the span itself so that types embedding Span satisfy
// position-accessor interfaces through promotion.
func (s Span) Pos() Span { return s }

// Merge returns the smallest span covering both a and b.
func Merge(a, b Span) Span {
	return Span{Start: min(a.Start, b.Start), End: max(a.End, b.End)}
}

// Position is a 1-based line and column for a byte offset.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Line is a numbered line of source text.
type Line struct {
	Number int
	Text   string
}

// SourceMap resolves byte offsets to line and column positions.
type SourceMap struct {
	source     string
	lineStarts []int
}

// NewSourceMap indexes the line starts of source.
func NewSourceMap(source string) *SourceMap {
	lineStarts := []int{0}
	for i := range len(source) {
		if source[i] == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	return &SourceMap{source: source, lineStarts: lineStarts}
}

// PositionAt returns the 1-based line and column containing offset.
func (m *SourceMap) PositionAt(offset int) Position {
	line := m.lineIndexAt(offset)
	return Position{Offset: offset, Line: line + 1, Column: offset - m.lineStarts[line] + 1}
}

// LineAt returns the line of source text containing offset.
func (m *SourceMap) LineAt(offset int) Line {
	index := m.lineIndexAt(offset)
	start := m.lineStarts[index]
	end := len(m.source)
	if index+1 < len(m.lineStarts) {
		end = m.lineStarts[index+1] - 1
	}
	return Line{Number: index + 1, Text: strings.TrimSuffix(m.source[start:end], "\r")}
}

func (m *SourceMap) lineIndexAt(offset int) int {
	index := 0
	for candidate, start := range m.lineStarts {
		if start > offset {
			break
		}
		index = candidate
	}
	return index
}
