package css

import (
	"testing"

	"github.com/stubbedev/gast/internal/sasscore"
	"github.com/stubbedev/gast/pkg/gast"
)

func kinds(t *testing.T, source string) []string {
	t.Helper()
	tokens, err := Tokenize(source)
	if err != nil {
		t.Fatalf("%s: %v", source, err)
	}
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		result = append(result, token.Kind)
	}
	return result
}

func TestTokenizeValues(t *testing.T) {
	t.Parallel()
	for source, want := range map[string][]string{
		`div {}`:    {sasscore.KindIdent, sasscore.LBrace, sasscore.RBrace, sasscore.KindEOF},
		`#main {}`:  {sasscore.KindHashID, sasscore.LBrace, sasscore.RBrace, sasscore.KindEOF},
		`#123 {}`:   {sasscore.KindHash, sasscore.LBrace, sasscore.RBrace, sasscore.KindEOF},
		`@media {}`: {sasscore.KindAtKeyword, sasscore.LBrace, sasscore.RBrace, sasscore.KindEOF},
		`--gap: 0;`: {sasscore.KindIdent, sasscore.Colon, sasscore.KindNumber, sasscore.Semicolon, sasscore.KindEOF},
		`a { color: f(x); }`: {
			sasscore.KindIdent, sasscore.LBrace, sasscore.KindIdent, sasscore.Colon,
			sasscore.KindFunction, sasscore.KindIdent, sasscore.RParen, sasscore.Semicolon,
			sasscore.RBrace, sasscore.KindEOF,
		},
		`<!-- -->`: {sasscore.KindCDO, sasscore.KindCDC, sasscore.KindEOF},
	} {
		got := kinds(t, source)
		if len(got) != len(want) {
			t.Errorf("%s: got %v, want %v", source, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: token %d: got %q, want %q", source, i, got[i], want[i])
			}
		}
	}
}

func TestTokenizeNumbers(t *testing.T) {
	t.Parallel()
	for source, want := range map[string]string{
		"42":          sasscore.KindNumber,
		"-3.5":        sasscore.KindNumber,
		".5":          sasscore.KindNumber,
		"+2.5e3":      sasscore.KindNumber,
		"1.5E-3":      sasscore.KindNumber,
		"50%":         sasscore.KindPercentage,
		"-50%":        sasscore.KindPercentage,
		"12px":        sasscore.KindDimension,
		"1.5e3ms":     sasscore.KindDimension,
		"2e1em":       sasscore.KindDimension,
		"1.5e":        sasscore.KindDimension, // no exponent digits: e is the unit
		"U+4E00-9FFF": sasscore.KindUnicodeRange,
		"U+3??":       sasscore.KindUnicodeRange,
	} {
		got := kinds(t, "x:"+source+";")
		if got[2] != want {
			t.Errorf("%s: got kind %q, want %q", source, got[2], want)
		}
	}
}

func TestTokenizeStringsAndURLs(t *testing.T) {
	t.Parallel()
	source := "a { content: \"quote \\\" \\\\ \\A esc\\\nnew\"; background: url(img/x.png) url(\"q.png\"); }"
	tokens, err := Tokenize(source)
	if err != nil {
		t.Fatal(err)
	}
	strings := 0
	urls := 0
	for _, token := range tokens {
		switch token.Kind {
		case sasscore.KindString:
			strings++
			if token.Text[0] != '"' || token.Text[len(token.Text)-1] != '"' {
				t.Errorf("string text should include quotes: %q", token.Text)
			}
		case sasscore.KindURL:
			urls++
		}
	}
	if strings != 1 || urls != 2 {
		t.Errorf("got %d strings, %d urls", strings, urls)
	}
}

func TestTokenizeCommentsAreTrivia(t *testing.T) {
	t.Parallel()
	got := kinds(t, "/* top level */ a { color: red /* inline */; }")
	want := []string{
		sasscore.KindIdent, sasscore.LBrace, sasscore.KindIdent, sasscore.Colon,
		sasscore.KindIdent, sasscore.Semicolon, sasscore.RBrace, sasscore.KindEOF,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: got %q, want %q", i, got[i], want[i])
		}
	}
	if _, err := Tokenize(`a { color: /* never closed red }`); err == nil {
		t.Error("unclosed comment: expected error")
	}
	if _, err := Tokenize(`a { content: "open }`); err == nil {
		t.Error("unclosed string: expected error")
	}
}

func TestTokenizeSpans(t *testing.T) {
	t.Parallel()
	tokens, err := Tokenize("a .b { }")
	if err != nil {
		t.Fatal(err)
	}
	// . is a delimiter and b an identifier; the ident spans bytes 3..4.
	if tokens[1].Kind != sasscore.Dot || tokens[2].Text != "b" || tokens[2].Span != (gast.Span{Start: 3, End: 4}) {
		t.Errorf(".b tokens: %v %v", tokens[1], tokens[2])
	}
}
