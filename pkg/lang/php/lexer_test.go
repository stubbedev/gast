package php

import (
	"errors"
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

func TestTokenizeTags(t *testing.T) {
	t.Parallel()
	got, err := kinds("<h1>title</h1><?php echo 1; ?>plain text")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		KindInlineHTML, KindOpenTag, KindEcho, KindInt, Semicolon, KindCloseTag, KindInlineHTML, KindEOF,
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

func TestTokenizeShortEchoTag(t *testing.T) {
	t.Parallel()
	got, err := kinds("<?= $x ?>")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{KindOpenTagEcho, KindVariable, KindCloseTag, KindEOF}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTokenizeOperators(t *testing.T) {
	t.Parallel()
	source := "=== !== **= <<= >>= ??= ?-> <=> << >> == != <> <= >= && || ++ -- -> => ::"
	source += " += -= *= /= %= .= &= |= ^= ** ?? ... < = > ! . + - * / % & | ^ ~ ? : ;"
	want := []string{
		KindOpenTag,
		Identical, NotIdentical, PowAssign, ShiftLeftAssign, ShiftRightAssign,
		CoalesceAssign, NullsafeArrow, Spaceship, ShiftLeft,
		ShiftRight, Equal, NotEqual, AngleNotEqual, LtEqual, GtEqual, LogicalAnd,
		LogicalOr, Incr, Decr, Arrow, DoubleArrow, DoubleColon,
		AddAssign, SubAssign, MulAssign, DivAssign, ModAssign, ConcatAssign,
		BitAndAssign, BitOrAssign, BitXorAssign, Pow, Coalesce, Ellipsis,
		Lt, Assign, Gt, Bang, Concat, Plus, Minus, Asterisk,
		Slash, Percent, Ampersand, Pipe, Caret, Tilde, Question, Colon,
		Semicolon, KindEOF,
	}
	got, err := kinds("<?php " + source)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d tokens %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTokenizeKeywordsCaseInsensitive(t *testing.T) {
	t.Parallel()
	got, err := kinds("<?php IF Else eChO WHILE Array LIST Match TRUE FALSE NULL")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		KindOpenTag, KindIf, KindElse, KindEcho, KindWhile, KindArray, KindList,
		KindMatch, KindTrue, KindFalse, KindNull, KindEOF,
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

func TestTokenizeNumbers(t *testing.T) {
	t.Parallel()
	valid := map[string]string{
		"42":          KindInt,
		"0":           KindInt,
		"0x1A_2b":     KindInt,
		"0b1010_0001": KindInt,
		"0o777":       KindInt,
		"017":         KindInt,
		"0_1":         KindInt,
		"1_000_000":   KindInt,
		"08.5":        KindFloat,
		"09e1":        KindFloat,
		".5":          KindFloat,
		"1.":          KindFloat,
		"1.5":         KindFloat,
		"1.5e3":       KindFloat,
		"1.5e-3":      KindFloat,
		"1e+10":       KindFloat,
		"1.5e1_0":     KindFloat,
	}
	for source, wantKind := range valid {
		got, err := kinds("<?php " + source + ";")
		if err != nil {
			t.Errorf("%s: unexpected error %v", source, err)
			continue
		}
		if got[1] != wantKind {
			t.Errorf("%s: got kind %q, want %q", source, got[1], wantKind)
		}
	}

	invalid := []string{"08", "0_9", "1__2", "1_", "0x", "0b", "0o_7", "1e", "1x", "0xFFz"}
	for _, source := range invalid {
		if _, err := kinds("<?php " + source + ";"); err == nil {
			t.Errorf("%s: expected error", source)
		}
	}

	tokens, err := Tokenize("<?php echo 1and 2;")
	if err != nil {
		t.Fatal(err)
	}
	if tokens[2].Kind != KindInt || tokens[2].Text != "1" {
		t.Errorf("1and: got %q %q", tokens[2].Kind, tokens[2].Text)
	}
	if tokens[3].Kind != KindAnd {
		t.Errorf("1and: got %q for and", tokens[3].Kind)
	}
}

func TestTokenizeStrings(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		`'single \' \\ \n literal'`,
		`"double \n \t \x41 \101 \u{1F600} $var {$obj->prop} unknown\escape"`,
		"`shell {$cmd} output`",
	} {
		tokens, err := Tokenize("<?php $a = " + source + ";")
		if err != nil {
			t.Errorf("%s: %v", source, err)
		} else if tokens[3].Kind != KindString {
			t.Errorf("%s: got kind %q", source, tokens[3].Kind)
		}
	}

	for _, source := range []string{`"unterminated`, `'also unterminated`, `"bad \u{zz}"`} {
		if _, err := Tokenize("<?php $a = " + source + ";"); err == nil {
			t.Errorf("%s: expected error", source)
		}
	}
}

func TestTokenizeHeredoc(t *testing.T) {
	t.Parallel()
	tokens, err := Tokenize("<?php $a = <<<EOT\nline one\nline two\nEOT;\n$b = <<<'NOW'\nraw $x\nNOW;\n")
	if err != nil {
		t.Fatal(err)
	}
	if tokens[3].Kind != KindString || tokens[3].Text == "" {
		t.Fatalf("heredoc token: %q %q", tokens[3].Kind, tokens[3].Text)
	}
	if tokens[3].Text[0] != '<' || tokens[3].Text[len(tokens[3].Text)-1] != 'T' {
		t.Errorf("heredoc text should span the whole literal: %q", tokens[3].Text)
	}

	if _, err := Tokenize("<?php $a = <<<EOT\nnever closed\n"); err == nil {
		t.Error("unterminated heredoc: expected error")
	}
}

func TestTokenizeCasts(t *testing.T) {
	t.Parallel()
	for source, cast := range map[string]string{
		"(int)":     "int",
		"(INTEGER)": "int",
		"(bool)":    "bool",
		"(float)":   "float",
		"(double)":  "float",
		"(real)":    "float",
		"(string)":  "string",
		"(binary)":  "string",
		"(array)":   "array",
		"(object)":  "object",
		"(unset)":   "unset",
	} {
		file, err := Parse("<?php " + source + " $b;")
		if err != nil {
			t.Errorf("%s: %v", source, err)
			continue
		}
		stmt := file.Stmts[0].(*ExpressionStmt)
		castExpr, ok := stmt.Expr.(*CastExpr)
		if !ok {
			t.Errorf("%s: expected cast, got %T", source, stmt.Expr)
			continue
		}
		if castExpr.Cast != cast {
			t.Errorf("%s: got canonical %q, want %q", source, castExpr.Cast, cast)
		}
	}
}

func TestTokenizeAttributesAndComments(t *testing.T) {
	t.Parallel()
	got, err := kinds("<?php // comment ?> html <?php /* block */ #[Attr] class C {}")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		KindOpenTag, KindCloseTag, KindInlineHTML, KindOpenTag, KindAttributeOpen,
		KindIdent, RBracket, KindClass, KindIdent, LBrace, RBrace, KindEOF,
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

func TestLexErrorSpan(t *testing.T) {
	t.Parallel()
	_, err := Tokenize("<?php $a = '\\unterminated;")
	if err == nil {
		t.Fatal("expected error")
	}
	var lexErr *gast.LexError
	if !errors.As(err, &lexErr) {
		t.Fatalf("expected *gast.LexError, got %T", err)
	}
	if lexErr.Pos.Start != 11 || lexErr.Pos.End != len("<?php $a = '\\unterminated;") {
		t.Errorf("span: got %v", lexErr.Pos)
	}
}
