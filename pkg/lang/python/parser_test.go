package python

import (
	"strings"
	"testing"

	"github.com/stubbedev/gast/pkg/gast"
)

// exprOf parses source as a module and returns its single expression
// statement's expression.
func exprOf(t *testing.T, source string) Expr {
	t.Helper()
	file, err := Parse(source + "\n")
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

func stmtOf(t *testing.T, source string) Stmt {
	t.Helper()
	file, err := Parse(source)
	if err != nil {
		t.Fatalf("%s: %v", source, err)
	}
	if len(file.Stmts) != 1 {
		t.Fatalf("%s: got %d statements", source, len(file.Stmts))
	}
	return file.Stmts[0]
}

func assertBinary(t *testing.T, expr Expr, op string) (Expr, Expr) {
	t.Helper()
	binary, ok := expr.(*BinOpExpr)
	if !ok {
		t.Fatalf("expected *BinOpExpr, got %T", expr)
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
	if lit, ok := left.(*IntLit); !ok || lit.Text != "1" {
		t.Fatalf("left: %#v", left)
	}
	_, _ = assertBinary(t, right, Star)

	// 2 ** 3 ** 4 groups right: 2 ** (3 ** 4)
	_, right = assertBinary(t, exprOf(t, "2 ** 3 ** 4"), DoubleStar)
	_, _ = assertBinary(t, right, DoubleStar)

	// -2 ** 2 binds as -(2 ** 2): unary binds looser than ** on the right
	unary, ok := exprOf(t, "-2 ** 2").(*UnaryOpExpr)
	if !ok || unary.Op != Minus {
		t.Fatalf("expected unary minus, got %#v", unary)
	}
	_, _ = assertBinary(t, unary.Operand, DoubleStar)

	// 2 ** -1 admits a unary exponent
	_, right = assertBinary(t, exprOf(t, "2 ** -1"), DoubleStar)
	if exp, ok := right.(*UnaryOpExpr); !ok || exp.Op != Minus {
		t.Fatalf("unary exponent: %#v", right)
	}

	// not binds looser than comparison
	not, ok := exprOf(t, "not a == b").(*UnaryOpExpr)
	if !ok || not.Op != KindNot {
		t.Fatalf("expected not, got %#v", not)
	}
	if _, ok := not.Operand.(*CompareExpr); !ok {
		t.Fatalf("not operand: %T", not.Operand)
	}

	// 1 | 2 ^ 3 & 4 groups as 1 | (2 ^ (3 & 4))
	_, right = assertBinary(t, exprOf(t, "1 | 2 ^ 3 & 4"), Pipe)
	_, right = assertBinary(t, right, Caret)
	_, _ = assertBinary(t, right, Ampersand)

	// additive binds tighter than shift
	_, right = assertBinary(t, exprOf(t, "1 << 2 + 3"), LeftShift)
	_, _ = assertBinary(t, right, Plus)

	// a and b or c groups as (a and b) or c
	orNode, ok := exprOf(t, "a and b or c").(*BoolOpExpr)
	if !ok || orNode.Op != KindOr {
		t.Fatalf("or node: %#v", orNode)
	}
	andNode, ok := orNode.Values[0].(*BoolOpExpr)
	if !ok || andNode.Op != KindAnd || len(andNode.Values) != 2 {
		t.Fatalf("and chain: %#v", orNode.Values[0])
	}

	// chained and flattens
	flat, ok := exprOf(t, "a and b and c").(*BoolOpExpr)
	if !ok || len(flat.Values) != 3 {
		t.Fatalf("flattened and: %#v", flat)
	}
}

func TestParseChainedComparison(t *testing.T) {
	t.Parallel()

	// a < b <= c is ONE Compare node with two ops
	compare, ok := exprOf(t, "a < b <= c").(*CompareExpr)
	if !ok {
		t.Fatalf("got %T", compare)
	}
	if len(compare.Ops) != 2 || compare.Ops[0] != Less || compare.Ops[1] != LessEqual {
		t.Fatalf("ops: %v", compare.Ops)
	}
	if len(compare.Comparators) != 2 {
		t.Fatalf("comparators: %d", len(compare.Comparators))
	}

	for source, want := range map[string]string{
		"a not in b": opNotIn,
		"a is not b": opIsNot,
		"a is b":     opIs,
		"a in b":     opIn,
	} {
		compare, ok := exprOf(t, source).(*CompareExpr)
		if !ok {
			t.Fatalf("%s: got %T", source, compare)
		}
		if len(compare.Ops) != 1 || compare.Ops[0] != want {
			t.Fatalf("%s: ops %v, want [%s]", source, compare.Ops, want)
		}
	}
}

func TestParseTuplesAndGrouping(t *testing.T) {
	t.Parallel()
	tuple, ok := exprOf(t, "1, 2").(*TupleExpr)
	if !ok || tuple.Parenthesized || len(tuple.Elts) != 2 {
		t.Fatalf("unparenthesized tuple: %#v", tuple)
	}
	paren, ok := exprOf(t, "(1, 2)").(*TupleExpr)
	if !ok || !paren.Parenthesized {
		t.Fatalf("parenthesized tuple: %#v", paren)
	}
	if _, ok := exprOf(t, "(1)").(*IntLit); !ok {
		t.Fatal("grouped int")
	}
	if _, ok := exprOf(t, "()").(*TupleExpr); !ok {
		t.Fatal("empty tuple")
	}
	single, ok := exprOf(t, "1,").(*TupleExpr)
	if !ok || len(single.Elts) != 1 {
		t.Fatalf("single tuple: %#v", single)
	}
}

func TestParseStringsAndConcat(t *testing.T) {
	t.Parallel()
	lit, ok := exprOf(t, "'a' \"b\"").(*StringLit)
	if !ok || lit.Value != "ab" {
		t.Fatalf("concatenated literals: %#v", lit)
	}
	escaped, ok := exprOf(t, `"a\tb\x41\n"`).(*StringLit)
	if !ok || escaped.Value != "a\tbA\n" {
		t.Fatalf("escapes: %q", escaped.Value)
	}
	raw, ok := exprOf(t, `r"a\nb"`).(*StringLit)
	if !ok || raw.Value != `a\nb` {
		t.Fatalf("raw: %q", raw.Value)
	}
}

func TestParseFStrings(t *testing.T) {
	t.Parallel()
	joined, ok := exprOf(t, `f"a{x}b"`).(*JoinedStr)
	if !ok || len(joined.Parts) != 3 {
		t.Fatalf("parts: %#v", joined)
	}
	if lit, ok := joined.Parts[0].(*StringLit); !ok || lit.Value != "a" {
		t.Fatalf("leading literal: %#v", joined.Parts[0])
	}
	field, ok := joined.Parts[1].(*FormattedValue)
	if !ok {
		t.Fatalf("field: %#v", joined.Parts[1])
	}
	if name, ok := field.Value.(*NameExpr); !ok || name.ID != "x" {
		t.Fatalf("field value: %#v", field.Value)
	}

	spec, ok := exprOf(t, `f"{x:.2f}"`).(*JoinedStr)
	if !ok {
		t.Fatalf("got %T", spec)
	}
	field = spec.Parts[0].(*FormattedValue)
	if field.FormatSpec == nil {
		t.Fatal("missing format spec")
	}
	if lit, ok := field.FormatSpec.(*JoinedStr).Parts[0].(*StringLit); !ok || lit.Value != ".2f" {
		t.Fatalf("spec: %#v", field.FormatSpec)
	}

	conv, ok := exprOf(t, `f"{x!r:>{w}}"`).(*JoinedStr)
	if !ok {
		t.Fatalf("got %T", conv)
	}
	field = conv.Parts[0].(*FormattedValue)
	if field.Conversion != "r" {
		t.Fatalf("conversion: %q", field.Conversion)
	}
	specJoined, ok := field.FormatSpec.(*JoinedStr)
	if !ok || len(specJoined.Parts) != 2 {
		t.Fatalf("nested spec: %#v", field.FormatSpec)
	}
	nested, ok := specJoined.Parts[1].(*FormattedValue)
	if !ok {
		t.Fatalf("nested field: %#v", specJoined.Parts[1])
	}
	if name, ok := nested.Value.(*NameExpr); !ok || name.ID != "w" {
		t.Fatalf("nested field value: %#v", nested.Value)
	}

	debug, ok := exprOf(t, `f"{x=}"`).(*JoinedStr)
	if !ok {
		t.Fatalf("got %T", debug)
	}
	if field = debug.Parts[0].(*FormattedValue); !field.Debug {
		t.Fatalf("debug flag: %#v", field)
	}

	// PEP 701: nested same quotes.
	nestedQuotes, err := Parse("d = {}\ny = f\"{d['k']}\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(nestedQuotes.Stmts) != 2 {
		t.Fatalf("statements: %d", len(nestedQuotes.Stmts))
	}
}

func TestParseCallsAndSubscripts(t *testing.T) {
	t.Parallel()
	call, ok := exprOf(t, "f(a, b=1, *c, **d)").(*CallExpr)
	if !ok || len(call.Args) != 4 {
		t.Fatalf("args: %#v", call)
	}
	if call.Args[1].Name != "b" || call.Args[2].Star != Star || call.Args[3].Star != DoubleStar {
		t.Fatalf("arg flags: %#v", call.Args[1:])
	}

	sub, ok := exprOf(t, "x[1:2:3, ::-1]").(*SubscriptExpr)
	if !ok {
		t.Fatalf("got %T", sub)
	}
	tuple, ok := sub.Index.(*TupleExpr)
	if !ok || len(tuple.Elts) != 2 {
		t.Fatalf("index: %#v", sub.Index)
	}
	slice, ok := tuple.Elts[0].(*SliceExpr)
	if !ok {
		t.Fatalf("slice: %T", tuple.Elts[0])
	}
	if lit, ok := slice.Lower.(*IntLit); !ok || lit.Text != "1" {
		t.Fatalf("slice lower: %#v", slice.Lower)
	}
	if _, ok := tuple.Elts[1].(*SliceExpr); !ok {
		t.Fatalf("step-only slice: %T", tuple.Elts[1])
	}
	if step, ok := tuple.Elts[1].(*SliceExpr); ok {
		if un, ok := step.Step.(*UnaryOpExpr); !ok || un.Op != Minus {
			t.Fatalf("step: %#v", step.Step)
		}
	}

	plain, ok := exprOf(t, "x[k]").(*SubscriptExpr)
	if !ok {
		t.Fatalf("got %T", plain)
	}
	if _, ok := plain.Index.(*NameExpr); !ok {
		t.Fatalf("plain index: %T", plain.Index)
	}
}

func TestParseComprehensions(t *testing.T) {
	t.Parallel()
	for source, want := range map[string]string{
		"[x for x in y]":       "list",
		"{x for x in y}":       "set",
		"{x: 1 for x in y}":    "dict",
		"(x for x in y)":       "generator",
		"[x async for x in y]": "list",
	} {
		comp, ok := exprOf(t, source).(*ComprehensionExpr)
		if !ok {
			t.Fatalf("%s: got %T", source, comp)
		}
		if comp.Kind != want {
			t.Fatalf("%s: kind %q, want %q", source, comp.Kind, want)
		}
	}
	nested, ok := exprOf(t, "[a for b in c for d in e if f if g]").(*ComprehensionExpr)
	if !ok || len(nested.Generators) != 2 || len(nested.Generators[1].Ifs) != 2 {
		t.Fatalf("nested clauses: %#v", nested)
	}
	if nested.Generators[0].Target.Pos().End == 0 {
		t.Fatal("clause span")
	}
}

func TestParseAssignmentForms(t *testing.T) {
	t.Parallel()
	assign, ok := stmtOf(t, "a = b = 1\n").(*AssignStmt)
	if !ok || len(assign.Targets) != 2 {
		t.Fatalf("chained targets: %#v", assign)
	}
	starred, ok := stmtOf(t, "a, *rest = seq\n").(*AssignStmt)
	if !ok {
		t.Fatalf("got %T", starred)
	}
	tuple, ok := starred.Targets[0].(*TupleExpr)
	if !ok || len(tuple.Elts) != 2 {
		t.Fatalf("star target: %#v", starred.Targets[0])
	}
	if _, ok := tuple.Elts[1].(*StarredExpr); !ok {
		t.Fatalf("star element: %#v", tuple.Elts[1])
	}

	aug, ok := stmtOf(t, "x //= 2\n").(*AugAssignStmt)
	if !ok || aug.Op != FloorDivAssign {
		t.Fatalf("aug: %#v", aug)
	}

	ann, ok := stmtOf(t, "count: int = 0\n").(*AnnAssignStmt)
	if !ok || !ann.Simple || ann.Value == nil {
		t.Fatalf("ann: %#v", ann)
	}

	if err := Validate("f(x) = 1\n"); err == nil {
		t.Error("call target: expected error")
	}
	if err := Validate("1 = x\n"); err == nil {
		t.Error("literal target: expected error")
	}
	if err := Validate("a, b += 1\n"); err == nil {
		t.Error("tuple aug target: expected error")
	}
	if err := Validate("x := 1\n"); err == nil {
		t.Error("bare walrus statement: expected error")
	}
}

func TestParseWalrus(t *testing.T) {
	t.Parallel()
	named, ok := exprOf(t, "(x := 1)").(*NamedExpr)
	if !ok {
		t.Fatalf("got %T", named)
	}
	if name, ok := named.Target.(*NameExpr); !ok || name.ID != "x" {
		t.Fatalf("target: %#v", named.Target)
	}
	whileStmt, ok := stmtOf(t, "while chunk := read():\n    pass\n").(*WhileStmt)
	if !ok {
		t.Fatalf("got %T", whileStmt)
	}
	if _, ok := whileStmt.Cond.(*NamedExpr); !ok {
		t.Fatalf("while cond: %T", whileStmt.Cond)
	}
	if err := Validate("a + (b := 1) + c := 2\n"); err == nil {
		t.Error("walrus at wrong level: expected error")
	}
}

func TestParseLambdasAndConditionals(t *testing.T) {
	t.Parallel()
	lambda, ok := exprOf(t, "lambda x, y=1: x + y").(*LambdaExpr)
	if !ok {
		t.Fatalf("got %T", lambda)
	}
	if len(lambda.Params.PosOrKeyword) != 2 || lambda.Params.PosOrKeyword[1].Default == nil {
		t.Fatalf("params: %#v", lambda.Params)
	}

	ternary, ok := exprOf(t, "a if b else c if d else e").(*IfExpExpr)
	if !ok {
		t.Fatalf("got %T", ternary)
	}
	if _, ok := ternary.Orelse.(*IfExpExpr); !ok {
		t.Fatalf("nested else: %T", ternary.Orelse)
	}
}

func TestParseYield(t *testing.T) {
	t.Parallel()
	file, err := Parse("def g():\n    yield 1, 2\n    yield from h()\n")
	if err != nil {
		t.Fatal(err)
	}
	fn := file.Stmts[0].(*FunctionDef)
	first := fn.Body[0].(*ExpressionStmt).Expr.(*YieldExpr)
	if _, ok := first.Value.(*TupleExpr); !ok {
		t.Fatalf("yield tuple: %T", first.Value)
	}
	second := fn.Body[1].(*ExpressionStmt).Expr.(*YieldFromExpr)
	if _, ok := second.Value.(*CallExpr); !ok {
		t.Fatalf("yield from: %T", second.Value)
	}
}

func TestParseMatchPatterns(t *testing.T) {
	t.Parallel()
	match, ok := stmtOf(t, "match x:\n    case Color.RED:\n        pass\n").(*MatchStmt)
	if !ok || len(match.Cases) != 1 {
		t.Fatalf("match: %#v", match)
	}
	if _, ok := match.Cases[0].Pattern.(*MatchValue); !ok {
		t.Fatalf("value pattern: %T", match.Cases[0].Pattern)
	}

	match, ok = stmtOf(t, "match x:\n    case [1, *rest] as seq if seq:\n        pass\n").(*MatchStmt)
	if !ok {
		t.Fatalf("got %T", match)
	}
	as, ok := match.Cases[0].Pattern.(*MatchAs)
	if !ok {
		t.Fatalf("as pattern: %T", match.Cases[0].Pattern)
	}
	seq, ok := as.Pattern.(*MatchSequence)
	if !ok || len(seq.Patterns) != 2 {
		t.Fatalf("sequence: %#v", as.Pattern)
	}
	if _, ok := seq.Patterns[1].(*MatchStar); !ok {
		t.Fatalf("star pattern: %T", seq.Patterns[1])
	}
	if match.Cases[0].Guard == nil {
		t.Fatal("guard")
	}

	match, _ = stmtOf(t, "match x:\n    case 1 | 2:\n        pass\n").(*MatchStmt)
	if _, ok := match.Cases[0].Pattern.(*MatchOr); !ok {
		t.Fatalf("or pattern: %T", match.Cases[0].Pattern)
	}

	match, _ = stmtOf(t, "match x:\n    case P(a, k=1):\n        pass\n").(*MatchStmt)
	cls, ok := match.Cases[0].Pattern.(*MatchClass)
	if !ok || len(cls.Patterns) != 1 || len(cls.KwdNames) != 1 {
		t.Fatalf("class pattern: %#v", cls)
	}

	match, _ = stmtOf(t, "match x:\n    case {'k': v, **rest}:\n        pass\n").(*MatchStmt)
	mapping, ok := match.Cases[0].Pattern.(*MatchMapping)
	if !ok || len(mapping.Keys) != 1 || mapping.Rest != "rest" {
		t.Fatalf("mapping pattern: %#v", mapping)
	}

	if err := Validate("match x:\n    case *_:\n        pass\n"); err == nil {
		t.Error("wildcard star: expected error")
	}
}

func TestParseSemicolonLine(t *testing.T) {
	t.Parallel()
	seq, ok := stmtOf(t, "a = 1; b = 2; c = 3\n").(*SeqStmt)
	if !ok || len(seq.Stmts) != 3 {
		t.Fatalf("seq: %#v", seq)
	}
}

func TestParseSpans(t *testing.T) {
	t.Parallel()
	file, err := Parse("def f(a, b):\n    return a\n")
	if err != nil {
		t.Fatal(err)
	}
	fn := file.Stmts[0].(*FunctionDef)
	if fn.Span != (gast.Span{Start: 0, End: 26}) {
		t.Fatalf("function span: %v", fn.Span)
	}
	if fn.Name != "f" || len(fn.Params.PosOrKeyword) != 2 {
		t.Fatalf("function: %#v", fn)
	}

	assign := stmtOf(t, "x = 1 + 2\n").(*AssignStmt)
	if assign.Span != (gast.Span{Start: 0, End: 9}) {
		t.Fatalf("assign span: %v", assign.Span)
	}

	bin := exprOf(t, "1 + 2").(*BinOpExpr)
	if bin.Span != (gast.Span{Start: 0, End: 5}) {
		t.Fatalf("binary span: %v", bin.Span)
	}
}

func TestParseWithItems(t *testing.T) {
	t.Parallel()
	with, ok := stmtOf(t, "with (open(a) as f, open(b) as g):\n    pass\n").(*WithStmt)
	if !ok || len(with.Items) != 2 || with.Items[0].OptionalVars == nil {
		t.Fatalf("paren items: %#v", with)
	}
	with = stmtOf(t, "with (a, b), c as d:\n    pass\n").(*WithStmt)
	if len(with.Items) != 2 {
		t.Fatalf("mixed items: %#v", with.Items)
	}
	if _, ok := with.Items[0].Context.(*TupleExpr); !ok {
		t.Fatalf("tuple context: %T", with.Items[0].Context)
	}
}

func TestFormatErrorCaret(t *testing.T) {
	t.Parallel()
	source := "x = 1\n1 = x\n"
	err := Validate(source)
	if err == nil {
		t.Fatal("expected error")
	}
	got := gast.FormatError(err, source)
	want := strings.Join([]string{
		"ParseError at 2:1: cannot assign to literal",
		"2 | 1 = x",
		"  | ^",
	}, "\n")
	if got != want {
		t.Fatalf("FormatError:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
