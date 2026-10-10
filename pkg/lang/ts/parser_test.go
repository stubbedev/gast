package ts

import (
	"errors"
	"strings"
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

	// `as` folds tighter than the ternary: (x as boolean) ? 1 : 2
	cond, _, _ := assertCond(t, exprOf(t, "x as boolean ? 1 : 2"))
	if assertion, ok := cond.(*TypeAssertionExpr); !ok {
		t.Fatalf("as before ternary: %T", cond)
	} else if ref, ok := assertion.Type.(*TypeRef); !ok || len(ref.Parts) != 1 || ref.Parts[0] != "boolean" {
		t.Fatalf("as type: %#v", assertion.Type)
	}
}

func assertCond(t *testing.T, expr Expr) (Expr, Expr, Expr) {
	t.Helper()
	cond, ok := expr.(*CondExpr)
	if !ok {
		t.Fatalf("expected *CondExpr, got %T", expr)
	}
	return cond.Cond, cond.Then, cond.Else
}

// TestParseAssertionShapes covers the TypeScript-only expression
// shapes: as/as const, satisfies, non-null, legacy angle assertions,
// and generic instantiation.
func TestParseAssertionShapes(t *testing.T) {
	t.Parallel()

	asConst := exprOf(t, `x = { a: 1 } as const`).(*AssignExpr).Value
	assertion, ok := asConst.(*TypeAssertionExpr)
	if !ok {
		t.Fatalf("as const: %T", asConst)
	}
	if !assertion.ConstForm || assertion.Type != nil {
		t.Fatalf("as const flags: %#v", assertion)
	}

	cast := exprOf(t, `value as unknown as string`)
	assertion, ok = cast.(*TypeAssertionExpr)
	if !ok {
		t.Fatalf("double as: %T", cast)
	}
	inner, ok := assertion.Expr.(*TypeAssertionExpr)
	if !ok || len(inner.Type.(*TypeRef).Parts) != 1 || inner.Type.(*TypeRef).Parts[0] != "unknown" {
		t.Fatalf("inner as: %#v", assertion.Expr)
	}

	satisfies := exprOf(t, `y = { a: 1 } satisfies Shape`).(*AssignExpr).Value
	sat, ok := satisfies.(*SatisfiesExpr)
	if !ok {
		t.Fatalf("satisfies: %T", satisfies)
	}
	if _, ok := sat.Expr.(*ObjectExpr); !ok {
		t.Fatalf("satisfies operand: %T", sat.Expr)
	}

	nonNull := exprOf(t, `maybe!.length!`)
	member, ok := nonNull.(*NonNullExpr)
	if !ok {
		t.Fatalf("non-null chain: %T", nonNull)
	}
	if inner, ok := member.Expr.(*MemberExpr); !ok {
		t.Fatalf("non-null operand: %T", member.Expr)
	} else if _, ok := inner.Obj.(*NonNullExpr); !ok {
		t.Fatalf("inner non-null: %T", inner.Obj)
	}

	angle := exprOf(t, `<string>input`)
	angleAssertion, ok := angle.(*AngleAssertionExpr)
	if !ok {
		t.Fatalf("angle assertion: %T", angle)
	}
	if _, ok := angleAssertion.Expr.(*IdentExpr); !ok {
		t.Fatalf("angle operand: %T", angleAssertion.Expr)
	}

	instantiation := exprOf(t, `identity<string>`)
	instantiated, ok := instantiation.(*InstantiationExpr)
	if !ok {
		t.Fatalf("instantiation: %T", instantiation)
	}
	if len(instantiated.TypeArgs) != 1 {
		t.Fatalf("instantiation type args: %d", len(instantiated.TypeArgs))
	}
}

// TestParseGenerics covers generic calls, nested closers that lex as
// shift operators, generic arrows, and type-argument shapes.
func TestParseGenerics(t *testing.T) {
	t.Parallel()

	call := exprOf(t, `first<string[]>([["a"]])`)
	node, ok := call.(*CallExpr)
	if !ok {
		t.Fatalf("generic call: %T", call)
	}
	if len(node.TypeArgs) != 1 || node.TypeArgs[0].(*ArrayType) == nil {
		t.Fatalf("call type args: %#v", node.TypeArgs)
	}
	array, ok := node.TypeArgs[0].(*ArrayType)
	if !ok {
		t.Fatalf("nested type arg: %T", node.TypeArgs[0])
	}
	if ref, ok := array.Elem.(*TypeRef); !ok || ref.Parts[0] != "string" {
		t.Fatalf("array element: %#v", array.Elem)
	}

	arrow := exprOf(t, `<T,>(value: T): T => value`)
	fn, ok := arrow.(*ArrowExpr)
	if !ok {
		t.Fatalf("generic arrow: %T", arrow)
	}
	if len(fn.TypeParams) != 1 || fn.TypeParams[0].Name != "T" {
		t.Fatalf("arrow type params: %#v", fn.TypeParams)
	}
	if len(fn.Params) != 1 || fn.Params[0].Type == nil {
		t.Fatalf("arrow params: %#v", fn.Params)
	}
	if _, ok := fn.ReturnType.(*TypeRef); !ok {
		t.Fatalf("arrow return type: %#v", fn.ReturnType)
	}

	asyncArrow := exprOf(t, `async <T>(value: T) => value`)
	fn, ok = asyncArrow.(*ArrowExpr)
	if !ok || !fn.Async || len(fn.TypeParams) != 1 {
		t.Fatalf("async generic arrow: %#v", asyncArrow)
	}

	// a < b stays a comparison, not a generic call
	left, _ := assertBinary(t, exprOf(t, "a < b"), Lt)
	if _, ok := left.(*IdentExpr); !ok {
		t.Fatalf("comparison left: %T", left)
	}
}

// TestParseTypeShapes covers representative type nodes.
func TestParseTypeShapes(t *testing.T) {
	t.Parallel()
	typeOf := func(source string) TypeNode {
		t.Helper()
		file, err := Parse("type Probe = " + source + ";")
		if err != nil {
			t.Fatalf("%s: %v", source, err)
		}
		alias := file.Stmts[0].(*TypeAliasDecl)
		return alias.Type
	}

	conditional, ok := typeOf("T extends Array<infer U> ? U : T").(*ConditionalType)
	if !ok {
		t.Fatalf("conditional: %T", conditional)
	}
	extends, ok := conditional.Extends.(*TypeRef)
	if !ok || len(extends.TypeArgs) != 1 {
		t.Fatalf("conditional extends: %#v", conditional.Extends)
	}
	if _, ok := extends.TypeArgs[0].(*InferType); !ok {
		t.Fatalf("conditional infer: %T", extends.TypeArgs[0])
	}
	if _, ok := conditional.True.(*TypeRef); !ok {
		t.Fatalf("conditional true: %T", conditional.True)
	}

	union, ok := typeOf("string | number | null").(*UnionType)
	if !ok || len(union.Types) != 3 {
		t.Fatalf("union: %#v", union)
	}

	mapped, ok := typeOf("{ +readonly [K in keyof T as `get${K & string}`]?: T[K] }").(*MappedType)
	if !ok {
		t.Fatalf("mapped: %T", mapped)
	}
	if mapped.ReadonlyMod != "+readonly" || mapped.OptionalMod != "?" {
		t.Fatalf("mapped modifiers: %q %q", mapped.ReadonlyMod, mapped.OptionalMod)
	}
	if _, ok := mapped.As.(*TemplateLiteralType); !ok {
		t.Fatalf("mapped as: %T", mapped.As)
	}

	fn, ok := typeOf("(x: number, ...rest: string[]) => void").(*FunctionType)
	if !ok || len(fn.Params) != 2 || !fn.Params[1].Rest {
		t.Fatalf("function type: %#v", fn)
	}

	ctor, ok := typeOf("abstract new (x: number) => object").(*FunctionType)
	if !ok || !ctor.New || !ctor.Abstract {
		t.Fatalf("constructor type: %#v", ctor)
	}

	tuple, ok := typeOf("[a: number, b?: string, ...c: boolean[]]").(*TupleType)
	if !ok || len(tuple.Elems) != 3 {
		t.Fatalf("tuple: %#v", tuple)
	}
	if tuple.Elems[0].Name != "a" || !tuple.Elems[1].Optional || !tuple.Elems[2].Rest {
		t.Fatalf("tuple elements: %#v", tuple.Elems)
	}

	object, ok := typeOf("{ readonly name?: string; (input: string): boolean; new (x: number): object; }").(*ObjectType)
	if !ok || len(object.Members) != 3 {
		t.Fatalf("object type: %#v", object)
	}
	if _, ok := object.Members[0].(*PropertySig); !ok {
		t.Fatalf("property sig: %T", object.Members[0])
	}
	if _, ok := object.Members[1].(*CallSig); !ok {
		t.Fatalf("call sig: %T", object.Members[1])
	}
	if _, ok := object.Members[2].(*ConstructSig); !ok {
		t.Fatalf("construct sig: %T", object.Members[2])
	}

	indexed, ok := typeOf("keyof T[K]").(*TypeOperator)
	if !ok || indexed.Op != "keyof" {
		t.Fatalf("keyof: %#v", indexed)
	}

	literal, ok := typeOf(`-1 | "a" | 2n | true`).(*UnionType)
	if !ok || len(literal.Types) != 4 {
		t.Fatalf("literal union: %#v", literal)
	}
	neg, ok := literal.Types[0].(*LiteralType)
	if !ok || !neg.Neg || neg.Text != "1" {
		t.Fatalf("negative literal: %#v", literal.Types[0])
	}

	imported, ok := typeOf(`import("./m").A.B<string>`).(*ImportType)
	if !ok || imported.Source != `"./m"` || len(imported.Parts) != 2 {
		t.Fatalf("import type: %#v", imported)
	}
}

// TestParseDeclarationShapes covers the TypeScript-only declarations.
func TestParseDeclarationShapes(t *testing.T) {
	t.Parallel()
	file, err := Parse("const enum E { A, B = A + 1, C = \"c\" }\ninterface I<T> extends A, B<T> { m(x: T): T }\ndeclare namespace N.M { export const x: number; }\nabstract class C<T> implements I<T> { abstract m(): void; constructor(private readonly x: T) {} }")
	if err != nil {
		t.Fatal(err)
	}
	enum, ok := file.Stmts[0].(*EnumDecl)
	if !ok || !enum.Const || len(enum.Members) != 3 {
		t.Fatalf("enum: %#v", file.Stmts[0])
	}
	iface, ok := file.Stmts[1].(*InterfaceDecl)
	if !ok || len(iface.TypeParams) != 1 || len(iface.Extends) != 2 {
		t.Fatalf("interface: %#v", file.Stmts[1])
	}
	namespace, ok := file.Stmts[2].(*NamespaceDecl)
	if !ok || !namespace.Declare || namespace.Name != "N.M" {
		t.Fatalf("namespace: %#v", file.Stmts[2])
	}
	class, ok := file.Stmts[3].(*ClassDecl)
	if !ok || !class.Abstract || len(class.Implements) != 1 {
		t.Fatalf("class: %#v", file.Stmts[3])
	}
	ctor := class.Body[len(class.Body)-1].(*MethodDef)
	if ctor.Kind != "constructor" {
		t.Fatalf("constructor: %#v", ctor)
	}
	param := ctor.Params[0]
	if param.Accessibility != "private" || !param.Readonly || param.Type == nil {
		t.Fatalf("parameter property: %#v", param)
	}
}

// TestParseSpans asserts half-open spans of representative nodes.
func TestParseSpans(t *testing.T) {
	t.Parallel()
	file, err := Parse("let x: number = 1;\n")
	if err != nil {
		t.Fatal(err)
	}
	decl := file.Stmts[0].(*VarStmt)
	if decl.Span != (gast.Span{Start: 0, End: 18}) {
		t.Fatalf("var span: %v", decl.Span)
	}
	binding := decl.Decls[0].Binding.Pos()
	if binding != (gast.Span{Start: 4, End: 5}) {
		t.Fatalf("binding span: %v", binding)
	}

	file, err = Parse("function f<T>(x: T): T { return x; }")
	if err != nil {
		t.Fatal(err)
	}
	fn := file.Stmts[0].(*FunctionDecl)
	if fn.Span != (gast.Span{Start: 0, End: 36}) {
		t.Fatalf("function span: %v", fn.Span)
	}
	if fn.TypeParams[0].Span != (gast.Span{Start: 11, End: 12}) {
		t.Fatalf("type param span: %v", fn.TypeParams[0].Span)
	}

	file, err = Parse("f<number>(x);")
	if err != nil {
		t.Fatal(err)
	}
	call := file.Stmts[0].(*ExpressionStmt).Expr.(*CallExpr)
	if call.Span != (gast.Span{Start: 0, End: 12}) {
		t.Fatalf("call span: %v", call.Span)
	}
}

// TestParseErrorSpans asserts ParseError positions are spanned.
func TestParseErrorSpans(t *testing.T) {
	t.Parallel()
	var spanned *gast.ParseError
	err := Validate("let broken: => = 1;")
	if !errors.As(err, &spanned) {
		t.Fatalf("expected *gast.ParseError, got %T", err)
	}
	if spanned.Pos.Start != 12 {
		t.Fatalf("error span: %v", spanned.Pos)
	}
}

// TestFormatErrorCaret asserts the rendered diagnostic.
func TestFormatErrorCaret(t *testing.T) {
	t.Parallel()
	source := "const x: Array<string> = 1;"
	err := Validate(source + "\nconst x Array<string> = 2;")
	if err == nil {
		t.Fatal("expected an error")
	}
	formatted := gast.FormatError(err, source+"\nconst x Array<string> = 2;")
	wantLines := []string{
		"ParseError at 2:9: expected ';' after statement, found 'Array'",
		"2 | const x Array<string> = 2;",
		"  | " + strings.Repeat(" ", 8) + "^",
	}
	want := strings.Join(wantLines, "\n")
	if formatted != want {
		t.Fatalf("caret output:\n%s\nwant:\n%s", formatted, want)
	}
}
