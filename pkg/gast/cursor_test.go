package gast_test

import (
	"errors"
	"testing"

	"github.com/stubbedev/gast/pkg/gast"
)

func TestCursorBasics(t *testing.T) {
	t.Parallel()
	c := gast.NewCursor("ab c")
	if c.Offset() != 0 || !c.Check('a') || c.AtEnd() {
		t.Fatalf("initial state: offset %d", c.Offset())
	}
	if b, _ := c.Advance(); b != 'a' {
		t.Fatalf("advance: %q", b)
	}
	if !c.CheckAhead(1, ' ') {
		t.Error("CheckAhead")
	}
	if !c.Match('b') {
		t.Error("Match b")
	}
	if c.Match('x') {
		t.Error("Match should fail on x")
	}
	c.SkipWhitespace()
	if c.Offset() != 3 || c.AtEnd() || !c.Check('c') {
		t.Errorf("after skip: %d", c.Offset())
	}
}

func TestCursorPeekBounds(t *testing.T) {
	t.Parallel()
	c := gast.NewCursor("a")
	if _, ok := c.Peek(5); ok {
		t.Error("peek past end should fail")
	}
	if _, ok := c.Peek(-1); ok {
		t.Error("negative peek should fail")
	}
}

func TestCursorSetOffset(t *testing.T) {
	t.Parallel()
	c := gast.NewCursor("abcdef")
	c.SetOffset(4)
	if !c.Check('e') {
		t.Error("SetOffset")
	}
}

func TestCursorMatchWord(t *testing.T) {
	t.Parallel()
	tests := []struct {
		source string
		word   string
		match  bool
		offset int
	}{
		{"let x", "let", true, 3},
		{"letter", "let", false, 0},
		{"letx", "let", false, 0},
		{"leta", "let", false, 0},
		{"let", "let", true, 3},
		{"let2", "let", false, 0},
	}
	for _, tc := range tests {
		c := gast.NewCursor(tc.source)
		if got := c.MatchWord(tc.word); got != tc.match {
			t.Errorf("MatchWord(%q, %q): got %v, want %v", tc.source, tc.word, got, tc.match)
		}
		if c.Offset() != tc.offset {
			t.Errorf("MatchWord(%q, %q): offset %d, want %d", tc.source, tc.word, c.Offset(), tc.offset)
		}
	}
}

func TestCursorTokens(t *testing.T) {
	t.Parallel()
	c := gast.NewCursor("let x")
	c.StartToken()
	c.MatchWord("let")
	token := func(kind string) gast.Token {
		return gast.Token{Kind: kind, Text: c.MarkedText(), Span: c.Marked()}
	}
	first := token("keyword")
	if first.Text != "let" || first.Span != (gast.Span{Start: 0, End: 3}) {
		t.Errorf("first token: %#v", first)
	}
	c.SkipWhitespace()
	c.StartToken()
	c.Match('x')
	second := token("ident")
	if second.Text != "x" || second.Span != (gast.Span{Start: 4, End: 5}) {
		t.Errorf("second token: %#v", second)
	}
}

func TestCursorFail(t *testing.T) {
	t.Parallel()
	c := gast.NewCursor("a?")
	c.StartToken()
	c.Advance()
	err := c.Fail("unexpected")
	var lexErr *gast.LexError
	if !errors.As(err, &lexErr) {
		t.Fatalf("got %T", err)
	}
	if lexErr.Message != "unexpected" || lexErr.Pos != (gast.Span{Start: 0, End: 1}) {
		t.Errorf("error: %#v", lexErr)
	}
}

func TestTokenCursor(t *testing.T) {
	t.Parallel()
	tokens := []gast.Token{
		{Kind: "a", Text: "a", Span: gast.Span{Start: 0, End: 1}},
		{Kind: "b", Text: "b", Span: gast.Span{Start: 2, End: 3}},
	}
	p := gast.NewTokenCursor(tokens, "eof")

	if !p.Check("a") || !p.CheckAhead(1, "b") {
		t.Fatal("initial navigation")
	}
	if !p.Match("a") {
		t.Fatal("match a")
	}
	if p.Current().Kind != "b" {
		t.Fatalf("current after match: %q", p.Current().Kind)
	}
	span := p.SpanFrom(tokens[0].Span)
	if span != (gast.Span{Start: 0, End: 1}) {
		t.Errorf("SpanFrom: %v", span)
	}
	if !p.Match("b") || !p.AtEnd() {
		t.Fatal("consume b")
	}

	eof := p.Current()
	if eof.Kind != "eof" || eof.Span != (gast.Span{Start: 3, End: 3}) {
		t.Fatalf("synthetic eof: %#v", eof)
	}
	p.Advance()
	if p.Current().Kind != "eof" {
		t.Fatal("advance at end must stay at eof")
	}
	span = p.SpanFrom(tokens[0].Span)
	if span != (gast.Span{Start: 0, End: 3}) {
		t.Errorf("SpanFrom over all tokens: %v", span)
	}
}

func TestTokenCursorEmpty(t *testing.T) {
	t.Parallel()
	p := gast.NewTokenCursor(nil, "eof")
	eof := p.Current()
	if eof.Kind != "eof" || eof.Span != (gast.Span{Start: 0, End: 0}) {
		t.Fatalf("eof on empty input: %#v", eof)
	}
	if p.Advance().Kind != "eof" {
		t.Fatal("advance on empty")
	}
}

func TestTokenCursorExpect(t *testing.T) {
	t.Parallel()
	tokens := []gast.Token{{Kind: "b", Text: "b", Span: gast.Span{Start: 4, End: 5}}}
	p := gast.NewTokenCursor(tokens, "eof")
	_, err := p.Expect("a", "'a'")
	var parseErr *gast.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("got %T", err)
	}
	if parseErr.Message != "expected 'a', found 'b'" {
		t.Errorf("message: %q", parseErr.Message)
	}
	if parseErr.Pos != (gast.Span{Start: 4, End: 5}) {
		t.Errorf("span: %v", parseErr.Pos)
	}

	tok, err := p.Expect("b", "")
	if err != nil || tok.Kind != "b" {
		t.Errorf("matching expect: %v", err)
	}

	if _, err := p.Expect("a", ""); err == nil {
		t.Error("expected failure at end")
	} else if msg := err.Error(); msg != "expected 'a', found end of input" {
		t.Errorf("end of input message: %q", msg)
	}
}
