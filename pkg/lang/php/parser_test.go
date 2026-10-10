package php

import (
	"strings"
	"testing"

	"github.com/stubbedev/gast/pkg/gast"
)

// exprOf parses `<?php <source>;` and returns the single expression.
func exprOf(t *testing.T, source string) Expr {
	t.Helper()
	file, err := Parse("<?php " + source + ";")
	if err != nil {
		t.Fatalf("%s: %v", source, err)
	}
	if len(file.Stmts) != 1 {
		t.Fatalf("%s: got %d statements", source, len(file.Stmts))
	}
	stmt, ok := file.Stmts[0].(*ExpressionStmt)
	if !ok {
		t.Fatalf("%s: got %T", source, file.Stmts[0])
	}
	return stmt.Expr
}

func assertBinary(t *testing.T, expr Expr, op string) (Expr, Expr) {
	t.Helper()
	binary, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if binary.Op != op {
		t.Fatalf("binary op: got %q, want %q", binary.Op, op)
	}
	return binary.Left, binary.Right
}

func TestParsePrecedence(t *testing.T) {
	t.Parallel()

	// 1 + 2 * 3 groups as 1 + (2 * 3)
	left, right := assertBinary(t, exprOf(t, "1 + 2 * 3"), Plus)
	if lit, ok := left.(*IntegerLit); !ok || lit.Text != "1" {
		t.Fatalf("left: %#v", left)
	}
	_, _ = assertBinary(t, right, Asterisk)

	// 1 . 2 + 3: additive binds tighter than concatenation
	_, right = assertBinary(t, exprOf(t, "1 . 2 + 3"), Concat)
	_, _ = assertBinary(t, right, Plus)

	// "a" . "b" . "c" groups left: ("a" . "b") . "c"
	left, _ = assertBinary(t, exprOf(t, `"a" . "b" . "c"`), Concat)
	_, _ = assertBinary(t, left, Concat)

	// 2 ** 3 ** 4 groups right: 2 ** (3 ** 4)
	_, right = assertBinary(t, exprOf(t, "2 ** 3 ** 4"), Pow)
	_, _ = assertBinary(t, right, Pow)

	// -2 ** 2 binds as -(2 ** 2)
	unary, ok := exprOf(t, "-2 ** 2").(*UnaryExpr)
	if !ok || unary.Op != Minus {
		t.Fatalf("expected unary minus, got %#v", unary)
	}
	_, _ = assertBinary(t, unary.Expr, Pow)

	// $a ?? $b ?? $c groups right
	_, right = assertBinary(t, exprOf(t, "$a ?? $b ?? $c"), Coalesce)
	_, _ = assertBinary(t, right, Coalesce)

	// 1 && 2 || 3 groups as (1 && 2) || 3
	left, _ = assertBinary(t, exprOf(t, "1 && 2 || 3"), LogicalOr)
	_, _ = assertBinary(t, left, LogicalAnd)

	// keyword operators bind looser than assignment
	left, _ = assertBinary(t, exprOf(t, "$x = 1 and 2"), KindAnd)
	assign, ok := left.(*AssignExpr)
	if !ok {
		t.Fatalf("expected assign under 'and', got %T", left)
	}
	if _, ok := assign.Value.(*IntegerLit); !ok {
		t.Fatalf("assign value: %T", assign.Value)
	}

	// shift binds tighter than concat and looser than additive
	_, right = assertBinary(t, exprOf(t, "1 . 2 << 3"), Concat)
	_, _ = assertBinary(t, right, ShiftLeft)
}

func TestParseTernary(t *testing.T) {
	t.Parallel()
	ternary, ok := exprOf(t, "$a ? 1 : 2").(*TernaryExpr)
	if !ok {
		t.Fatalf("got %T", ternary)
	}
	if ternary.Then == nil || ternary.Else == nil {
		t.Fatalf("both branches required: %#v", ternary)
	}

	short, ok := exprOf(t, "$a ?: 2").(*TernaryExpr)
	if !ok {
		t.Fatalf("got %T", short)
	}
	if short.Then != nil {
		t.Fatalf("elvis Then must be nil: %#v", short.Then)
	}

	if err := Validate("<?php $a = $b ? 1;"); err == nil {
		t.Error("ternary missing colon: expected error")
	}
}

func TestParseAssignment(t *testing.T) {
	t.Parallel()
	for source, op := range map[string]string{
		"$a = 1":   Assign,
		"$a += 1":  AddAssign,
		"$a ??= 1": CoalesceAssign,
		"$a <<= 1": ShiftLeftAssign,
	} {
		assign, ok := exprOf(t, source).(*AssignExpr)
		if !ok {
			t.Fatalf("%s: got %T", source, exprOf(t, source))
		}
		if assign.Op != op {
			t.Fatalf("%s: op %q, want %q", source, assign.Op, op)
		}
	}

	ref, ok := exprOf(t, "$a = &$b").(*AssignExpr)
	if !ok || !ref.Ref {
		t.Fatalf("reference assignment: %#v", ref)
	}

	chained, ok := exprOf(t, "$a = $b = 1").(*AssignExpr)
	if !ok {
		t.Fatalf("chained assignment: %T", chained)
	}
	if _, ok := chained.Value.(*AssignExpr); !ok {
		t.Fatalf("right side of chained assignment: %T", chained.Value)
	}
}

func TestParsePostfix(t *testing.T) {
	t.Parallel()
	call, ok := exprOf(t, "foo(1, name: 2, ...$rest)").(*CallExpr)
	if !ok {
		t.Fatalf("got %T", call)
	}
	if len(call.Args) != 3 {
		t.Fatalf("args: %d", len(call.Args))
	}
	if call.Args[1].Name != "name" {
		t.Fatalf("named arg: %q", call.Args[1].Name)
	}
	if !call.Args[2].Spread {
		t.Fatal("spread arg")
	}

	method, ok := exprOf(t, "$obj->run(1)").(*MethodCall)
	if !ok {
		t.Fatalf("got %T", method)
	}
	if ident, ok := method.Method.(*Identifier); !ok || ident.Name != "run" {
		t.Fatalf("method name: %#v", method.Method)
	}

	nullsafe, ok := exprOf(t, "$obj?->prop?->method()").(*NullsafeMethodCall)
	if !ok {
		t.Fatalf("got %T", nullsafe)
	}
	if prop, ok := nullsafe.Target.(*NullsafePropertyFetch); !ok {
		t.Fatalf("chained nullsafe: %T", nullsafe.Target)
	} else if _, ok := prop.Name.(*Identifier); !ok {
		t.Fatalf("nullsafe property name: %T", prop.Name)
	}

	staticCall, ok := exprOf(t, "Cls::$prop::CONST[0]").(*IndexExpr)
	if !ok {
		t.Fatalf("got %T", staticCall)
	}
	if _, ok := staticCall.Target.(*ClassConstFetch); !ok {
		t.Fatalf("class const: %T", staticCall.Target)
	}

	if err := Validate("<?php $obj->{$name} = 1;"); err != nil {
		t.Fatal(err)
	}
}

func TestParseVariables(t *testing.T) {
	t.Parallel()
	if _, ok := exprOf(t, "$name").(*Variable); !ok {
		t.Fatal("simple variable")
	}
	vv, ok := exprOf(t, "$$name").(*VariableVariable)
	if !ok {
		t.Fatalf("variable variable: %T", vv)
	}
	braced, ok := exprOf(t, "${'name'}").(*VariableVariable)
	if !ok {
		t.Fatalf("braced variable variable: %T", braced)
	}
}

func TestParseArraysAndList(t *testing.T) {
	t.Parallel()
	arr, ok := exprOf(t, "[1, 'k' => 2, ...$rest, &$byRef]").(*ArrayExpr)
	if !ok {
		t.Fatalf("got %T", arr)
	}
	if len(arr.Items) != 4 || arr.Legacy {
		t.Fatalf("items: %#v", arr.Items)
	}
	if !arr.Items[2].Spread || !arr.Items[3].ByRef {
		t.Fatalf("spread and byref items: %#v", arr.Items[2:])
	}

	legacy, ok := exprOf(t, "array('k' => 1)").(*ArrayExpr)
	if !ok || !legacy.Legacy {
		t.Fatalf("legacy array: %T", legacy)
	}

	list, ok := exprOf(t, "list($a, , $c)").(*ArrayExpr)
	if !ok || !list.Legacy {
		t.Fatalf("list(): %T", list)
	}
	if len(list.Items) != 3 || list.Items[1].Value != nil {
		t.Fatalf("empty list slot: %#v", list.Items)
	}
}

func TestParseStringLiterals(t *testing.T) {
	t.Parallel()
	tests := []struct {
		source string
		style  string
		value  string
	}{
		{`'a\nb'`, "single", `a\nb`},
		{`'it\'s \\ done'`, "single", `it's \ done`},
		{`"a\tb\x41A\u{41}"`, "double", "a\tbAAA"},
		{`"oct\101al"`, "double", "octAal"},
	}
	for _, tc := range tests {
		lit, ok := exprOf(t, tc.source).(*StringLit)
		if !ok {
			t.Fatalf("%s: got %T", tc.source, exprOf(t, tc.source))
		}
		if lit.Kind != tc.style {
			t.Errorf("%s: style %q, want %q", tc.source, lit.Kind, tc.style)
		}
		if lit.Value != tc.value {
			t.Errorf("%s: value %q, want %q", tc.source, lit.Value, tc.value)
		}
	}
}

func TestParseClosuresAndArrows(t *testing.T) {
	t.Parallel()
	if err := Validate("<?php $f = function (int $x) use (&$sum, $base): int { return $x; };"); err != nil {
		t.Fatal(err)
	}

	arrow, ok := exprOf(t, "fn(int ...$n): int => array_sum($n)").(*ArrowFnExpr)
	if !ok {
		t.Fatalf("arrow fn: %T", arrow)
	}
	if arrow.ReturnType == nil || len(arrow.Params) != 1 || !arrow.Params[0].Variadic {
		t.Fatalf("arrow fn shape: %#v", arrow)
	}

	if err := Validate("<?php $f = static function () use ($x) {};"); err != nil {
		t.Fatal(err)
	}
}

func TestParseMatchAndThrow(t *testing.T) {
	t.Parallel()
	match, ok := exprOf(t, "match($x) { 1, 2 => 'a', default => 'b' }").(*MatchExpr)
	if !ok {
		t.Fatalf("match: %T", match)
	}
	if len(match.Arms) != 2 || len(match.Arms[0].Conds) != 2 || match.Arms[1].Conds != nil {
		t.Fatalf("match arms: %#v", match.Arms)
	}

	throw, ok := exprOf(t, "$x ?: throw new E('no')").(*TernaryExpr)
	if !ok {
		t.Fatalf("throw expr: %T", throw)
	}
	if _, ok := throw.Else.(*ThrowExpr); !ok {
		t.Fatalf("else branch: %T", throw.Else)
	}
}

func TestParseControlFlow(t *testing.T) {
	t.Parallel()
	sources := []string{
		"<?php if ($a) { 1; } elseif ($b) { 2; } else { 3; }",
		"<?php if ($a): 1; elseif ($b): 2; else: 3; endif;",
		"<?php while (true) { break 2; }",
		"<?php while (true): continue; endwhile;",
		"<?php do { echo 1; } while ($c);",
		"<?php for ($i = 0, $j = 1; $i < $j; $i++, $j--) { echo $i; }",
		"<?php for (;;);",
		"<?php foreach ($list as $item) {}",
		"<?php foreach ($list as $key => &$value): echo $key; endforeach;",
		"<?php goto end; end: echo 'done';",
		"<?php switch ($a) { case 1: case 2; break; default: return; }",
		"<?php switch ($a): case 1: return 1; default: return 0; endswitch;",
		"<?php declare(strict_types=1);",
		"<?php declare(ticks=1) { echo 1; }",
		"<?php declare(ticks=1): echo 1; enddeclare;",
		"<?php try { risky(); } catch (A | B $e) { handle($e); } catch (C) {} finally { cleanup(); }",
	}
	for _, source := range sources {
		if err := Validate(source); err != nil {
			t.Errorf("%s: %v", source, err)
		}
	}
}

func TestParseDeclarations(t *testing.T) {
	t.Parallel()
	sources := []string{
		"<?php function &f(int $a, ?string $b = null, ...$rest): void {}",
		"<?php function typed(array &$d = [], self $e) {}",
		"<?php abstract class A extends B implements C, D { abstract protected function m(): static; }",
		"<?php final readonly class V { public function __construct(public int $x) {} }",
		"<?php class C { use T1, T2 { T1::m insteadof T2; T2::m as protected aliased; } }",
		"<?php class C { public ?int $nullable = null; private readonly int $id; var $legacy; public int $a, $b = 2; }",
		"<?php class C { public const int TYPED = 1; final public const FLIPPED = 'v'; }",
		"<?php interface I extends A, B { const LIMIT = 10; public function m(): mixed; }",
		"<?php trait T { public function m(): void {} }",
		"<?php enum Suit: string implements H { case Hearts = 'H'; public function color(): string { return 'r'; } }",
		"<?php enum Status { case Draft; }",
		"#[A, B(name: 'x')] class C {}",
		"<?php $anon = new class(1) extends Base implements I { private int $x; };",
		"<?php $anon = new readonly class {};",
		"<?php namespace App\\Sub; use function strlen; use const PHP_EOL;",
		"<?php namespace App { use A\\{B, function c\\d, const E}; class Local {} }",
		"<?php const APP = 'name', OTHER = 2;",
		"<?php global $a, $$b;",
		"<?php static $counter = 0, $other;",
		"<?php unset($a[0], $b);",
		"<?php echo 'a', 'b';",
		"<?php $x = new ($factory->build())();",
		"<?php $y = new $class;",
		"<?php $z = $obj instanceof \\App\\Concrete;",
	}
	for _, source := range sources {
		if err := Validate(source); err != nil {
			t.Errorf("%s: %v", source, err)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	t.Parallel()
	sources := []string{
		"<?php $a = ;",
		"<?php function f( {}",
		"<?php $a = 1 2;",
		"<?php $a = 1.2.3;",
		"<?php $x = $b ? 1;",
		"<?php foreach ($list $item) {}",
		"<?php class { }",
		"<?php $a = new;",
		"<?php $a = $b->;",
		"<?php try {}",
		"<?php }",
		"<?php switch (1) { echo 2; }",
		"<?php #[Attr class C {}",
		"<?php enum E: int { case A; }",
	}
	for _, source := range sources {
		if err := Validate(source); err == nil {
			t.Errorf("%s: expected error", source)
		}
	}
}

func TestParseSpans(t *testing.T) {
	t.Parallel()
	source := "<?php $a = 1 + 2;"
	file, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if file.Pos() != (gast.Span{Start: 0, End: len(source)}) {
		t.Errorf("file span: %v", file.Pos())
	}
	binary := exprOf(t, "1 + 2")
	if binary.Pos().Start != 6 || binary.Pos().End != 11 {
		t.Errorf("binary span: %v", binary.Pos())
	}
}

func TestFormatErrorCaret(t *testing.T) {
	t.Parallel()
	source := "<?php\n$a = ;"
	err := Validate(source)
	if err == nil {
		t.Fatal("expected error")
	}
	formatted := gast.FormatError(err, source)
	if !strings.Contains(formatted, "ParseError at 2:6:") {
		t.Errorf("missing position: %q", formatted)
	}
	if !strings.Contains(formatted, "2 | $a = ;") {
		t.Errorf("missing source line: %q", formatted)
	}
	if !strings.Contains(formatted, "  |      ^") {
		t.Errorf("missing caret: %q", formatted)
	}
}

func TestHeredocValue(t *testing.T) {
	t.Parallel()
	lit, ok := exprOf(t, "<<<EOT\nline\nEOT").(*StringLit)
	if !ok {
		t.Fatalf("heredoc: %T", lit)
	}
	if lit.Kind != "heredoc" || lit.Value != "line" {
		t.Errorf("heredoc value: %q", lit.Value)
	}
}
