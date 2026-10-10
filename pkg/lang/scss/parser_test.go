package scss

import (
	"os"
	"strings"
	"testing"

	"github.com/stubbedev/gast/internal/sasscore"
	"github.com/stubbedev/gast/pkg/gast"
)

func parseOne(t *testing.T, source string) sasscore.Node {
	t.Helper()
	file, err := Parse(source)
	if err != nil {
		t.Fatalf("%s: %v", source, err)
	}
	if len(file.Nodes) != 1 {
		t.Fatalf("%s: got %d top-level nodes", source, len(file.Nodes))
	}
	return file.Nodes[0]
}

func TestParseNestingAndParent(t *testing.T) {
	t.Parallel()
	file, err := Parse(".nav { ul { margin: 0; } a { &:hover { color: navy; } &.active { top: 0; } &__icon { left: 0; } } }")
	if err != nil {
		t.Fatal(err)
	}
	nav := file.Nodes[0].(*sasscore.QualifiedRule)
	link := nav.Block.Nodes[1].(*sasscore.QualifiedRule)
	if len(link.Block.Nodes) != 3 {
		t.Fatalf("got %d nested rules", len(link.Block.Nodes))
	}
	hover := link.Block.Nodes[0].(*sasscore.QualifiedRule)
	compound := hover.Selectors.Complexes[0].Compounds[0]
	if !compound.Parent || len(compound.Pseudos) != 1 || compound.Pseudos[0].Name != "hover" {
		t.Errorf("&:hover compound: %#v", compound)
	}
	active := link.Block.Nodes[1].(*sasscore.QualifiedRule).Selectors.Complexes[0].Compounds[0]
	if !active.Parent || len(active.Classes) != 1 || active.Classes[0] != "active" {
		t.Errorf("&.active compound: %#v", active)
	}
	icon := link.Block.Nodes[2].(*sasscore.QualifiedRule).Selectors.Complexes[0].Compounds[0]
	if !icon.Parent || len(icon.Interp) != 1 || icon.Interp[0].Text != "__icon" {
		t.Errorf("&__icon compound: %#v", icon)
	}
}

func TestParseVariableDecl(t *testing.T) {
	t.Parallel()
	file, err := Parse("$base: 4px !default; $wide: 8px !global; $plain: 1px;")
	if err != nil {
		t.Fatal(err)
	}
	first := file.Nodes[0].(*sasscore.VariableDecl)
	if first.Name != "base" || !first.Default || first.Global || len(first.Value) != 1 {
		t.Errorf("bad $base: %#v", first)
	}
	second := file.Nodes[1].(*sasscore.VariableDecl)
	if !second.Global || second.Default {
		t.Errorf("bad $wide: %#v", second)
	}
	if third := file.Nodes[2].(*sasscore.VariableDecl); third.Default || third.Global {
		t.Errorf("bad $plain: %#v", third)
	}
}

func TestParseInterpolation(t *testing.T) {
	t.Parallel()
	file, err := Parse("$name: panel; .#{$name} { margin-#{$side}: 1rem; #{$side}-x: 2rem; }")
	if err != nil {
		t.Fatal(err)
	}
	rule := file.Nodes[1].(*sasscore.QualifiedRule)
	compound := rule.Selectors.Complexes[0].Compounds[0]
	if len(compound.Classes) != 0 || len(compound.Interp) == 0 || compound.Interp[0].Kind != sasscore.KindInterpOpen {
		t.Errorf("selector interpolation: %#v", compound)
	}
	first := rule.Block.Nodes[0].(*sasscore.Declaration)
	if first.Name[0].Kind != sasscore.KindIdent || first.Name[len(first.Name)-1].Kind != sasscore.RBrace {
		t.Errorf("property name interpolation: %v", first.Name)
	}
	second := rule.Block.Nodes[1].(*sasscore.Declaration)
	if second.Name[0].Kind != sasscore.KindInterpOpen {
		t.Errorf("leading interpolation name: %v", second.Name)
	}
}

func TestParseIfElseChain(t *testing.T) {
	t.Parallel()
	node := parseOne(t, "@if $x { a { } } @else if $y { b { } } @else { c { } }")
	ifRule := node.(*sasscore.IfRule)
	if len(ifRule.Cond) != 1 || ifRule.Then == nil {
		t.Errorf("bad @if: %#v", ifRule)
	}
	if len(ifRule.ElseIfs) != 1 || len(ifRule.ElseIfs[0].Cond) != 1 {
		t.Errorf("bad @else if: %#v", ifRule.ElseIfs)
	}
	if ifRule.Else == nil {
		t.Error("missing @else block")
	}
	if ifRule.End != len("@if $x { a { } } @else if $y { b { } } @else { c { } }") {
		t.Errorf("span does not cover the chain: %v", ifRule.Span)
	}
	if err := Validate("@else { a { } }"); err == nil {
		t.Error("@else without @if: expected error")
	}
}

func TestParseEach(t *testing.T) {
	t.Parallel()
	node := parseOne(t, "@each $a, $b in (a b, c d) { .#{$a} { top: 0; } }")
	each := node.(*sasscore.EachRule)
	if len(each.Vars) != 2 || each.Vars[1] != "b" || each.Rest {
		t.Errorf("bad vars: %#v", each)
	}
	if len(each.Expr) == 0 || each.Block == nil {
		t.Errorf("bad expr: %#v", each)
	}
	node = parseOne(t, "@each $pair... in $list { a { } }")
	each = node.(*sasscore.EachRule)
	if !each.Rest || len(each.Vars) != 1 {
		t.Errorf("bad rest var: %#v", each)
	}
}

func TestParseFor(t *testing.T) {
	t.Parallel()
	node := parseOne(t, "@for $i from 1 through 3 { .c-#{$i} { top: 0; } }")
	forRule := node.(*sasscore.ForRule)
	if forRule.Var != "i" || !forRule.Inclusive || len(forRule.From) != 1 || len(forRule.To) != 1 {
		t.Errorf("bad @for: %#v", forRule)
	}
	node = parseOne(t, "@for $i from 0 to 2 { a { } }")
	if node.(*sasscore.ForRule).Inclusive {
		t.Error("'to' must not be inclusive")
	}
	if err := Validate("@for $i from 1 { a { } }"); err == nil {
		t.Error("@for without bounds: expected error")
	}
}

func TestParseMixinInclude(t *testing.T) {
	t.Parallel()
	file, err := Parse("@mixin box($w, $h: 2px, $rest...) { width: $w; } a { @include box(1px, $h: 3px); @include bare { color: red; } }")
	if err != nil {
		t.Fatal(err)
	}
	mixin := file.Nodes[0].(*sasscore.MixinRule)
	if mixin.Name != "box" || len(mixin.Params) != 3 {
		t.Fatalf("bad mixin: %#v", mixin)
	}
	if mixin.Params[0].Default != nil || len(mixin.Params[1].Default) != 1 || !mixin.Params[2].Rest {
		t.Errorf("bad params: %#v", mixin.Params)
	}
	rule := file.Nodes[1].(*sasscore.QualifiedRule)
	include := rule.Block.Nodes[0].(*sasscore.IncludeRule)
	if include.Name != "box" || len(include.Args) != 7 {
		t.Errorf("bad include: %#v", include)
	}
	bare := rule.Block.Nodes[1].(*sasscore.IncludeRule)
	if bare.Block == nil {
		t.Error("include content block missing")
	}
}

func TestParseFunctionReturn(t *testing.T) {
	t.Parallel()
	node := parseOne(t, "@function double($n) { @return $n * 2; }")
	fn := node.(*sasscore.FunctionRule)
	if fn.Name != "double" || len(fn.Params) != 1 || fn.Block == nil {
		t.Fatalf("bad function: %#v", fn)
	}
	ret, ok := fn.Block.Nodes[0].(*sasscore.ReturnRule)
	if !ok || len(ret.Value) != 3 {
		t.Errorf("bad @return: %#v", fn.Block.Nodes[0])
	}
}

func TestParseExtendPlaceholder(t *testing.T) {
	t.Parallel()
	file, err := Parse("%base { color: red; } .btn { @extend %base; }")
	if err != nil {
		t.Fatal(err)
	}
	placeholder := file.Nodes[0].(*sasscore.QualifiedRule)
	compound := placeholder.Selectors.Complexes[0].Compounds[0]
	if !compound.Placeholder || compound.Type != "base" {
		t.Errorf("placeholder rule: %#v", compound)
	}
	rule := file.Nodes[1].(*sasscore.QualifiedRule)
	extend, ok := rule.Block.Nodes[0].(*sasscore.ExtendRule)
	if !ok || len(extend.Selector) != 2 {
		t.Errorf("bad @extend: %#v", rule.Block.Nodes[0])
	}
}

func TestParseUseForward(t *testing.T) {
	t.Parallel()
	node := parseOne(t, `@use "helper" as h with ($radius: 8px, $gap: 1px);`)
	use := node.(*sasscore.UseRule)
	if use.Namespace != "h" || len(use.Config) != 2 || use.Config[1].Name != "gap" {
		t.Errorf("bad @use: %#v", use)
	}
	node = parseOne(t, `@forward "helper" as list-* show $a, $b hide $c;`)
	forward := node.(*sasscore.ForwardRule)
	if forward.Prefix != "list-*" || len(forward.Show) != 2 || len(forward.Hide) != 1 {
		t.Errorf("bad @forward: %#v", forward)
	}
}

func TestParseAtRootAndMediaNesting(t *testing.T) {
	t.Parallel()
	file, err := Parse("@media screen { .a { color: red; @at-root .b { color: blue; } padding: 0; } }")
	if err != nil {
		t.Fatal(err)
	}
	media := file.Nodes[0].(*sasscore.AtRule)
	rule := media.Block.Nodes[0].(*sasscore.QualifiedRule)
	if len(rule.Block.Nodes) != 3 {
		t.Fatalf("got %d block nodes", len(rule.Block.Nodes))
	}
	atRoot, ok := rule.Block.Nodes[1].(*sasscore.AtRootRule)
	if !ok || atRoot.Block == nil {
		t.Errorf("bad @at-root: %#v", rule.Block.Nodes[1])
	}
	// Declarations sit directly inside the @media block.
	if _, ok := media.Block.Nodes[0].(*sasscore.QualifiedRule); !ok {
		t.Errorf("first @media node: %T", media.Block.Nodes[0])
	}
}

func TestParseKeyframesWithInterpolation(t *testing.T) {
	t.Parallel()
	file, err := Parse("$n: 50; @keyframes slide { #{$n}% { left: 0; } to { left: 1px; } }")
	if err != nil {
		t.Fatal(err)
	}
	keyframes := file.Nodes[1].(*sasscore.AtRule)
	first := keyframes.Block.Nodes[0].(*sasscore.KeyframeRule)
	if len(first.Selectors) != 4 || first.Selectors[0].Kind != sasscore.KindInterpOpen {
		t.Errorf("interpolated keyframe selector: %v", first.Selectors)
	}
}

func TestParseSpans(t *testing.T) {
	t.Parallel()
	file, err := Parse("$size: 4px !default;")
	if err != nil {
		t.Fatal(err)
	}
	decl := file.Nodes[0].(*sasscore.VariableDecl)
	// Spans the whole declaration, flags included.
	if decl.Span != (gast.Span{Start: 0, End: 20}) {
		t.Errorf("variable span: %v", decl.Span)
	}
}

func TestValidateRules(t *testing.T) {
	t.Parallel()
	invalid := []string{
		"@else { }",
		"@mixin 12px { }",
		"@for $i from 1 { }",
		"$x: ;",
		"a { color: #{1; }",
		"@each $x (a b) { }",
		"@use helper;",
	}
	for _, source := range invalid {
		if err := Validate(source); err == nil {
			t.Errorf("%s: expected error", source)
		}
	}
}

func TestFormatErrorCaret(t *testing.T) {
	t.Parallel()
	err := Validate("a {\n  @else { }\n}")
	if err == nil {
		t.Fatal("expected error")
	}
	formatted := gast.FormatError(err, "a {\n  @else { }\n}")
	if !strings.Contains(formatted, "ParseError at 2:3:") || !strings.Contains(formatted, "^") {
		t.Errorf("bad format: %q", formatted)
	}
}

func BenchmarkParse(b *testing.B) {
	source, err := os.ReadFile("testdata/valid/large.scss")
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(source)))
	for b.Loop() {
		if _, err := Parse(string(source)); err != nil {
			b.Fatal(err)
		}
	}
}
