package ts

import (
	"errors"
	"strings"
	"testing"

	"github.com/stubbedev/gast/pkg/gast"
)

// jsxExprOf parses `<source>;` in TSX mode and returns the single
// expression.
func jsxExprOf(t *testing.T, source string) Expr {
	t.Helper()
	file, err := ParseTSX(source + ";")
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

// TestParseJSXShapes covers element, fragment, attribute, and child
// shapes against the AST.
func TestParseJSXShapes(t *testing.T) {
	t.Parallel()

	el := jsxExprOf(t, `<div id="a" bool spread-me={1 + 2} {...rest}>x{y}<span/></div>`).(*JSXElement)
	if el.Fragment || el.SelfClosing {
		t.Fatalf("element flags: %#v", el)
	}
	if el.Name == nil || len(el.Name.Parts) != 1 || el.Name.Parts[0] != "div" {
		t.Fatalf("element name: %#v", el.Name)
	}
	if len(el.Attrs) != 4 {
		t.Fatalf("attrs: %#v", el.Attrs)
	}
	if attr := el.Attrs[0]; attr.Value == nil || attr.Spread != nil {
		t.Fatalf("string attr: %#v", attr)
	}
	if attr := el.Attrs[1]; attr.Value != nil || attr.Spread != nil {
		t.Fatalf("boolean attr: %#v", attr)
	}
	if attr := el.Attrs[2]; attr.Name.Parts[0] != "spread-me" {
		t.Fatalf("dashed attr name: %#v", attr.Name)
	}
	if attr := el.Attrs[3]; attr.Spread == nil || attr.Name.Parts != nil {
		t.Fatalf("spread attr: %#v", attr)
	}
	if len(el.Children) != 3 {
		t.Fatalf("children: %#v", el.Children)
	}
	if text := el.Children[0].(*JSXText); text.Text != "x" {
		t.Fatalf("text child: %q", text.Text)
	}
	if container := el.Children[1].(*JSXExprContainer); container.Expr == nil {
		t.Fatalf("expression child: %#v", container)
	}
	if nested := el.Children[2].(*JSXElement); !nested.SelfClosing {
		t.Fatalf("nested element: %#v", nested)
	}

	frag := jsxExprOf(t, `<>{/* comment */}{}</>`).(*JSXElement)
	if !frag.Fragment || frag.Name != nil || len(frag.Children) != 2 {
		t.Fatalf("fragment: %#v", frag)
	}
	if container := frag.Children[0].(*JSXExprContainer); container.Expr != nil {
		t.Fatalf("comment container: %#v", container)
	}

	ns := jsxExprOf(t, `<svg:use xlink:href="#a"/>`).(*JSXElement)
	if !ns.Name.Namespace || ns.Name.Parts[0] != "svg" || ns.Name.Parts[1] != "use" {
		t.Fatalf("namespaced name: %#v", ns.Name)
	}
	if attr := ns.Attrs[0]; !attr.Name.Namespace || attr.Name.Parts[1] != "href" {
		t.Fatalf("namespaced attr: %#v", attr.Name)
	}

	member := jsxExprOf(t, `<A.B.C m="v">body</A.B.C>`).(*JSXElement)
	if member.Name.Namespace || len(member.Name.Parts) != 3 {
		t.Fatalf("member name: %#v", member.Name)
	}
}

// TestParseJSXTextRawness pins that JSX text and attribute strings keep
// their raw source: entities are never decoded, and characters the
// JavaScript lexer would treat as literals ('#', quotes, slashes) are
// plain text.
func TestParseJSXTextRawness(t *testing.T) {
	t.Parallel()

	el := jsxExprOf(t, "<p>&amp; &#65; &notanentity; don't \"q\" `t` #h a / b</p>").(*JSXElement)
	text, ok := el.Children[0].(*JSXText)
	if !ok || len(el.Children) != 1 {
		t.Fatalf("children: %#v", el.Children)
	}
	if text.Text != `&amp; &#65; &notanentity; don't "q" `+"`t`"+` #h a / b` {
		t.Fatalf("raw text: %q", text.Text)
	}
	if el.Attrs != nil {
		t.Fatalf("attrs: %#v", el.Attrs)
	}
}

// TestParseTSXArrows covers the arrow-versus-JSX disambiguation: a
// generic arrow needs a constraint, default, or trailing comma.
func TestParseTSXArrows(t *testing.T) {
	t.Parallel()

	arrow := jsxExprOf(t, `<T,>(v: T): T => v`).(*ArrowExpr)
	if len(arrow.TypeParams) != 1 || arrow.TypeParams[0].Name != "T" {
		t.Fatalf("trailing-comma arrow: %#v", arrow.TypeParams)
	}

	arrow = jsxExprOf(t, `<T extends object, U = string>(v: T): U => v`).(*ArrowExpr)
	if len(arrow.TypeParams) != 2 || arrow.TypeParams[1].Default == nil {
		t.Fatalf("constraint+default arrow: %#v", arrow.TypeParams)
	}

	arrow = jsxExprOf(t, `async <T extends string>(v: T) => v`).(*ArrowExpr)
	if !arrow.Async || len(arrow.TypeParams) != 1 {
		t.Fatalf("async disambiguated arrow: %#v", arrow)
	}

	// The plain form is a JSX element in TSX mode and fails for the
	// missing closing tag, like tsc.
	err := ValidateTSX(`const f = <T>(v: T) => v;`)
	if err == nil {
		t.Fatal("expected plain generic arrow to be rejected in TSX mode")
	}
	var spanned *gast.ParseError
	if !errors.As(err, &spanned) || spanned.Pos.Start != 10 {
		t.Fatalf("error at the '<': %v", spanned)
	}
}

// TestParseTSXErrors asserts the diagnostic shape of representative
// TSX errors: unclosed elements and mismatches fail at a '<'.
func TestParseTSXErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		source string
		at     int
		want   string
	}{
		{source: "const el = <div>;", at: 11, want: "JSX element 'div' has no corresponding closing tag"},
		{source: "declare const x: unknown;\nconst y = <string>x;", at: 36, want: "JSX element 'string' has no corresponding closing tag"},
		{source: "const el = <div></span>;", at: 17, want: "expected corresponding JSX closing tag for 'div', found 'span'"},
		{source: "const el = <div>a } b</div>;", at: 18, want: "unexpected '}' in JSX text"},
	}
	for _, tc := range cases {
		err := ValidateTSX(tc.source)
		var spanned *gast.ParseError
		if !errors.As(err, &spanned) {
			t.Fatalf("%s: expected *gast.ParseError, got %v", tc.source, err)
		}
		if spanned.Pos.Start != tc.at {
			t.Errorf("%s: error at %d, want %d (%v)", tc.source, spanned.Pos.Start, tc.at, err)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q does not contain %q", tc.source, err, tc.want)
		}
	}
}

// TestParseAngleBracketSpeculation covers the .ts-mode '<': the verdict
// comes from bounded speculation (generic arrow, then legacy assertion,
// then comparison), mirrored against tsc without --jsx.
func TestParseAngleBracketSpeculation(t *testing.T) {
	t.Parallel()

	// Comparisons stay comparisons; tsc agrees that `a < b > (c)` is
	// the generic call a<b>(c).
	for _, source := range []string{"a < b", "a < b > c", "a < b >> c", "a <b> c"} {
		expr := exprOf(t, source)
		if _, ok := expr.(*BinaryExpr); !ok {
			t.Errorf("%s: got %T, want comparison", source, expr)
		}
	}
	call, ok := exprOf(t, "a < b > (c)").(*CallExpr)
	if !ok || len(call.TypeArgs) != 1 || len(call.Args) != 1 {
		t.Fatalf("generic-call reading of 'a < b > (c)': %#v", exprOf(t, "a < b > (c)"))
	}

	// Assertions parse as assertions.
	assertion, ok := exprOf(t, "<string>input").(*AngleAssertionExpr)
	if !ok {
		t.Fatalf("angle assertion: %T", exprOf(t, "<string>input"))
	}
	if _, ok := assertion.Expr.(*IdentExpr); !ok {
		t.Fatalf("assertion operand: %#v", assertion.Expr)
	}
	exprOf(t, "<number | null>len! + 1") // folds into surrounding operators

	// Generic arrows win over assertions; the plain and disambiguated
	// forms both parse in .ts mode.
	arrow, ok := exprOf(t, "<T>(v: T) => v").(*ArrowExpr)
	if !ok || len(arrow.TypeParams) != 1 {
		t.Fatalf("plain generic arrow: %#v", exprOf(t, "<T>(v: T) => v"))
	}
	if arrow, ok = exprOf(t, "<T extends U>(x) => x").(*ArrowExpr); !ok || arrow.TypeParams[0].Constraint == nil {
		t.Fatalf("constrained generic arrow: %#v", exprOf(t, "<T extends U>(x) => x"))
	}
	if arrow, ok = exprOf(t, "async <T>(x: T) => x").(*ArrowExpr); !ok || !arrow.Async {
		t.Fatalf("async generic arrow: %#v", exprOf(t, "async <T>(x: T) => x"))
	}

	// What is neither - a '<' with no operand to its left and no
	// assertion or arrow shape - fails with the assertion branch's
	// error rather than looping.
	if err := Validate("< << x;"); err == nil {
		t.Error("expected an error for '< << x'")
	}
}

// TestTSXLexerModes drives the lazy lexer through the mode switches
// the parser performs, checking tokens only the mode stack can
// produce: single-character '>' and '/' in tags and raw JSX text runs.
func TestTSXLexerModes(t *testing.T) {
	t.Parallel()

	source := `<div {...r} s="1">a'b / <b></b>{x}</div>`
	l := newTSXLexer(source)
	expect := func(mode lexMode, want, text string) {
		t.Helper()
		l.mode = mode
		tok, err := l.next()
		if err != nil {
			t.Fatalf("mode %d: %v", mode, err)
		}
		if tok.Kind != want || tok.Text != text {
			t.Fatalf("mode %d: got %q %q, want %q %q", mode, tok.Kind, tok.Text, want, text)
		}
	}

	expect(lexTag, Lt, "<")
	expect(lexTag, KindIdent, "div")
	expect(lexTag, LBrace, "{")
	expect(lexJS, Ellipsis, "...")
	expect(lexJS, KindIdent, "r")
	expect(lexJS, RBrace, "}")
	expect(lexTag, KindIdent, "s")
	expect(lexTag, Assign, "=")
	expect(lexTag, KindString, `"1"`)
	expect(lexTag, Gt, ">")
	expect(lexText, KindJSXText, "a'b / ")
	expect(lexText, Lt, "<")
	expect(lexTag, KindIdent, "b")
	expect(lexTag, Gt, ">")
	expect(lexText, Lt, "<") // empty text run falls through to '<'
	expect(lexTag, Slash, "/")
	expect(lexTag, KindIdent, "b")
	expect(lexTag, Gt, ">")
	expect(lexText, LBrace, "{")
	expect(lexJS, KindIdent, "x")
	expect(lexJS, RBrace, "}")
	expect(lexText, Lt, "<")
	expect(lexTag, Slash, "/")
	expect(lexTag, KindIdent, "div")
	expect(lexTag, Gt, ">")
	l.mode = lexJS // after the element, ordinary code resumes
	tok, err := l.next()
	if err != nil || tok.Kind != KindEOF {
		t.Fatalf("after element: %q %v", tok.Kind, err)
	}
}
