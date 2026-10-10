package css

import (
	"os"
	"strings"
	"testing"

	"github.com/stubbedev/gast/internal/sasscore"
	"github.com/stubbedev/gast/pkg/gast"
)

// ruleOf parses source and returns its single top-level qualified rule.
func ruleOf(t *testing.T, source string) *sasscore.QualifiedRule {
	t.Helper()
	file, err := Parse(source)
	if err != nil {
		t.Fatalf("%s: %v", source, err)
	}
	if len(file.Nodes) != 1 {
		t.Fatalf("%s: got %d top-level nodes", source, len(file.Nodes))
	}
	rule, ok := file.Nodes[0].(*sasscore.QualifiedRule)
	if !ok {
		t.Fatalf("%s: got %T", source, file.Nodes[0])
	}
	return rule
}

func compoundAt(t *testing.T, rule *sasscore.QualifiedRule, complex, compound int) *sasscore.CompoundSelector {
	t.Helper()
	return rule.Selectors.Complexes[complex].Compounds[compound]
}

func TestParseSelectorShapes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		source   string
		complex  int
		compound int
		check    func(*sasscore.CompoundSelector) error
	}{
		{"* {}", 0, 0, func(c *sasscore.CompoundSelector) error {
			if !c.Universal {
				t.Error("universal not set")
			}
			return nil
		}},
		{"a:hover {}", 0, 0, func(c *sasscore.CompoundSelector) error {
			if c.Type != "a" || len(c.Pseudos) != 1 || c.Pseudos[0].Name != "hover" {
				t.Errorf("bad compound: %#v", c)
			}
			return nil
		}},
		{"#id.class1.class2 {}", 0, 0, func(c *sasscore.CompoundSelector) error {
			if c.ID != "id" || len(c.Classes) != 2 || c.Classes[1] != "class2" {
				t.Errorf("bad compound: %#v", c)
			}
			return nil
		}},
		{"input[type=\"text\" i] {}", 0, 0, func(c *sasscore.CompoundSelector) error {
			if len(c.Attributes) != 1 {
				t.Fatalf("no attribute selector: %#v", c)
			}
			attr := c.Attributes[0]
			if attr.Name != "type" || attr.Op != "=" || attr.Value != "text" || !attr.Quoted || attr.Flag != "i" {
				t.Errorf("bad attribute: %#v", attr)
			}
			return nil
		}},
		{"a[href^=\"https\"] {}", 0, 0, func(c *sasscore.CompoundSelector) error {
			if attr := c.Attributes[0]; attr.Op != "^=" || !attr.Quoted || attr.Value != "https" {
				t.Errorf("bad attribute: %#v", attr)
			}
			return nil
		}},
		{"a > b + c ~ d {}", 0, 0, func(_ *sasscore.CompoundSelector) error { return nil }},
		{":is(ol, ul.menu) > li:not(:first-child) {}", 0, 0, func(c *sasscore.CompoundSelector) error {
			is := c.Pseudos[0]
			if is.Name != "is" || is.Selector == nil || len(is.Selector.Complexes) != 2 {
				t.Errorf("bad :is selector: %#v", is)
			}
			if len(is.Selector.Complexes[1].Compounds) != 1 ||
				len(is.Selector.Complexes[1].Compounds[0].Classes) != 1 {
				t.Errorf(":is second complex: %#v", is.Selector.Complexes[1])
			}
			return nil
		}},
		{"p::first-line {}", 0, 0, func(c *sasscore.CompoundSelector) error {
			if !c.Pseudos[0].Element {
				t.Errorf("pseudo-element flag: %#v", c.Pseudos[0])
			}
			return nil
		}},
	}
	for _, test := range tests {
		rule := ruleOf(t, test.source)
		if err := test.check(compoundAt(t, rule, test.complex, test.compound)); err != nil {
			t.Errorf("%s: %v", test.source, err)
		}
	}
}

func TestParseCombinators(t *testing.T) {
	t.Parallel()
	rule := ruleOf(t, "a > b c + d {}")
	complex := rule.Selectors.Complexes[0]
	if len(complex.Compounds) != 4 {
		t.Fatalf("got %d compounds", len(complex.Compounds))
	}
	want := []string{">", " ", "+"}
	for i, combinator := range want {
		if complex.Combinators[i] != combinator {
			t.Errorf("combinator %d: got %q, want %q", i, complex.Combinators[i], combinator)
		}
	}
}

func TestParseDeclarations(t *testing.T) {
	t.Parallel()
	file, err := Parse("a { color: red !important; margin: 0 }")
	if err != nil {
		t.Fatal(err)
	}
	rule := file.Nodes[0].(*sasscore.QualifiedRule)
	if len(rule.Block.Nodes) != 2 {
		t.Fatalf("got %d declarations", len(rule.Block.Nodes))
	}
	first := rule.Block.Nodes[0].(*sasscore.Declaration)
	if first.Name[0].Text != "color" || !first.Important || len(first.Value) != 1 {
		t.Errorf("bad declaration: %#v", first)
	}
	second := rule.Block.Nodes[1].(*sasscore.Declaration)
	if second.Important {
		t.Error("second declaration should not be important")
	}
}

func TestParseCustomPropertyRawValue(t *testing.T) {
	t.Parallel()
	file, err := Parse(":root { --x: { raw: tokens; keep }; }")
	if err != nil {
		t.Fatal(err)
	}
	rule := file.Nodes[0].(*sasscore.QualifiedRule)
	decl := rule.Block.Nodes[0].(*sasscore.Declaration)
	if !decl.Custom || decl.Name[0].Text != "--x" {
		t.Fatalf("bad declaration: %#v", decl)
	}
	// The value keeps every raw token up to the final semicolon.
	if len(decl.Value) != 7 {
		t.Errorf("raw value: %v", decl.Value)
	}
}

func TestParseAtRules(t *testing.T) {
	t.Parallel()
	file, err := Parse("@charset \"UTF-8\"; @media screen { a { color: red; } } @import url(x.css);")
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Nodes) != 3 {
		t.Fatalf("got %d nodes", len(file.Nodes))
	}
	charset := file.Nodes[0].(*sasscore.AtRule)
	if charset.Name != "charset" || len(charset.Prelude) != 1 || charset.Prelude[0].Kind != sasscore.KindString {
		t.Errorf("bad @charset: %#v", charset)
	}
	media := file.Nodes[1].(*sasscore.AtRule)
	if media.Name != "media" || media.Block == nil || len(media.Block.Nodes) != 1 {
		t.Errorf("bad @media: %#v", media)
	}
	imported := file.Nodes[2].(*sasscore.AtRule)
	if imported.Name != "import" || imported.Block != nil {
		t.Errorf("bad @import: %#v", imported)
	}
}

func TestParseKeyframes(t *testing.T) {
	t.Parallel()
	file, err := Parse("@keyframes slide { from, 50% { left: 0; } to { left: 10px; } }")
	if err != nil {
		t.Fatal(err)
	}
	keyframes := file.Nodes[0].(*sasscore.AtRule)
	if len(keyframes.Block.Nodes) != 2 {
		t.Fatalf("got %d keyframe rules", len(keyframes.Block.Nodes))
	}
	first := keyframes.Block.Nodes[0].(*sasscore.KeyframeRule)
	if len(first.Selectors) != 3 || first.Selectors[2].Kind != sasscore.KindPercentage {
		t.Errorf("bad keyframe selectors: %v", first.Selectors)
	}
}

func TestParseSpans(t *testing.T) {
	t.Parallel()
	source := "a .b { color: red; }"
	file, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	rule := file.Nodes[0].(*sasscore.QualifiedRule)
	// The selector list spans bytes 0..13 ("a .b" ends at 4; braces included in block).
	if rule.Selectors.Span != (gast.Span{Start: 0, End: 4}) {
		t.Errorf("selector span: %v", rule.Selectors.Span)
	}
	decl := rule.Block.Nodes[0].(*sasscore.Declaration)
	if decl.Span != (gast.Span{Start: 7, End: 17}) {
		t.Errorf("declaration span: %v", decl.Span)
	}
}

func TestValidateRules(t *testing.T) {
	t.Parallel()
	valid := []string{
		"a { color: red; }",
		"<!-- a { } -->",
		"@media print { a { } }",
		"a { color: red }",
		"/* only a comment */",
	}
	for _, source := range valid {
		if err := Validate(source); err != nil {
			t.Errorf("%s: unexpected error %v", source, err)
		}
	}
	invalid := map[string]string{
		"a { color: red;":                       "unclosed block",
		"a { } }":                               "stray }",
		"@charset \"x\"; a { } @charset \"y\";": "@charset must be first",
		"{ }":                                   "empty selector",
		"a { color red; }":                      "missing colon",
		"a { color: ; }":                        "empty value",
		"a { b(1)); }":                          "stray )",
		"@media }":                              "unexpected }",
	}
	for source, why := range invalid {
		if err := Validate(source); err == nil {
			t.Errorf("%s: expected error (%s)", source, why)
		}
	}
}

func TestFormatErrorCaret(t *testing.T) {
	t.Parallel()
	err := Validate("a {\n  color red;\n}")
	if err == nil {
		t.Fatal("expected error")
	}
	formatted := gast.FormatError(err, "a {\n  color red;\n}")
	lines := strings.Split(formatted, "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "ParseError at 2:3:") {
		t.Fatalf("bad header: %q", formatted)
	}
	if lines[1] != "2 |   color red;" {
		t.Errorf("bad source line: %q", lines[1])
	}
	if lines[2] != "  |   ^" {
		t.Errorf("bad caret line: %q", lines[2])
	}
}

func BenchmarkParse(b *testing.B) {
	source, err := os.ReadFile("testdata/valid/large.css")
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
