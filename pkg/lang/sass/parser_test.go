package sass

import (
	"os"
	"strings"
	"testing"

	"github.com/stubbedev/gast/internal/sasscore"
	"github.com/stubbedev/gast/pkg/gast"
)

func TestParseIndentedRules(t *testing.T) {
	t.Parallel()
	file, err := Parse(".nav\n  ul\n    margin: 0\n  li\n    display: inline-block\n")
	if err != nil {
		t.Fatal(err)
	}
	nav := file.Nodes[0].(*sasscore.QualifiedRule)
	if len(nav.Block.Nodes) != 2 {
		t.Fatalf("got %d children", len(nav.Block.Nodes))
	}
	list := nav.Block.Nodes[0].(*sasscore.QualifiedRule)
	if list.Selectors.Complexes[0].Compounds[0].Type != "ul" {
		t.Errorf("ul selector: %#v", list.Selectors)
	}
	margin := list.Block.Nodes[0].(*sasscore.Declaration)
	if margin.Name[0].Text != "margin" || len(margin.Value) != 1 {
		t.Errorf("margin declaration: %#v", margin)
	}
}

func TestParseParentAndNested(t *testing.T) {
	t.Parallel()
	file, err := Parse("a\n  color: blue\n  &:hover\n    color: navy\n")
	if err != nil {
		t.Fatal(err)
	}
	rule := file.Nodes[0].(*sasscore.QualifiedRule)
	hover := rule.Block.Nodes[1].(*sasscore.QualifiedRule)
	compound := hover.Selectors.Complexes[0].Compounds[0]
	if !compound.Parent || len(compound.Pseudos) != 1 {
		t.Errorf("&:hover: %#v", compound)
	}
}

func TestParseAtRulesByIndentation(t *testing.T) {
	t.Parallel()
	file, err := Parse("@if $x == 1\n  a\n    color: red\n@else\n  a\n    color: blue\n@for $i from 1 through 2\n  .c-#{$i}\n    top: 0\n@each $k, $v in (a: 1)\n  .#{$k}\n    left: $v\n")
	if err != nil {
		t.Fatal(err)
	}
	ifRule := file.Nodes[0].(*sasscore.IfRule)
	if len(ifRule.Cond) != 4 || ifRule.Else == nil {
		t.Errorf("bad @if/@else: %#v", ifRule)
	}
	forRule := file.Nodes[1].(*sasscore.ForRule)
	if forRule.Var != "i" || !forRule.Inclusive {
		t.Errorf("bad @for: %#v", forRule)
	}
	each := file.Nodes[2].(*sasscore.EachRule)
	if len(each.Vars) != 2 || each.Vars[0] != "k" {
		t.Errorf("bad @each: %#v", each)
	}
}

func TestParseMixinIncludeByIndentation(t *testing.T) {
	t.Parallel()
	file, err := Parse("@mixin pad($n)\n  padding: $n\n\n.card\n  @include pad(4px)\n\n.panel\n  @include responsive\n    color: red\n")
	if err != nil {
		t.Fatal(err)
	}
	mixin := file.Nodes[0].(*sasscore.MixinRule)
	if mixin.Name != "pad" || len(mixin.Params) != 1 {
		t.Errorf("bad mixin: %#v", mixin)
	}
	card := file.Nodes[1].(*sasscore.QualifiedRule)
	include := card.Block.Nodes[0].(*sasscore.IncludeRule)
	if include.Name != "pad" || include.Block != nil {
		t.Errorf("bad include: %#v", include)
	}
	panel := file.Nodes[2].(*sasscore.QualifiedRule)
	content := panel.Block.Nodes[0].(*sasscore.IncludeRule)
	if content.Block == nil {
		t.Errorf("include content block: %#v", content)
	}
}

func TestParseMediaAndKeyframesByIndentation(t *testing.T) {
	t.Parallel()
	file, err := Parse("@media screen\n  .sidebar\n    width: 30%\n\n@keyframes slide\n  from\n    left: 0\n\n  to\n    left: 100%\n")
	if err != nil {
		t.Fatal(err)
	}
	media := file.Nodes[0].(*sasscore.AtRule)
	rule := media.Block.Nodes[0].(*sasscore.QualifiedRule)
	if rule.Selectors.Complexes[0].Compounds[0].Classes[0] != "sidebar" {
		t.Errorf("media child: %#v", rule)
	}
	keyframes := file.Nodes[1].(*sasscore.AtRule)
	first := keyframes.Block.Nodes[0].(*sasscore.KeyframeRule)
	if first.Selectors[0].Text != "from" {
		t.Errorf("keyframe selector: %v", first.Selectors)
	}
}

func TestParseSpans(t *testing.T) {
	t.Parallel()
	file, err := Parse("a\n  color: red\n")
	if err != nil {
		t.Fatal(err)
	}
	rule := file.Nodes[0].(*sasscore.QualifiedRule)
	// The block spans from the indent token to the trailing newline.
	if rule.Block.Span != (gast.Span{Start: 2, End: 15}) {
		t.Errorf("block span: %v", rule.Block.Span)
	}
	if file.Span != (gast.Span{Start: 0, End: 15}) {
		t.Errorf("file span: %v", file.Span)
	}
}

func TestValidateErrors(t *testing.T) {
	t.Parallel()
	invalid := map[string]string{
		"  color: red\n":             "unexpected indent at top level",
		"a\n  color: red\n}":         "stray }",
		"$x:\n":                      "empty variable value",
		"a\n  color: red\n  b(1))\n": "stray )",
	}
	for source, why := range invalid {
		if err := Validate(source); err == nil {
			t.Errorf("%q: expected error (%s)", source, why)
		}
	}
}

func TestFormatErrorCaret(t *testing.T) {
	t.Parallel()
	source := "a\n  color: red\n\tmargin: 0\n"
	err := Validate(source)
	if err == nil {
		t.Fatal("expected error")
	}
	formatted := gast.FormatError(err, source)
	if !strings.Contains(formatted, "LexError at 3:1:") || !strings.Contains(formatted, "^") {
		t.Errorf("bad format: %q", formatted)
	}
}

func BenchmarkParse(b *testing.B) {
	source, err := os.ReadFile("testdata/valid/large.sass")
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
