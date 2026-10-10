package js

import (
	"errors"
	"testing"

	"github.com/stubbedev/gast/pkg/gast"
)

// exprOf parses `<source>;` and returns the single expression.
func exprOf(t *testing.T, source string) Expr {
	t.Helper()
	file, err := Parse(source + ";")
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
	if lit, ok := left.(*NumberLit); !ok || lit.Text != "1" {
		t.Fatalf("left: %#v", left)
	}
	_, _ = assertBinary(t, right, Asterisk)

	// a - b - c groups left
	left, _ = assertBinary(t, exprOf(t, "a - b - c"), Minus)
	if _, ok := left.(*BinaryExpr); !ok {
		t.Fatalf("left assoc: %T", left)
	}

	// 2 ** 3 ** 4 groups right
	_, right = assertBinary(t, exprOf(t, "2 ** 3 ** 4"), Pow)
	_, _ = assertBinary(t, right, Pow)

	// a ** b ** c and unary restriction
	if err := Validate("-a ** 2;"); err == nil {
		t.Error("unary left of '**': expected error")
	}
	if err := Validate("(-a) ** 2;"); err != nil {
		t.Errorf("parenthesized unary left of '**': %v", err)
	}

	// ?? is left-associative per the spec grammar
	left, _ = assertBinary(t, exprOf(t, "a ?? b ?? c"), Nullish)
	_, _ = assertBinary(t, left, Nullish)

	// 1 && 2 || 3 groups as (1 && 2) || 3
	left, _ = assertBinary(t, exprOf(t, "a && b || c"), LogicalOr)
	_, _ = assertBinary(t, left, LogicalAnd)

	// shift binds tighter than relational
	left, _ = assertBinary(t, exprOf(t, "a << b < c"), Lt)
	_, _ = assertBinary(t, left, ShiftLeft)
}

func TestParseTernary(t *testing.T) {
	t.Parallel()
	ternary, ok := exprOf(t, "a ? b : c ? d : e").(*CondExpr)
	if !ok {
		t.Fatalf("got %T", ternary)
	}
	if _, ok := ternary.Else.(*CondExpr); !ok {
		t.Fatalf("nested ternary: %T", ternary.Else)
	}
	if err := Validate("a ? b;"); err == nil {
		t.Error("ternary missing colon: expected error")
	}
}

func TestParseAssignment(t *testing.T) {
	t.Parallel()
	for source, op := range map[string]string{
		"a = 1":    Assign,
		"a += 1":   AddAssign,
		"a **= 2":  PowAssign,
		"a ??= b":  NullishAssign,
		"a >>>= b": UnsignedShiftAssign,
		"a ||= b":  LogicalOrAssign,
	} {
		assign, ok := exprOf(t, source).(*AssignExpr)
		if !ok {
			t.Fatalf("%s: got %T", source, exprOf(t, source))
		}
		if assign.Op != op {
			t.Fatalf("%s: op %q, want %q", source, assign.Op, op)
		}
	}

	// chained assignment groups right
	chained, ok := exprOf(t, "a = b = 1").(*AssignExpr)
	if !ok {
		t.Fatalf("chained: %T", chained)
	}
	if _, ok := chained.Value.(*AssignExpr); !ok {
		t.Fatalf("right side of chained assignment: %T", chained.Value)
	}

	// invalid targets
	for _, source := range []string{"1 = 2", "(1) = 2", `"s" = 2`, "a + b = 2", "f() = 1", "({a} += b)"} {
		if err := Validate(source + ";"); err == nil {
			t.Errorf("%s: expected invalid assignment target error", source)
		}
	}
	// valid odd-looking targets
	for _, source := range []string{"(a) = 2", "({a: obj.k} = src)", "[a[0]] = b", "({a = 1} = src)"} {
		if err := Validate(source + ";"); err != nil {
			t.Errorf("%s: %v", source, err)
		}
	}
}

func TestParsePostfixAndUpdate(t *testing.T) {
	t.Parallel()
	post, ok := exprOf(t, "a++").(*UpdateExpr)
	if !ok || post.Prefix {
		t.Fatalf("got %#v", post)
	}
	pre, ok := exprOf(t, "++a").(*UpdateExpr)
	if !ok || !pre.Prefix {
		t.Fatalf("got %#v", pre)
	}
	for _, source := range []string{"1++", "++1"} {
		if err := Validate(source + ";"); err == nil {
			t.Errorf("%s: expected invalid operand", source)
		}
	}
	// sequence and in-operator
	seq, ok := exprOf(t, "a, b, c").(*SequenceExpr)
	if !ok || len(seq.Exprs) != 3 {
		t.Fatalf("got %#v", seq)
	}
}

func TestParseSubscripts(t *testing.T) {
	t.Parallel()
	chain, ok := exprOf(t, "a.b[c]?.d(e).f`t`").(*TaggedTemplate)
	if !ok {
		t.Fatalf("got %T", chain)
	}
	tag, ok := chain.Tag.(*MemberExpr)
	if !ok {
		t.Fatalf("tag: %T", chain.Tag)
	}
	call, ok := tag.Obj.(*CallExpr)
	if !ok {
		t.Fatalf("tag object: %T", tag.Obj)
	}
	member, ok := call.Callee.(*MemberExpr)
	if !ok || !member.Optional {
		t.Fatalf("callee: %#v", member)
	}
	index, ok := member.Obj.(*MemberExpr)
	if !ok || !index.Computed || index.Optional {
		t.Fatalf("computed object: %#v", index)
	}
	if len(chain.Quasis) != 1 || chain.Quasis[0] != "t" {
		t.Fatalf("quasis: %#v", chain.Quasis)
	}

	optionalCall, ok := exprOf(t, "a?.(1)").(*CallExpr)
	if !ok || !optionalCall.Optional {
		t.Fatalf("got %#v", optionalCall)
	}
	optionalIndex, ok := exprOf(t, "a?.[0]").(*MemberExpr)
	if !ok || !optionalIndex.Optional || !optionalIndex.Computed {
		t.Fatalf("got %#v", optionalIndex)
	}
}

func TestParseNew(t *testing.T) {
	t.Parallel()
	// new without an argument list: Args is nil
	bare, ok := exprOf(t, "new Foo").(*NewExpr)
	if !ok {
		t.Fatalf("got %T", bare)
	}
	if bare.Args != nil {
		t.Fatalf("bare new args: %#v", bare.Args)
	}
	// new (a)(b) applies the list to new
	paren, ok := exprOf(t, "new (a)(b)").(*NewExpr)
	if !ok || paren.Args == nil {
		t.Fatalf("got %#v", paren)
	}
	// new new A()(): outer new over inner new with args
	nested, ok := exprOf(t, "new new A()()").(*NewExpr)
	if !ok || nested.Args == nil {
		t.Fatalf("got %#v", nested)
	}
	if inner, ok := nested.Callee.(*NewExpr); !ok || inner.Args == nil {
		t.Fatalf("inner: %#v", nested.Callee)
	}
	meta, ok := exprOf(t, "new.target").(*MetaProperty)
	if !ok || meta.Meta != "new" || meta.Prop != "target" {
		t.Fatalf("got %#v", meta)
	}
}

func TestParseFunctionsAndArrows(t *testing.T) {
	t.Parallel()
	if err := Validate("function* g(a, b = 1, ...c) { yield a; }"); err != nil {
		t.Fatal(err)
	}
	fn, ok := exprOf(t, "(function* g(a, b = 1, ...c) { yield a; })").(*ParenExpr)
	if !ok {
		t.Fatalf("got %T", fn)
	}
	if inner, ok := fn.Expr.(*FunctionExpr); !ok || !inner.Generator || len(inner.Params) != 3 || !inner.Params[2].Rest {
		t.Fatalf("generator function: %#v", fn.Expr)
	}
	anon, ok := exprOf(t, "(function () {})").(*ParenExpr)
	if !ok {
		t.Fatalf("got %T", anon)
	}
	asyncFn, ok := exprOf(t, "(async function f() { await g(); })").(*ParenExpr)
	if !ok {
		t.Fatalf("got %T", asyncFn)
	}
	if inner, ok := asyncFn.Expr.(*FunctionExpr); !ok || !inner.Async {
		t.Fatalf("async function: %#v", asyncFn.Expr)
	}

	arrow, ok := exprOf(t, "(a, b) => a + b").(*ArrowExpr)
	if !ok || !arrow.Expression || len(arrow.Params) != 2 {
		t.Fatalf("got %#v", arrow)
	}
	single, ok := exprOf(t, "x => x * 2").(*ArrowExpr)
	if !ok || len(single.Params) != 1 {
		t.Fatalf("got %#v", single)
	}
	asyncArrow, ok := exprOf(t, "async x => await x").(*ArrowExpr)
	if !ok || !asyncArrow.Async {
		t.Fatalf("got %#v", asyncArrow)
	}
	asyncParen, ok := exprOf(t, "async (a) => await a").(*ArrowExpr)
	if !ok || !asyncParen.Async || len(asyncParen.Params) != 1 {
		t.Fatalf("got %#v", asyncParen)
	}
	block, ok := exprOf(t, "x => { return x; }").(*ArrowExpr)
	if !ok || block.Expression {
		t.Fatalf("got %#v", block)
	}
	// param named async
	named, ok := exprOf(t, "async => async").(*ArrowExpr)
	if !ok || named.Async {
		t.Fatalf("got %#v", named)
	}
	// call that is not an arrow
	call, ok := exprOf(t, "async (a)").(*CallExpr)
	if !ok {
		t.Fatalf("got %T", call)
	}
	// no line break before =>
	if err := Validate("const f = (a, b)\n=> a + b;"); err == nil {
		t.Error("newline before '=>': expected error")
	}
	// destructuring params
	destructured, ok := exprOf(t, "({a, b: {c = 1}}, [d, ...e]) => a").(*ArrowExpr)
	if !ok || len(destructured.Params) != 2 {
		t.Fatalf("got %#v", destructured)
	}
}

func TestParseYieldAwait(t *testing.T) {
	t.Parallel()
	file, err := Parse("function* g() { yield 1; yield* it; const x = yield; }")
	if err != nil {
		t.Fatal(err)
	}
	decl := file.Stmts[0].(*FunctionDecl)
	if !decl.Generator {
		t.Fatalf("got %#v", decl)
	}
	body := decl.Body.Stmts[0].(*ExpressionStmt).Expr.(*YieldExpr)
	if body.Arg == nil {
		t.Fatalf("yield arg: %#v", body)
	}
	delegate := decl.Body.Stmts[1].(*ExpressionStmt).Expr.(*YieldExpr)
	if !delegate.Delegate {
		t.Fatalf("yield*: %#v", delegate)
	}
	bare := decl.Body.Stmts[2].(*VarStmt).Decls[0].Init.(*YieldExpr)
	if bare.Arg != nil {
		t.Fatalf("bare yield: %#v", bare)
	}
	// yield is a plain identifier outside generators
	if err := Validate("var yield = 1;"); err != nil {
		t.Errorf("yield as identifier: %v", err)
	}
	// restricted production: a newline after yield ends it with no
	// argument, so the next line is a separate statement
	if err := Validate("function* g() {\n yield\n 1;\n}"); err != nil {
		t.Errorf("yield with newline: %v", err)
	}
}

func TestParseTemplateAST(t *testing.T) {
	t.Parallel()
	tpl, ok := exprOf(t, "`a${b}c${`d${e}`}f`").(*TemplateLit)
	if !ok {
		t.Fatalf("got %T", tpl)
	}
	if len(tpl.Quasis) != 3 || len(tpl.Exprs) != 2 {
		t.Fatalf("quasis %v exprs %v", tpl.Quasis, tpl.Exprs)
	}
	if tpl.Quasis[0] != "a" || tpl.Quasis[1] != "c" || tpl.Quasis[2] != "f" {
		t.Fatalf("quasis: %v", tpl.Quasis)
	}
	inner, ok := tpl.Exprs[1].(*TemplateLit)
	if !ok || inner.Quasis[0] != "d" {
		t.Fatalf("inner template: %#v", tpl.Exprs[1])
	}
}

func TestParseClasses(t *testing.T) {
	t.Parallel()
	file, err := Parse("class A extends B { static #n = 1; #m() {} get x() {} static { init(); } async *gen() {} constructor() { super(); } }")
	if err != nil {
		t.Fatal(err)
	}
	decl, ok := file.Stmts[0].(*ClassDecl)
	if !ok || decl.Name != "A" {
		t.Fatalf("got %#v", file.Stmts[0])
	}
	if _, ok := decl.Extends.(*IdentExpr); !ok {
		t.Fatalf("extends: %#v", decl.Extends)
	}
	if len(decl.Body) != 6 {
		t.Fatalf("members: %d", len(decl.Body))
	}
	field, ok := decl.Body[0].(*PropertyDef)
	if !ok || !field.Static {
		t.Fatalf("field: %#v", decl.Body[0])
	}
	if priv, ok := field.Key.(*PrivateName); !ok || priv.Name != "n" {
		t.Fatalf("private field: %#v", field.Key)
	}
	method, ok := decl.Body[1].(*MethodDef)
	if !ok || method.Kind != "method" {
		t.Fatalf("method: %#v", decl.Body[1])
	}
	ctor, ok := decl.Body[5].(*MethodDef)
	if !ok || ctor.Kind != "constructor" {
		t.Fatalf("ctor: %#v", decl.Body[5])
	}
	gen, ok := decl.Body[4].(*MethodDef)
	if !ok || !gen.Async || !gen.Generator {
		t.Fatalf("async generator: %#v", gen)
	}
	if _, ok := decl.Body[3].(*StaticBlock); !ok {
		t.Fatalf("static block: %#v", decl.Body[3])
	}
	// getter/setter arity is checked
	if err := Validate("class A { get x(a) {} }"); err == nil {
		t.Error("getter with parameters: expected error")
	}
	if err := Validate("class A { set x() {} }"); err == nil {
		t.Error("setter without parameters: expected error")
	}
}

func TestParseDestructuring(t *testing.T) {
	t.Parallel()
	file, err := Parse("const [a, , b = 1, ...c] = xs;")
	if err != nil {
		t.Fatal(err)
	}
	varStmt := file.Stmts[0].(*VarStmt)
	if varStmt.Kind != "const" || len(varStmt.Decls) != 1 {
		t.Fatalf("got %#v", varStmt)
	}
	pattern, ok := varStmt.Decls[0].Binding.(*ArrayPattern)
	if !ok || len(pattern.Elements) != 4 {
		t.Fatalf("elements: %#v", pattern)
	}
	if pattern.Elements[1] != nil {
		t.Fatalf("hole: %#v", pattern.Elements[1])
	}
	if _, ok := pattern.Elements[2].(*DefaultPattern); !ok {
		t.Fatalf("default: %#v", pattern.Elements[2])
	}
	if _, ok := pattern.Elements[3].(*RestElement); !ok {
		t.Fatalf("rest: %#v", pattern.Elements[3])
	}

	file, err = Parse("const {a, b: {c}, d = 1, ...e} = o;")
	if err != nil {
		t.Fatal(err)
	}
	object := file.Stmts[0].(*VarStmt).Decls[0].Binding.(*ObjectPattern)
	if len(object.Props) != 4 || !object.Props[3].Rest || object.Props[3].Value == nil {
		t.Fatalf("props: %#v", object)
	}
	if _, ok := object.Props[1].Value.(*ObjectPattern); !ok {
		t.Fatalf("nested: %#v", object.Props[1])
	}
}

func TestParseFor(t *testing.T) {
	t.Parallel()
	file, err := Parse("for (let i = 0, j = 1; i < j; i++) {}")
	if err != nil {
		t.Fatal(err)
	}
	classic := file.Stmts[0].(*ForStmt)
	init := classic.Init.(*VarStmt)
	if init.Kind != "let" || len(init.Decls) != 2 {
		t.Fatalf("init: %#v", init)
	}
	file, err = Parse("for (const x of xs) {}")
	if err != nil {
		t.Fatal(err)
	}
	of := file.Stmts[0].(*ForOfStmt)
	if _, ok := of.Left.(*VarStmt); !ok {
		t.Fatalf("left: %#v", of.Left)
	}
	file, err = Parse("async function f() { for await (const c of s) {} }")
	if err != nil {
		t.Fatal(err)
	}
	fn := file.Stmts[0].(*FunctionDecl)
	awaitOf := fn.Body.Stmts[0].(*ForOfStmt)
	if !awaitOf.Await {
		t.Fatalf("for await: %#v", awaitOf)
	}
	// let as identifier inside the head
	if err = Validate("for (let of;;) break;"); err != nil {
		t.Errorf("let as identifier in for head: %v", err)
	}
	if err = Validate("for (let in obj) break;"); err != nil {
		t.Errorf("let as identifier in for-in: %v", err)
	}
	// in is not a comparison inside the init
	file, err = Parse("for (x = 'a' in b;;) {}")
	if err != nil {
		t.Fatal(err)
	}
	assign := file.Stmts[0].(*ForStmt).Init.(*ExpressionStmt).Expr.(*AssignExpr)
	if _, ok := assign.Value.(*BinaryExpr); !ok || assign.Value.(*BinaryExpr).Op != KindIn {
		t.Fatalf("in inside init: %#v", assign.Value)
	}
}

func TestParseASI(t *testing.T) {
	t.Parallel()
	// plain ASI at newline, }, and EOF
	for _, source := range []string{
		"a = 1\nb = 2",
		"if (a) { b() }",
		"function f() {\n  return\n}\n",
		"label:\nif (a) b()\nelse c()",
		"do {} while (a)\n",
	} {
		if err := Validate(source); err != nil {
			t.Errorf("%q: %v", source, err)
		}
	}
	// restricted production: return value cannot start on the next line
	file, err := Parse("function f() {\n  return\n  x;\n}")
	if err != nil {
		t.Fatal(err)
	}
	ret := file.Stmts[0].(*FunctionDecl).Body.Stmts[0].(*ReturnStmt)
	if ret.Expr != nil {
		t.Fatalf("return arg after newline: %#v", ret.Expr)
	}
	// postfix ++ attaches to the next statement after a newline
	file, err = Parse("a\n++b")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := file.Stmts[0].(*ExpressionStmt); !ok {
		t.Fatalf("first stmt: %T", file.Stmts[0])
	}
	pre := file.Stmts[1].(*ExpressionStmt).Expr.(*UpdateExpr)
	if !pre.Prefix {
		t.Fatalf("expected prefix after newline: %#v", pre)
	}
	// a semicolon is genuinely required between statements
	if err := Validate("const a = 1 const b = 2"); err == nil {
		t.Error("missing semicolon: expected error")
	}
	if err := Validate("throw\nnew Error()"); err == nil {
		t.Error("throw with newline: expected error")
	}
}

func TestParseSpans(t *testing.T) {
	t.Parallel()
	file, err := Parse("let x = 42;")
	if err != nil {
		t.Fatal(err)
	}
	if file.Span != (gast.Span{Start: 0, End: 11}) {
		t.Fatalf("file span: %v", file.Span)
	}
	lit := file.Stmts[0].(*VarStmt).Decls[0].Init.(*NumberLit)
	if lit.Span != (gast.Span{Start: 8, End: 10}) {
		t.Fatalf("literal span: %v", lit.Span)
	}
	call := exprOf(t, "foo(1, 2)")
	if call.Pos() != (gast.Span{Start: 0, End: 9}) {
		t.Fatalf("call span: %v", call.Pos())
	}
}

func TestParseErrorFormatting(t *testing.T) {
	t.Parallel()
	source := "let x = ;"
	err := Validate(source)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := errors.AsType[*gast.ParseError](err); !ok {
		t.Fatalf("got %T, want *gast.ParseError", err)
	}
	formatted := gast.FormatError(err, source)
	want := "ParseError at 1:9: unexpected ';'\n1 | let x = ;\n  |         ^"
	if formatted != want {
		t.Fatalf("caret output:\n%s\nwant:\n%s", formatted, want)
	}
	var lexErr *gast.LexError
	if err := Validate("var s = 'x"); !errors.As(err, &lexErr) {
		t.Errorf("got %T, want *gast.LexError", err)
	}
}
