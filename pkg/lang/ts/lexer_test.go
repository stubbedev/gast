package ts

import (
	"testing"

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

func requireKinds(t *testing.T, source string, want []string) {
	t.Helper()
	got, err := kinds(source)
	if err != nil {
		t.Fatalf("%s: %v", source, err)
	}
	if len(got) != len(want) {
		t.Fatalf("%s: got %d tokens %v, want %d", source, len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: token %d: got %q, want %q (%v)", source, i, got[i], want[i], got)
		}
	}
}

// TestTokenizeKeywordKinds covers reserved words, TypeScript's
// contextual keywords (which lex as identifiers), and the decorator
// token.
func TestTokenizeKeywordKinds(t *testing.T) {
	t.Parallel()
	requireKinds(t, "var let x = typeof new", []string{
		KindVar, KindIdent, KindIdent, Assign, KindTypeof, KindNew, KindEOF,
	})
	requireKinds(t, "type interface enum namespace declare readonly satisfies", []string{
		KindIdent, KindIdent, KindIdent, KindIdent, KindIdent, KindIdent, KindIdent, KindEOF,
	})
	requireKinds(t, "@sealed class Point {}", []string{
		At, KindIdent, KindClass, KindIdent, LBrace, RBrace, KindEOF,
	})
}

func TestTokenizeOperators(t *testing.T) {
	t.Parallel()
	requireKinds(t, `>>>= >>> <<= >>= **= === !== &&= ||= ??= ... => ?. ++ --`, []string{
		UnsignedShiftAssign, UnsignedShift, ShiftLeftAssign, ShiftRightAssign,
		PowAssign, StrictEqual, StrictNotEqual, LogicalAndAssign, LogicalOrAssign,
		NullishAssign, Ellipsis, Arrow, OptChain, Incr, Decr, KindEOF,
	})
}

// TestTokenizeRegexVsDivision covers the previous-significant-token
// heuristic shared with the JavaScript lexer: after operand-ending
// tokens a "/" is division, anywhere else a regex literal.
func TestTokenizeRegexVsDivision(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"/re/":          KindRegex,
		"x /= 2":        DivAssign,
		"x / 2":         Slash,
		"x = /re/":      KindRegex,
		"return /re/":   KindRegex,
		"a + /re/":      KindRegex,
		"f(/re/)":       KindRegex,
		"a[1] / 2":      Slash,
		"(a) / 2":       Slash,
		"a++ / 2":       Slash,
		"x = y / z / w": Slash,
	}
	for source, want := range cases {
		got, err := kinds(source)
		if err != nil {
			t.Fatalf("%s: %v", source, err)
		}
		found := false
		for _, kind := range got {
			if kind == KindRegex || kind == Slash || kind == DivAssign {
				if kind != want {
					t.Fatalf("%s: got %q, want %q (%v)", source, kind, want, got)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: no regex or slash token (%v)", source, got)
		}
	}
}

// TestTokenizeTemplates covers nested template substitutions with
// type-annotated expressions inside.
func TestTokenizeTemplates(t *testing.T) {
	t.Parallel()
	requireKinds(t, "`a${x: T}b`", []string{
		KindTemplateHead, KindIdent, Colon, KindIdent, KindTemplateTail, KindEOF,
	})
	requireKinds(t, "`x${ {a: 1} }y`", []string{
		KindTemplateHead, LBrace, KindIdent, Colon, KindNum, RBrace,
		KindTemplateTail, KindEOF,
	})
	if _, err := kinds("`unterminated ${x"); err == nil {
		t.Error("unterminated template: expected error")
	}
}

func TestTokenizeNumbersAndStrings(t *testing.T) {
	t.Parallel()
	valid := map[string]string{
		"42":         KindNum,
		"1_000.5e-3": KindNum,
		"0x1F_ff":    KindNum,
		"0b1010n":    KindBigInt,
		"10n":        KindBigInt,
	}
	for source, want := range valid {
		got, err := kinds(source)
		if err != nil {
			t.Errorf("%s: %v", source, err)
			continue
		}
		if got[0] != want {
			t.Errorf("%s: got %q, want %q", source, got[0], want)
		}
	}
	for _, source := range []string{"1e", "0x", "1_000_", "1abc", "'unterminated"} {
		if _, err := kinds(source); err == nil {
			t.Errorf("%s: expected error", source)
		}
	}
}

// TestGtRunHelpers covers the angle-bracket bookkeeping that peels
// merged '>' runs apart while closing nested type arguments.
func TestGtRunHelpers(t *testing.T) {
	t.Parallel()
	if got := gtOpens(Lt); got != 1 {
		t.Errorf("gtOpens(<) = %d", got)
	}
	if got := gtOpens(ShiftLeft); got != 2 {
		t.Errorf("gtOpens(<<) = %d", got)
	}
	for kind, want := range map[string]int{
		Gt: 1, GtEqual: 1, ShiftRight: 2, ShiftRightAssign: 2,
		UnsignedShift: 3, UnsignedShiftAssign: 3,
	} {
		if got := gtCloses(kind); got != want {
			t.Errorf("gtCloses(%q) = %d, want %d", kind, got, want)
		}
	}
	run := []gast.Token{{
		Kind: ShiftRight, Text: ">>",
		Span: gast.Span{Start: 10, End: 12},
	}}
	split, ok := splitGreater(run, 0)
	if !ok {
		t.Fatal("expected split")
	}
	if len(split) != 2 || split[0].Kind != Gt || split[1].Kind != Gt {
		t.Fatalf("split >>: %v", split)
	}
	if split[0].Span != (gast.Span{Start: 10, End: 11}) || split[1].Span != (gast.Span{Start: 11, End: 12}) {
		t.Fatalf("split >> spans: %v", split)
	}
	run = []gast.Token{{
		Kind: GtEqual, Text: ">=",
		Span: gast.Span{Start: 0, End: 2},
	}}
	split, ok = splitGreater(run, 0)
	if !ok || len(split) != 2 || split[0].Kind != Gt || split[1].Kind != Assign {
		t.Fatalf("split >=: %v %v", split, ok)
	}
	if _, ok := splitGreater([]gast.Token{{Kind: Lt, Text: "<"}}, 0); ok {
		t.Error("expected no split for '<'")
	}
}
