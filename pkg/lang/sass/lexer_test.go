package sass

import (
	"testing"

	"github.com/stubbedev/gast/internal/sasscore"
	"github.com/stubbedev/gast/pkg/gast"
)

func kinds(source string) ([]string, error) {
	tokens, err := Tokenize(source)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		result = append(result, token.Kind)
	}
	return result, nil
}

func TestTokenizeIndentation(t *testing.T) {
	t.Parallel()
	got, err := kinds("a\n  b: c\n\nd: e\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		sasscore.KindIdent, sasscore.KindNewline,
		sasscore.KindIndent, sasscore.KindIdent, sasscore.Colon, sasscore.KindIdent,
		sasscore.KindNewline, sasscore.KindDedent,
		sasscore.KindIdent, sasscore.Colon, sasscore.KindIdent, sasscore.KindNewline,
		sasscore.KindEOF,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: got %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}

func TestTokenizeNestedIndentation(t *testing.T) {
	t.Parallel()
	got, err := kinds("a\n  b\n    c: 1\n  d: 2\ne: 3\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		sasscore.KindIdent, sasscore.KindNewline,
		sasscore.KindIndent,
		sasscore.KindIdent, sasscore.KindNewline,
		sasscore.KindIndent, sasscore.KindIdent, sasscore.Colon, sasscore.KindNumber,
		sasscore.KindNewline, sasscore.KindDedent,
		sasscore.KindIdent, sasscore.Colon, sasscore.KindNumber, sasscore.KindNewline,
		sasscore.KindDedent,
		sasscore.KindIdent, sasscore.Colon, sasscore.KindNumber, sasscore.KindNewline,
		sasscore.KindEOF,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTokenizeCommentOnlyLinesIgnored(t *testing.T) {
	t.Parallel()
	got, err := kinds("a\n  // silent note\n  /* loud note */\n  b: c\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		sasscore.KindIdent, sasscore.KindNewline,
		sasscore.KindIndent, sasscore.KindIdent, sasscore.Colon, sasscore.KindIdent,
		sasscore.KindNewline, sasscore.KindEOF,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTokenizeParenContinuation(t *testing.T) {
	t.Parallel()
	// A newline inside parens is plain whitespace, not a terminator.
	got, err := kinds("a\n  map: (one: 1,\n        two: 2)\n  b: c\n")
	if err != nil {
		t.Fatal(err)
	}
	newlines := 0
	parens := 0
	for _, kind := range got {
		switch kind {
		case sasscore.KindNewline:
			newlines++
		case sasscore.LParen, sasscore.RParen:
			parens++
		}
	}
	if parens != 2 {
		t.Errorf("paren tokens: %v", got)
	}
	if newlines != 3 { // after a, after the closing paren, after b: c
		t.Errorf("newline inside parens must be suppressed: %v", got)
	}
}

func TestTokenizeIndentErrors(t *testing.T) {
	t.Parallel()
	if _, err := kinds("a\n  b: c\n\tbottom: 0\n"); err == nil {
		t.Error("mixed tabs and spaces: expected error")
	} else if span, ok := err.(interface{ Span() gast.Span }); !ok || span.Span().Start == 0 {
		t.Errorf("error should point at the indentation: %v", err)
	}
	if _, err := kinds("a\n  b\n    c: 1\n   d: 2\n"); err == nil {
		t.Error("inconsistent dedent: expected error")
	}
	if _, err := kinds("a\n  content: \"open\n"); err == nil {
		t.Error("unclosed string: expected error")
	}
}
