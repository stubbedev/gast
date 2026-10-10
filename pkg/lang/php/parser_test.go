package php

import (
	"errors"
	"fmt"
	"slices"
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

func TestParseHaltCompiler(t *testing.T) {
	t.Parallel()
	t.Run("valid placements", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			source string
			data   string
		}{{
			source: "<?php __halt_compiler();\nany bytes ?> <?php $ $",
			data:   "\nany bytes ?> <?php $ $",
		}, {
			source: "<?php __halt_compiler() /* c */ ; trailing",
			data:   " trailing",
		}, {
			source: "<?php __halt_compiler();",
			data:   "",
		}, {
			source: "<?php echo 1; ?> html <?php __halt_compiler(); data",
			data:   " data",
		}} {
			file, err := Parse(tc.source)
			if err != nil {
				t.Fatalf("%s: %v", tc.source, err)
			}
			halt, ok := file.Stmts[len(file.Stmts)-1].(*HaltCompilerStmt)
			if !ok {
				t.Fatalf("%s: got %T", tc.source, file.Stmts[0])
			}
			if covered := tc.source[halt.Start:halt.End]; !strings.HasPrefix(covered, "__halt_compiler") || !strings.HasSuffix(covered, ";") {
				t.Errorf("%s: span %v covers %q", tc.source, halt.Span, covered)
			}
			if tc.data == "" {
				if halt.Data != nil {
					t.Errorf("%s: data %#v, want none", tc.source, halt.Data)
				}
			} else {
				if halt.Data == nil || halt.Data.Text != tc.data {
					t.Errorf("%s: data %#v, want %q", tc.source, halt.Data, tc.data)
				}
				want := len(tc.source) - len(tc.data)
				if halt.Data.Start != want || halt.Data.End != len(tc.source) {
					t.Errorf("%s: data span %v", tc.source, halt.Data.Span)
				}
			}
		}
	})

	t.Run("invalid placements", func(t *testing.T) {
		t.Parallel()
		sources := []string{
			"<?php function f() { __halt_compiler(); }",
			"<?php class C { function m() { __halt_compiler(); } }",
			"<?php if (true) { __halt_compiler(); }",
			"<?php if (true) __halt_compiler();",
			"<?php { __halt_compiler(); }",
			"<?php while (true): __halt_compiler(); endwhile;",
			"<?php namespace A { __halt_compiler(); }",
			"<?php $x = __halt_compiler();",
			"<?php $x = __halt_compiler;",
			"<?php function __halt_compiler() {}",
			"<?php class __halt_compiler {}",
			"<?php foo(__halt_compiler());",
			"<?php __halt_compiler;",
			"<?php __halt_compiler",
			"<?php __halt_compiler()",
			"<?php __halt_compiler() data",
			"<?php __halt_compiler(1);",
		}
		for _, source := range sources {
			_, err := Parse(source)
			if err == nil {
				t.Errorf("%s: expected error", source)
				continue
			}
			var parseErr *gast.ParseError
			if !errors.As(err, &parseErr) {
				t.Errorf("%s: expected *gast.ParseError, got %T", source, err)
				continue
			}
			if parseErr.Pos.Start < 0 || parseErr.Pos.End > len(source) || parseErr.Pos.Start >= parseErr.Pos.End && parseErr.Pos.End == len(source) && parseErr.Pos.Start != len(source) {
				t.Errorf("%s: error span %v out of range", source, parseErr.Pos)
			}
		}
	})
}

// partsOf parses `<?php $s = <literal>;` and returns the string parts.
func partsOf(t *testing.T, literal string) []StringPart {
	t.Helper()
	file, err := Parse("<?php $s = " + literal + ";")
	if err != nil {
		t.Fatalf("%s: %v", literal, err)
	}
	stmt := file.Stmts[0].(*ExpressionStmt)
	return stmt.Expr.(*AssignExpr).Value.(*StringLit).Parts
}

func TestParseInterpolationParts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		literal string
		want    []string // shape per part: "lit:<value>" or "expr:<raw>:<type>"
	}{
		{`"plain text"`, nil},
		{`"price 5$ and $$"`, nil},
		{`"escaped \$x stays raw"`, nil},
		{`'single $raw'`, nil},
		{`"$x"`, []string{"expr:$x:*php.Variable"}},
		{`"pre $x post"`, []string{"lit:pre ", "expr:$x:*php.Variable", "lit: post"}},
		{`"$$x"`, []string{"expr:$$x:*php.VariableVariable"}},
		{`"$obj->prop"`, []string{"expr:$obj->prop:*php.PropertyFetch"}},
		{`"$x->a->b"`, []string{"expr:$x->a:*php.PropertyFetch", "lit:->b"}},
		{`"$x?->p"`, []string{"expr:$x?->p:*php.NullsafePropertyFetch"}},
		{`"$x-> tail"`, []string{"expr:$x:*php.Variable", "lit:-> tail"}},
		{`"$arr[key]"`, []string{"expr:$arr[key]:*php.IndexExpr"}},
		{`"$arr[0]"`, []string{"expr:$arr[0]:*php.IndexExpr"}},
		{`"$arr[-1]"`, []string{"expr:$arr[-1]:*php.IndexExpr"}},
		{`"$arr[$i]"`, []string{"expr:$arr[$i]:*php.IndexExpr"}},
		{`"$arr[key][x]"`, []string{"expr:$arr[key]:*php.IndexExpr", "lit:[x]"}},
		{`"${name}"`, []string{"expr:${name}:*php.Variable"}},
		{`"${x[0]}"`, []string{"expr:${x[0]}:*php.IndexExpr"}},
		{`"${'lit'}"`, []string{"expr:${'lit'}:*php.VariableVariable"}},
		{`"${$v}"`, []string{"expr:${$v}:*php.VariableVariable"}},
		{`"{$a->b[1]}"`, []string{"expr:{$a->b[1]}:*php.IndexExpr"}},
		{`"{$o->m('}')}"`, []string{"expr:{$o->m('}')}:*php.MethodCall"}},
		{`"{$o->m()}"`, []string{"expr:{$o->m()}:*php.MethodCall"}},
		{`"{$x::$y}"`, []string{"expr:{$x::$y}:*php.StaticPropertyFetch"}},
		{`"{$x::$y()}"`, []string{"expr:{$x::$y()}:*php.CallExpr"}},
		{`"{$$v}"`, []string{"expr:{$$v}:*php.VariableVariable"}},
		{`"{@@$x}"`, []string{"lit:{@@", "expr:$x:*php.Variable", "lit:}"}},
		{`"{ $x } stays literal"`, []string{"lit:{ ", "expr:$x:*php.Variable", "lit: } stays literal"}},
		{`"a $x\nb"`, []string{"lit:a ", "expr:$x:*php.Variable", "lit:\nb"}},
		{"`cmd $v out`", []string{"lit:cmd ", "expr:$v:*php.Variable", "lit: out"}},
		{`"a $x\n\"q\" b"`, []string{"lit:a ", "expr:$x:*php.Variable", "lit:\n\\\"q\\\" b"}},
	}
	for _, tc := range tests {
		parts := partsOf(t, tc.literal)
		var got []string
		for _, part := range parts {
			switch p := part.(type) {
			case *LiteralPart:
				got = append(got, "lit:"+p.Value)
			case *ExprPart:
				got = append(got, "expr:"+p.Raw+":"+fmt.Sprintf("%T", p.Expr))
			}
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.literal, got, tc.want)
		}
	}
}

func TestParseInterpolationHeredoc(t *testing.T) {
	t.Parallel()
	parts := partsOf(t, "<<<EOT\nhead $x {$o->p} tail\nEOT")
	if len(parts) != 5 {
		t.Fatalf("heredoc parts: %#v", parts)
	}
	lit, ok := parts[0].(*LiteralPart)
	if !ok || lit.Value != "head " || lit.Start == 0 {
		t.Fatalf("heredoc literal part: %#v", parts[0])
	}
	if expr, ok := parts[2].(*LiteralPart); !ok || expr.Value != " " {
		t.Fatalf("heredoc middle part: %#v", parts[2])
	}
	if _, ok := parts[3].(*ExprPart); !ok {
		t.Fatalf("heredoc complex part: %#v", parts[3])
	}
	if expr, ok := parts[4].(*LiteralPart); !ok || expr.Value != " tail" {
		t.Fatalf("heredoc trailing part: %#v", parts[4])
	}
	if len(partsOf(t, "<<<'EOT'\nnowdoc $x raw\nEOT")) != 0 {
		t.Error("nowdoc must not interpolate")
	}
}

func TestParseInterpolationSpans(t *testing.T) {
	t.Parallel()
	source := `<?php $s = "a $x->p b";`
	file, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	lit := file.Stmts[0].(*ExpressionStmt).Expr.(*AssignExpr).Value.(*StringLit)
	if lit.Pos().Start != 11 || lit.Pos().End != 22 {
		t.Errorf("literal span: %v", lit.Pos())
	}
	part, ok := lit.Parts[1].(*ExprPart)
	if !ok {
		t.Fatalf("parts: %#v", lit.Parts)
	}
	if want := source[14:19]; part.Raw != want || part.Span != (gast.Span{Start: 14, End: 19}) {
		t.Errorf("expr part: raw %q span %v, want %q {14 19}", part.Raw, part.Span, want)
	}
}

func TestParseInterpolationInvalid(t *testing.T) {
	t.Parallel()
	literals := []string{
		`"${x"`,  // unterminated ${
		`"{$x"`,  // unterminated {$
		`"$x["`,  // unterminated index
		`"$x[]"`, // empty index
		`"$x[1a]"`,
		`"{$x . 'c'}"`, // binary operator in {$...}
		`"{$x + 1}"`,
		`"{$1}"`,    // literal root
		`"{$ x}"`,   // space after {$
		`"{$x::C}"`, // bare name after ::
		`"{$x ?: 1}"`,
		`"{$@$x}"`,
		`"${x junk}"`,
		`"{$x = 1}"`,
	}
	for _, literal := range literals {
		if err := Validate("<?php $s = " + literal + ";"); err == nil {
			t.Errorf("%s: expected error", literal)
		}
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
