package python

import (
	"errors"
	"strings"
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

func assertKinds(t *testing.T, source string, want []string) {
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

func TestTokenizeOperators(t *testing.T) {
	t.Parallel()
	assertKinds(t, "+ - * ** / // % @ << >> & | ^ ~ := < > <= >= == !=", []string{
		Plus, Minus, Star, DoubleStar, Slash, DoubleSlash, Percent, At,
		LeftShift, RightShift, Ampersand, Pipe, Caret, Tilde, Walrus,
		Less, Greater, LessEqual, GreaterEqual, EqualEqual, NotEqual,
		KindNewline, KindEOF,
	})
	assertKinds(t, "( ) [ ] { } , : . ; = -> ...", []string{
		LParen, RParen, LBracket, RBracket, LBrace, RBrace,
		Comma, Colon, Dot, Semicolon, Assign, Arrow, Ellipsis,
		KindNewline, KindEOF,
	})
	assertKinds(t, "+= -= *= /= //= %= @= &= |= ^= <<= >>= **=", []string{
		AddAssign, SubAssign, MulAssign, DivAssign, FloorDivAssign,
		ModAssign, MatMulAssign, BitAndAssign, BitOrAssign, BitXorAssign,
		LeftShiftAssign, RightShiftAssign, PowAssign, KindNewline, KindEOF,
	})
}

func TestTokenizeKeywords(t *testing.T) {
	t.Parallel()
	source := "False None True and as assert async await break class continue def del " +
		"elif else except finally for from global if import in is lambda nonlocal not " +
		"or pass raise return try while with yield"
	want := []string{
		KindFalse, KindNone, KindTrue, KindAnd, KindAs, KindAssert, KindAsync,
		KindAwait, KindBreak, KindClass, KindContinue, KindDef, KindDel, KindElif,
		KindElse, KindExcept, KindFinally, KindFor, KindFrom, KindGlobal, KindIf,
		KindImport, KindIn, KindIs, KindLambda, KindNonlocal, KindNot, KindOr,
		KindPass, KindRaise, KindReturn, KindTry, KindWhile, KindWith, KindYield,
		KindNewline, KindEOF,
	}
	assertKinds(t, source, want)
}

func TestTokenizeSoftKeywordsStayNames(t *testing.T) {
	t.Parallel()
	for _, word := range []string{"match", "case", "type", "_"} {
		assertKinds(t, word+" = 1", []string{KindName, Assign, KindInt, KindNewline, KindEOF})
	}
}

func TestTokenizeIndentDedent(t *testing.T) {
	t.Parallel()
	assertKinds(t, "if a:\n    pass\nx = 1\n", []string{
		KindIf, KindName, Colon, KindNewline,
		KindIndent, KindPass, KindNewline, KindDedent,
		KindName, Assign, KindInt, KindNewline, KindEOF,
	})
	assertKinds(t, "if a:\n    if b:\n        pass\nx = 1\n", []string{
		KindIf, KindName, Colon, KindNewline,
		KindIndent, KindIf, KindName, Colon, KindNewline,
		KindIndent, KindPass, KindNewline, KindDedent, KindDedent,
		KindName, Assign, KindInt, KindNewline, KindEOF,
	})
}

func TestTokenizeBlankAndCommentLinesProduceNothing(t *testing.T) {
	t.Parallel()
	assertKinds(t, "\n# comment\n\n   \nx = 1\n", []string{
		KindName, Assign, KindInt, KindNewline, KindEOF,
	})
}

func TestTokenizeNewlineWithoutTrailingLineBreak(t *testing.T) {
	t.Parallel()
	assertKinds(t, "x = 1", []string{KindName, Assign, KindInt, KindNewline, KindEOF})
}

func TestTokenizeLineJoins(t *testing.T) {
	t.Parallel()
	assertKinds(t, "x = (1 +\n    2)\n", []string{
		KindName, Assign, LParen, KindInt, Plus, KindInt, RParen, KindNewline, KindEOF,
	})
	assertKinds(t, "x = 1 + \\\n    2\n", []string{
		KindName, Assign, KindInt, Plus, KindInt, KindNewline, KindEOF,
	})
	assertKinds(t, "if a:\n    pass\n    \\\n    pass2 = 1\n", []string{
		KindIf, KindName, Colon, KindNewline,
		KindIndent, KindPass, KindNewline, KindName, Assign, KindInt, KindNewline,
		KindDedent, KindEOF,
	})
}

func TestTokenizeNumbers(t *testing.T) {
	t.Parallel()
	valid := map[string]string{
		"42":          KindInt,
		"0":           KindInt,
		"0_0":         KindInt,
		"1_000_000":   KindInt,
		"0x1A_2b":     KindInt,
		"0Xff":        KindInt,
		"0x_1":        KindInt,
		"0o777":       KindInt,
		"0b1010_1":    KindInt,
		"1.":          KindFloat,
		".5":          KindFloat,
		"1.5":         KindFloat,
		"1e10":        KindFloat,
		"1.5E-10":     KindFloat,
		"2j":          KindComplex,
		"3.5J":        KindComplex,
		"1_0.0_1e1_0": KindFloat,
	}
	for source, want := range valid {
		got, err := kinds(source)
		if err != nil {
			t.Fatalf("%s: %v", source, err)
		}
		if got[0] != want {
			t.Fatalf("%s: got %q, want %q", source, got[0], want)
		}
	}
	for _, source := range []string{"08", "018", "0x", "0b", "1e", "1_", "1abc", "0x1g", "1.2.3", "1jx", "1.real"} {
		if _, err := kinds(source); err == nil {
			t.Fatalf("%s: expected an error", source)
		}
	}
	if _, err := kinds("1if x else 2"); err != nil {
		t.Fatalf("1if x else 2: %v", err)
	}
}

func TestTokenizeStrings(t *testing.T) {
	t.Parallel()
	assertKinds(t, "'a' \"b\" '''c''' \"\"\"d\"\"\"", []string{
		KindString, KindString, KindString, KindString, KindNewline, KindEOF,
	})
	assertKinds(t, "b'x' B\"y\" rb'z' u'w'", []string{
		KindBytes, KindBytes, KindBytes, KindString, KindNewline, KindEOF,
	})
	if _, err := kinds("ur'x'"); err == nil {
		t.Fatal("expected invalid string prefix error")
	}
	if _, err := kinds("'unterminated"); err == nil {
		t.Fatal("expected unterminated string error")
	}
	if _, err := kinds("\"a\nb\""); err == nil {
		t.Fatal("expected newline in single-quoted string error")
	}
}

func TestTokenizeFStrings(t *testing.T) {
	t.Parallel()
	assertKinds(t, `f"a{x}b"`, []string{
		KindFStringStart, KindFStringMiddle, LBrace, KindName, RBrace,
		KindFStringMiddle, KindFStringEnd, KindNewline, KindEOF,
	})
	assertKinds(t, `f"{x!r}"`, []string{
		KindFStringStart, LBrace, KindName, KindConversion, RBrace,
		KindFStringEnd, KindNewline, KindEOF,
	})
	assertKinds(t, `f"{x:.2f}"`, []string{
		KindFStringStart, LBrace, KindName, Colon, KindFStringMiddle, RBrace,
		KindFStringEnd, KindNewline, KindEOF,
	})
	// PEP 701: nested same quotes inside the field.
	assertKinds(t, `f"{d["k"]}"`, []string{
		KindFStringStart, LBrace, KindName, LBracket, KindString, RBracket, RBrace,
		KindFStringEnd, KindNewline, KindEOF,
	})
	// A colon at field depth starts the spec, even before '='.
	assertKinds(t, `f"{x:=1}"`, []string{
		KindFStringStart, LBrace, KindName, Colon, KindFStringMiddle, RBrace,
		KindFStringEnd, KindNewline, KindEOF,
	})
	// Nested spec fields.
	assertKinds(t, `f"{x:{w}}"`, []string{
		KindFStringStart, LBrace, KindName, Colon, LBrace, KindName, RBrace, RBrace,
		KindFStringEnd, KindNewline, KindEOF,
	})
	// Walrus needs parentheses inside a field.
	assertKinds(t, `f"{(x := 1)}"`, []string{
		KindFStringStart, LBrace, LParen, KindName, Walrus, KindInt, RParen, RBrace,
		KindFStringEnd, KindNewline, KindEOF,
	})
	// Doubled braces are literal text.
	assertKinds(t, `f"{{x}}"`, []string{
		KindFStringStart, KindFStringMiddle, KindFStringEnd, KindNewline, KindEOF,
	})
	assertKinds(t, "f\"\"\"a\nb{x}\"\"\"", []string{
		KindFStringStart, KindFStringMiddle, LBrace, KindName, RBrace,
		KindFStringEnd, KindNewline, KindEOF,
	})
}

func TestTokenizeFStringErrors(t *testing.T) {
	t.Parallel()
	for _, source := range []string{`f"{a!}"`, `f"}"`, "f\"{a", `f"{a` + "\n" + `"`} {
		if _, err := kinds(source); err == nil {
			t.Fatalf("%s: expected an error", source)
		}
	}
}

func TestTokenizeLexErrors(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"if a:\n    x = 1\n\ty = 2\n": "tabs and spaces",
		"if a:\n        pass\n  x\n":  "unindent",
		"x = (1\n":                    "never closed",
		"x = 1 $\n":                   "unexpected character",
	}
	for source, want := range cases {
		_, err := Tokenize(source)
		if err == nil {
			t.Fatalf("%q: expected an error", source)
		}
		var lexErr *gast.LexError
		if !errors.As(err, &lexErr) {
			t.Fatalf("%q: expected *gast.LexError, got %T", source, err)
		}
		if !strings.Contains(lexErr.Message, want) {
			t.Fatalf("%q: message %q lacks %q", source, lexErr.Message, want)
		}
	}
}

func TestTokenizeSpans(t *testing.T) {
	t.Parallel()
	tokens, err := Tokenize("x = 1\n")
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Span != (gast.Span{Start: 0, End: 1}) {
		t.Fatalf("name span: %v", tokens[0].Span)
	}
	if tokens[3].Kind != KindNewline || tokens[3].Span != (gast.Span{Start: 5, End: 6}) {
		t.Fatalf("newline span: %v", tokens[3].Span)
	}
}
