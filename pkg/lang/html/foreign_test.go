package html

import (
	"strings"
	"testing"

	"github.com/stubbedev/gast/pkg/gast"
)

func TestParseForeignSVG(t *testing.T) {
	t.Parallel()
	doc, err := Parse(`<svg viewBox="0 0 10 10" preserveAspectRatio="xMidYMid"><clipPath id="c"><rect width="1" height="1"/></clipPath></svg>`)
	if err != nil {
		t.Fatal(err)
	}
	svg := elementChild(t, bodyOf(t, doc), 0)
	if svg.Name != "svg" {
		t.Fatalf("svg name: %q", svg.Name)
	}
	if got := svg.Attrs[0].Name; got != "viewBox" {
		t.Fatalf("attr 0: %q, want viewBox (source case kept)", got)
	}
	if got := svg.Attrs[1].Name; got != "preserveAspectRatio" {
		t.Fatalf("attr 1: %q, want preserveAspectRatio", got)
	}
	clip := elementChild(t, svg, 0)
	if clip.Name != "clipPath" {
		t.Fatalf("child: %q, want clipPath", clip.Name)
	}
	rect := elementChild(t, clip, 0)
	if len(rect.Children) != 0 {
		t.Fatalf("self-closed rect must stay childless, got %d children", len(rect.Children))
	}
}

func TestParseForeignSVGTagAdjustment(t *testing.T) {
	t.Parallel()
	// Lowercase source spellings are case-adjusted per spec 13.2.6.1.
	doc, err := Parse(`<svg><foreignobject></foreignobject></svg>`)
	if err != nil {
		t.Fatal(err)
	}
	svg := elementChild(t, bodyOf(t, doc), 0)
	if got := elementChild(t, svg, 0).Name; got != "foreignObject" {
		t.Fatalf("adjusted name: %q, want foreignObject", got)
	}
}

func TestParseForeignSVGAttrAdjustment(t *testing.T) {
	t.Parallel()
	doc, err := Parse(`<svg viewBox="0 0 1 1"><g viewbox="2 2 2 2"></g><lineargradient gradienttransform="x"></lineargradient></svg>`)
	if err != nil {
		t.Fatal(err)
	}
	svg := elementChild(t, bodyOf(t, doc), 0)
	// The svg root's own attributes are inserted by HTML rules and keep
	// their source spelling; children get the case adjustment.
	if got := svg.Attrs[0].Name; got != "viewBox" {
		t.Fatalf("svg root attr: %q, want source-case viewBox", got)
	}
	g := elementChild(t, svg, 0)
	if got := g.Attrs[0].Name; got != "viewBox" {
		t.Fatalf("adjusted attr: %q, want viewBox", got)
	}
	gradient := elementChild(t, svg, 1)
	if gradient.Name != "linearGradient" {
		t.Fatalf("adjusted name: %q, want linearGradient", gradient.Name)
	}
	if got := gradient.Attrs[0].Name; got != "gradientTransform" {
		t.Fatalf("adjusted attr: %q, want gradientTransform", got)
	}
}

func TestParseForeignSelfClosing(t *testing.T) {
	t.Parallel()
	// Self-closing works for any element inside foreign content.
	doc, err := Parse(`<svg><g/><g/><use href="#a"/></svg><svg/>`)
	if err != nil {
		t.Fatal(err)
	}
	body := bodyOf(t, doc)
	svg := elementChild(t, body, 0)
	if len(svg.Children) != 3 {
		t.Fatalf("svg children: %d, want 3 (no implicit nesting from self-closing)", len(svg.Children))
	}
}

func TestParseForeignNoImplicitClose(t *testing.T) {
	t.Parallel()
	// HTML sibling-closing rules never apply inside foreign content:
	// the second p nests inside the first instead of closing it.
	doc, err := Parse(`<svg><p>a<p>b</p></p></svg>`)
	if err != nil {
		t.Fatal(err)
	}
	svg := elementChild(t, bodyOf(t, doc), 0)
	if len(svg.Children) != 1 {
		t.Fatalf("svg children: %d, want a single nested p chain", len(svg.Children))
	}
	outer := elementChild(t, svg, 0)
	if len(outer.Children) != 2 {
		t.Fatalf("outer p children: %d, want text and the nested p", len(outer.Children))
	}
}

func TestParseForeignObjectHTMLIntegration(t *testing.T) {
	t.Parallel()
	// Children of foreignObject parse with HTML rules again: the </div>
	// implicitly closes the open p inside it.
	doc, err := Parse(`<svg><foreignObject><div><p>a</div><p>b</p></foreignObject></svg>`)
	if err != nil {
		t.Fatal(err)
	}
	svg := elementChild(t, bodyOf(t, doc), 0)
	foreignObject := elementChild(t, svg, 0)
	if len(foreignObject.Children) != 2 {
		t.Fatalf("foreignObject children: %d, want div and p", len(foreignObject.Children))
	}
	div := elementChild(t, foreignObject, 0)
	if len(div.Children) != 1 || div.Children[0].(*Element).Name != "p" {
		t.Fatalf("div children: %v, want one implicitly closed p", div.Children)
	}
}

func TestParseForeignDescTitleHTMLIntegration(t *testing.T) {
	t.Parallel()
	// desc and title are SVG HTML integration points; markup inside
	// them parses with HTML rules, so the em nests normally.
	doc, err := Parse(`<svg><desc><p>a <em>b</em></p></desc><title><i>t</i></title></svg>`)
	if err != nil {
		t.Fatal(err)
	}
	svg := elementChild(t, bodyOf(t, doc), 0)
	desc := elementChild(t, svg, 0)
	p := elementChild(t, desc, 0)
	if len(p.Children) != 2 {
		t.Fatalf("p children: %d, want text and em", len(p.Children))
	}
}

func TestParseForeignMath(t *testing.T) {
	t.Parallel()
	// MathML text integration points (mi/mo/mn/ms/mtext) take normal
	// character data, with character references decoded.
	doc, err := Parse(`<math><mi>x</mi><mo>+</mo><mn>1</mn><ms>&amp;</ms><mtext>a &amp; b</mtext></math>`)
	if err != nil {
		t.Fatal(err)
	}
	math := elementChild(t, bodyOf(t, doc), 0)
	if len(math.Children) != 5 {
		t.Fatalf("math children: %d, want 5", len(math.Children))
	}
	mtext := elementChild(t, math, 4)
	text, ok := mtext.Children[0].(*Text)
	if !ok {
		t.Fatalf("mtext child: %T", mtext.Children[0])
	}
	if text.Value != "a & b" {
		t.Fatalf("mtext value: %q", text.Value)
	}
}

func TestParseForeignMathAnnotationXML(t *testing.T) {
	t.Parallel()
	// annotation-xml with an HTML encoding is an HTML integration
	// point; its attribute names get the MathML adjustment.
	source := `<math><mi definitionurl="u">x</mi><annotation-xml encoding="application/xhtml+xml"><p>hi</p></annotation-xml></math>`
	doc, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	math := elementChild(t, bodyOf(t, doc), 0)
	mi := elementChild(t, math, 0)
	if got := mi.Attrs[0].Name; got != "definitionURL" {
		t.Fatalf("adjusted attr: %q, want definitionURL", got)
	}
	annotation := elementChild(t, math, 1)
	if got := annotation.Attrs[0].Name; got != "encoding" {
		t.Fatalf("encoding attr: %q", got)
	}
	if got := elementChild(t, annotation, 0).Name; got != "p" {
		t.Fatalf("annotation child: %q, want HTML p", got)
	}
}

func TestParseForeignEndTagRules(t *testing.T) {
	t.Parallel()
	invalid := map[string]string{
		// In foreign content the end tag must match the current
		// element; there are no implicit closes to pop through.
		`<svg><g></svg>`:                                   "end tag </svg> does not match open element <g>",
		`<math><mi>x</mo></math>`:                          "end tag </mo> does not match open element <mi>",
		`<svg><foreignObject><div>x</foreignObject></svg>`: "end tag </foreignObject> does not match open element <div>",
		// A self-closed foreign element is not on the stack, so its end
		// tag mismatches the still-open root.
		`<svg><path/></path></svg>`: "end tag </path> does not match open element <svg>",
		// Foreign roots must be closed like any other element.
		`<svg>`:         "unclosed element <svg>",
		`<svg><g></g>`:  "unclosed element <svg>",
		`<p><math></p>`: "end tag </p> does not match open element <math>",
		// Inside foreign content, attribute names case-fold for the
		// duplicate check even where no adjustment applies.
		`<svg viewBox="0 0 1 1" viewbox="1 1 1 1"></svg>`: "duplicate attribute viewbox",
	}
	for source, message := range invalid {
		_, err := Parse(source)
		if err == nil {
			t.Errorf("%q: expected error", source)
			continue
		}
		if err.Error() != message {
			t.Errorf("%q: got %q, want %q", source, err.Error(), message)
		}
	}
	// End tags match the foreign element case-insensitively, so a
	// differently-cased root end tag still closes its root.
	for _, source := range []string{
		`<svg><g></g></SVG>`,
		`<svg><foreignObject></foreignobject></svg>`,
		`<math><Mi>x</mi></math>`,
	} {
		if _, err := Parse(source); err != nil {
			t.Errorf("%q: unexpected error %v", source, err)
		}
	}
}

func TestTokenizeForeignSuppressesRawText(t *testing.T) {
	t.Parallel()
	// Inside foreign content the lexer never enters raw-text or rcdata
	// mode: an svg title's children tokenize as markup.
	tokens, err := Tokenize(`<svg><title><b>x</b></title></svg>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range tokens {
		if token.Kind == KindRCDATA || token.Kind == KindRawText {
			t.Fatalf("unexpected %s token in foreign content", token.Kind)
		}
	}
	// Once foreign content ends, rcdata elements behave normally again.
	tokens, err = Tokenize(`<svg></svg><title>a</title>`)
	if err != nil {
		t.Fatal(err)
	}
	if !containsKind(tokens, KindRCDATA) {
		t.Fatalf("no rcdata token after foreign content: %v", kindsOf(tokens))
	}
}

func containsKind(tokens []gast.Token, kind string) bool {
	for _, token := range tokens {
		if token.Kind == kind {
			return true
		}
	}
	return false
}

func kindsOf(tokens []gast.Token) []string {
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		result = append(result, token.Kind)
	}
	return result
}

func TestParseForeignNestedRoots(t *testing.T) {
	t.Parallel()
	// Nested svg roots (an svg inside a foreignObject re-entering
	// foreign content) all close properly, and an inner svg keeps the
	// case rules while the outer HTML flow resumes after it.
	source := `<div><svg><foreignObject><svg viewBox="0 0 2 2"><circle r="1"/></svg><p>back to HTML</p></foreignObject></svg><p>after</p></div>`
	doc, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	div := elementChild(t, bodyOf(t, doc), 0)
	if len(div.Children) != 2 {
		t.Fatalf("div children: %d, want svg and p", len(div.Children))
	}
	svg := elementChild(t, div, 0)
	foreignObject := elementChild(t, svg, 0)
	inner := elementChild(t, foreignObject, 0)
	if inner.Name != "svg" || inner.Attrs[0].Name != "viewBox" {
		t.Fatalf("inner svg: %q %q", inner.Name, inner.Attrs[0].Name)
	}
	if got := elementChild(t, foreignObject, 1).Name; got != "p" {
		t.Fatalf("foreignObject child 1: %q, want p", got)
	}
	if !strings.EqualFold(elementChild(t, div, 1).Name, "p") {
		t.Fatal("div child 1 must be the HTML p after the svg")
	}
}
