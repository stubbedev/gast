package js

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

func TestTokenizeKeywords(t *testing.T) {
	t.Parallel()
	requireKinds(t, "var let x = typeof new", []string{
		KindVar, KindIdent, KindIdent, Assign, KindTypeof, KindNew, KindEOF,
	})
	requireKinds(t, "true false null this super", []string{
		KindTrue, KindFalse, KindNull, KindThis, KindSuper, KindEOF,
	})
}

func TestTokenizeOperators(t *testing.T) {
	t.Parallel()
	source := `>>>= >>> <<= >>= **= === !== &&= ||= ??= ... => ?. ++ --`
	source += ` << >> <= >= && || ?? == != += -= *= %= &= |= ^= ** < > = ! ~ + - * %`
	source += `( ) { } [ ] ; , . ? :`
	want := []string{
		UnsignedShiftAssign, UnsignedShift, ShiftLeftAssign, ShiftRightAssign,
		PowAssign, StrictEqual, StrictNotEqual, LogicalAndAssign, LogicalOrAssign,
		NullishAssign, Ellipsis, Arrow, OptChain, Incr, Decr,
		ShiftLeft, ShiftRight, LtEqual, GtEqual, LogicalAnd, LogicalOr, Nullish,
		Equal, NotEqual, AddAssign, SubAssign, MulAssign, ModAssign,
		BitAndAssign, BitOrAssign, BitXorAssign, Pow, Lt, Gt, Assign, Bang,
		Tilde, Plus, Minus, Asterisk, Percent,
		LParen, RParen, LBrace, RBrace, LBracket, RBracket, Semicolon, Comma,
		Dot, Question, Colon, KindEOF,
	}
	requireKinds(t, source, want)
}

// TestTokenizeRegexVsDivision covers the previous-significant-token
// heuristic: after operand-ending tokens a "/" is division, anywhere
// else it starts a regex literal.
func TestTokenizeRegexVsDivision(t *testing.T) {
	t.Parallel()
	gotKind := ""
	regex := map[string]string{
		"/re/":          KindRegex,
		"x /= 2":        DivAssign,
		"x / 2":         Slash,
		"x = /re/":      KindRegex,
		"return /re/":   KindRegex,
		"a + /re/":      KindRegex,
		"f(/re/)":       KindRegex,
		"a[1] / 2":      Slash,
		"x / y":         Slash,
		"'s' / 2":       Slash,
		"this / 2":      Slash,
		"true / false":  Slash,
		"(a) / 2":       Slash,
		"a++ / 2":       Slash,
		"x = y / z / w": Slash,
	}
	for source, want := range regex {
		got, err := kinds(source)
		if err != nil {
			t.Fatalf("%s: %v", source, err)
		}
		found := false
		for _, kind := range got {
			if kind == KindRegex || kind == Slash || kind == DivAssign {
				if found && kind != gotKind {
					t.Fatalf("%s: mixed slash tokens %v", source, got)
				}
				if kind != want {
					t.Fatalf("%s: got %q, want %q (%v)", source, kind, want, got)
				}
				found = true
				gotKind = kind
			}
		}
		if !found {
			t.Fatalf("%s: no regex or slash token (%v)", source, got)
		}
	}
}

func TestTokenizeRegexLiterals(t *testing.T) {
	t.Parallel()
	valid := []string{"/[\\/]/", "/a\\/b/g", "/x[/]y/u", "/=/", "/\\d+/gimsuy"}
	for _, source := range valid {
		if _, err := kinds(source); err != nil {
			t.Errorf("%s: %v", source, err)
		}
	}
	for _, source := range []string{"var re = /unclosed", "/line\nbreak/"} {
		if _, err := kinds(source); err == nil {
			t.Errorf("%s: expected error", source)
		}
	}
}

func TestTokenizeTemplateNesting(t *testing.T) {
	t.Parallel()
	requireKinds(t, "`plain`", []string{KindNoSubTemplate, KindEOF})
	requireKinds(t, "``", []string{KindNoSubTemplate, KindEOF})
	requireKinds(t, "`a${x}b`", []string{KindTemplateHead, KindIdent, KindTemplateTail, KindEOF})
	requireKinds(t, "`a${ `i${x}` }b${y}c`", []string{
		KindTemplateHead,
		KindTemplateHead, KindIdent, KindTemplateTail,
		KindTemplateMiddle, KindIdent, KindTemplateTail, KindEOF,
	})
	// brace tracking: an object literal inside a substitution
	requireKinds(t, "`x${ {a: 1} }y`", []string{
		KindTemplateHead, LBrace, KindIdent, Colon, KindNum, RBrace,
		KindTemplateTail, KindEOF,
	})
	if _, err := kinds("`unterminated ${x"); err == nil {
		t.Error("unterminated template: expected error")
	}
}

func TestTokenizeNumbers(t *testing.T) {
	t.Parallel()
	valid := map[string]string{
		"42":           KindNum,
		"0":            KindNum,
		"1_000_000":    KindNum,
		"1.5":          KindNum,
		".5":           KindNum,
		"5.":           KindNum,
		"1e3":          KindNum,
		"1_0.5_0e-1_0": KindNum,
		"0x1F_FF":      KindNum,
		"0Xff":         KindNum,
		"0o17":         KindNum,
		"0b1010_1":     KindNum,
		"0755":         KindNum,
		"089":          KindNum,
		"09.5e1":       KindNum,
		"10n":          KindBigInt,
		"0xFn":         KindBigInt,
		"1_0n":         KindBigInt,
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
	invalid := []string{"1_000_", "1__0", "0x", "0b2", "0o8", "1.5n", "1e", "1e+", "1abc", "0_1", "0_0n"}
	for _, source := range invalid {
		if _, err := kinds(source); err == nil {
			t.Errorf("%s: expected error", source)
		}
	}
}

func TestTokenizeStrings(t *testing.T) {
	t.Parallel()
	valid := []string{
		"'single'", `"double"`, "'\\''", `"\\"`, `"\n\t\r\b\f\v\0"`,
		`"\x41"`, `"\u0041"`, `"\u{1F600}"`, "'line\\\ncontinue'",
	}
	for _, source := range valid {
		if _, err := kinds(source); err != nil {
			t.Errorf("%s: %v", source, err)
		}
	}
	invalid := []string{"'unterminated", "\"multi\nline\"", `"\x4"`, `"\u12"`, `"\u{}"`, `"\u{110000}"`}
	for _, source := range invalid {
		if _, err := kinds(source); err == nil {
			t.Errorf("%s: expected error", source)
		}
	}
}

func TestTokenizeMisc(t *testing.T) {
	t.Parallel()
	requireKinds(t, "#!/usr/bin/env node\nx", []string{KindHashbang, KindIdent, KindEOF})
	requireKinds(t, "#priv", []string{KindPrivateName, KindEOF})
	requireKinds(t, "a.#priv", []string{KindIdent, Dot, KindPrivateName, KindEOF})
	requireKinds(t, "café_$1", []string{KindIdent, KindEOF})
	requireKinds(t, "5 .toFixed()", []string{KindNum, Dot, KindIdent, LParen, RParen, KindEOF})
	requireKinds(t, "5..toFixed()", []string{KindNum, Dot, KindIdent, LParen, RParen, KindEOF})
	requireKinds(t, "a?.5:1", []string{KindIdent, Question, KindNum, Colon, KindNum, KindEOF})
	requireKinds(t, "// c\n/* b */ x", []string{KindIdent, KindEOF})
	if _, err := kinds("/* unterminated"); err == nil {
		t.Error("unterminated comment: expected error")
	}
	if _, err := kinds("#"); err == nil {
		t.Error("bare '#': expected error")
	}
}

func TestTokenizeSpans(t *testing.T) {
	t.Parallel()
	tokens, err := Tokenize("  let x = 42;")
	if err != nil {
		t.Fatal(err)
	}
	want := []gast.Span{
		{Start: 2, End: 5},
		{Start: 6, End: 7},
		{Start: 8, End: 9},
		{Start: 10, End: 12},
		{Start: 12, End: 13},
	}
	for i, span := range want {
		if tokens[i].Span != span {
			t.Errorf("token %d span: got %v, want %v", i, tokens[i].Span, span)
		}
	}
}
