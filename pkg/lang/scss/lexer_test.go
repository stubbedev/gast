package scss

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

func TestTokenizeSassTokens(t *testing.T) {
	t.Parallel()
	for source, want := range map[string][]string{
		"$size: 1px;": {
			sasscore.KindVariable, sasscore.Colon, sasscore.KindDimension,
			sasscore.Semicolon, sasscore.KindEOF,
		},
		"a.#{&$x} { }": {
			sasscore.KindIdent, sasscore.Dot, sasscore.KindInterpOpen,
			sasscore.Ampersand, sasscore.KindVariable, sasscore.RBrace,
			sasscore.LBrace, sasscore.RBrace, sasscore.KindEOF,
		},
		"// silent\na { }": {
			sasscore.KindIdent, sasscore.LBrace, sasscore.RBrace, sasscore.KindEOF,
		},
		"m.max(1px, 2px);": {
			sasscore.KindIdent, sasscore.Dot, sasscore.KindFunction, sasscore.KindDimension,
			sasscore.Comma, sasscore.KindDimension, sasscore.RParen, sasscore.Semicolon,
			sasscore.KindEOF,
		},
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

func TestTokenizeInterpolationSpan(t *testing.T) {
	t.Parallel()
	tokens, err := Tokenize("#{1 + 2}")
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Kind != sasscore.KindInterpOpen || tokens[0].Text != "#{" ||
		tokens[0].Span != (gast.Span{Start: 0, End: 2}) {
		t.Errorf("interp open token: %#v", tokens[0])
	}
}

func TestTokenizeErrors(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		`a { content: "open; }`,
		`a { color: /* open }`,
	} {
		if _, err := Tokenize(source); err == nil {
			t.Errorf("%s: expected error", source)
		}
	}
}
