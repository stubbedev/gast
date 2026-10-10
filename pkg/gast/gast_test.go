package gast_test

import (
	"errors"
	"testing"

	"github.com/stubbedev/gast/pkg/gast"
)

func TestMerge(t *testing.T) {
	t.Parallel()
	a := gast.Span{Start: 5, End: 8}
	b := gast.Span{Start: 2, End: 6}
	merged := gast.Merge(a, b)
	if merged.Start != 2 || merged.End != 8 {
		t.Errorf("Merge: got %v", merged)
	}
}

func TestPos(t *testing.T) {
	t.Parallel()
	span := gast.Span{Start: 1, End: 2}
	if span.Pos() != span {
		t.Errorf("embedded span Pos: %v", span.Pos())
	}
}

func TestSourceMapPositions(t *testing.T) {
	t.Parallel()
	m := gast.NewSourceMap("ab\ncd\r\nef")
	tests := []struct {
		offset       int
		line, column int
	}{
		{0, 1, 1},
		{1, 1, 2},
		{2, 1, 3},
		{3, 2, 1},
		{6, 2, 4},
		{7, 3, 1},
		{9, 3, 3},
		{20, 3, 14},
	}
	for _, tc := range tests {
		pos := m.PositionAt(tc.offset)
		if pos.Line != tc.line || pos.Column != tc.column {
			t.Errorf("PositionAt(%d): got %d:%d, want %d:%d", tc.offset, pos.Line, pos.Column, tc.line, tc.column)
		}
	}
}

func TestSourceMapLines(t *testing.T) {
	t.Parallel()
	m := gast.NewSourceMap("ab\ncd\r\nef")
	if line := m.LineAt(0); line.Number != 1 || line.Text != "ab" {
		t.Errorf("line 1: %#v", line)
	}
	if line := m.LineAt(4); line.Number != 2 || line.Text != "cd" {
		t.Errorf("line 2 with CR stripped: %#v", line)
	}
	if line := m.LineAt(7); line.Number != 3 || line.Text != "ef" {
		t.Errorf("line 3: %#v", line)
	}
}

func TestDescribe(t *testing.T) {
	t.Parallel()
	if got := gast.Describe(gast.Token{}); got != "end of input" {
		t.Errorf("empty token: %q", got)
	}
	if got := gast.Describe(gast.Token{Text: "a\nb"}); got != `'a\nb'` {
		t.Errorf("escaped text: %q", got)
	}
}

func TestFormatError(t *testing.T) {
	t.Parallel()
	source := "<?php\n$a = ;"
	err := &gast.ParseError{Message: "unexpected ';'", Pos: gast.Span{Start: 11, End: 12}}
	formatted := gast.FormatError(err, source)
	for _, want := range []string{
		"ParseError at 2:6: unexpected ';'",
		"2 | $a = ;",
		"  |      ^",
	} {
		if !contains(formatted, want) {
			t.Errorf("missing %q in:\n%s", want, formatted)
		}
	}

	plain := errors.New("boom")
	if got := gast.FormatError(plain, source); got != "boom" {
		t.Errorf("non-spanned error: %q", got)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
