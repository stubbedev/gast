package html

import (
	"strings"
	"testing"

	"github.com/stubbedev/gast/pkg/gast"
)

func bodyOf(t *testing.T, doc *File) *Element {
	t.Helper()
	if len(doc.Children) == 0 {
		t.Fatal("document has no children")
	}
	html, ok := doc.Children[len(doc.Children)-1].(*Element)
	if !ok || !strings.EqualFold(html.Name, elementHTML) {
		t.Fatalf("last child: got %T", doc.Children[len(doc.Children)-1])
	}
	body, ok := html.Children[len(html.Children)-1].(*Element)
	if !ok || !strings.EqualFold(body.Name, elementBody) {
		t.Fatalf("html child: got %T", html.Children[len(html.Children)-1])
	}
	return body
}

// elementChild returns the index-th child of a File or Element,
// asserting it is an element.
func elementChild(t *testing.T, node Node, index int) *Element {
	t.Helper()
	var children []Node
	switch n := node.(type) {
	case *File:
		children = n.Children
	case *Element:
		children = n.Children
	default:
		t.Fatalf("expected *File or *Element, got %T", node)
	}
	if index >= len(children) {
		t.Fatalf("node has %d children, want index %d", len(children), index)
	}
	child, ok := children[index].(*Element)
	if !ok {
		t.Fatalf("child %d: got %T", index, children[index])
	}
	return child
}

// namedChild returns the first element child with the given name.
func namedChild(t *testing.T, node Node, name string) *Element {
	t.Helper()
	var children []Node
	switch n := node.(type) {
	case *File:
		children = n.Children
	case *Element:
		children = n.Children
	default:
		t.Fatalf("expected *File or *Element, got %T", node)
	}
	for _, child := range children {
		if el, ok := child.(*Element); ok && strings.EqualFold(el.Name, name) {
			return el
		}
	}
	t.Fatalf("no <%s> child", name)
	return nil
}

func TestParseStructure(t *testing.T) {
	t.Parallel()
	doc, err := Parse("<!DOCTYPE html>\n<html>\n<head><title>T</title></head>\n<body><p>hi <em>there</em></p></body>\n</html>\n")
	if err != nil {
		t.Fatal(err)
	}
	doctype, ok := doc.Children[0].(*Doctype)
	if !ok {
		t.Fatalf("child 0: got %T", doc.Children[0])
	}
	if doctype.Name != "html" {
		t.Fatalf("doctype name: %q", doctype.Name)
	}
	html := elementChild(t, doc, 2)
	if !strings.EqualFold(html.Name, elementHTML) {
		t.Fatalf("html element: %q", html.Name)
	}
	head := namedChild(t, html, elementHead)
	if !strings.EqualFold(head.Name, elementHead) {
		t.Fatalf("head element: %q", head.Name)
	}
	title := elementChild(t, head, 0)
	if len(title.Children) != 1 {
		t.Fatalf("title children: %d", len(title.Children))
	}
	if text, ok := title.Children[0].(*Text); !ok || text.Text != "T" {
		t.Fatalf("title text: %T %q", title.Children[0], title.Children[0])
	}
	body := namedChild(t, html, elementBody)
	p := elementChild(t, body, 0)
	if len(p.Children) != 2 {
		t.Fatalf("p children: %d", len(p.Children))
	}
	if text, ok := p.Children[0].(*Text); !ok || text.Text != "hi " {
		t.Fatalf("p text: %T %q", p.Children[0], p.Children[0])
	}
	em := elementChild(t, p, 1)
	if len(em.Children) != 1 {
		t.Fatalf("em children: %d", len(em.Children))
	}
}

func TestParseSynthesizesWrapper(t *testing.T) {
	t.Parallel()
	doc, err := Parse("<title>t</title>\n<meta charset=\"utf-8\">\n<p>body</p>\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Children) != 1 {
		t.Fatalf("children: %d", len(doc.Children))
	}
	html := elementChild(t, doc, 0)
	if html.Span != (gast.Span{}) {
		t.Fatalf("synthesized html must have no span, got %v", html.Span)
	}
	if len(html.Children) != 2 {
		t.Fatalf("html children: %d", len(html.Children))
	}
	head := elementChild(t, html, 0)
	if len(head.Children) != 4 {
		t.Fatalf("head children: %d", len(head.Children))
	}
	if got := elementChild(t, head, 0).Name; got != "title" {
		t.Fatalf("head child 0: %q", got)
	}
	if got := elementChild(t, head, 2).Name; got != "meta" {
		t.Fatalf("head child 2: %q", got)
	}
	body := elementChild(t, html, 1)
	if len(body.Children) != 2 { // p and the trailing newline text
		t.Fatalf("body children: %d", len(body.Children))
	}
	if got := elementChild(t, body, 0).Name; got != "p" {
		t.Fatalf("body child 0: %q", got)
	}
}

func TestParseImplicitClosing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		source string
		check  func(*testing.T, *Element)
	}{
		{
			name:   "li closes li",
			source: "<ul><li>one<li>two<li>three</ul>",
			check: func(t *testing.T, body *Element) {
				ul := elementChild(t, body, 0)
				if len(ul.Children) != 3 {
					t.Fatalf("ul children: %d", len(ul.Children))
				}
			},
		},
		{
			name:   "p closes p",
			source: "<p>one<p>two",
			check: func(t *testing.T, body *Element) {
				if len(body.Children) != 2 {
					t.Fatalf("body children: %d", len(body.Children))
				}
			},
		},
		{
			name:   "block closes p",
			source: "<div><p>text</div>",
			check: func(t *testing.T, body *Element) {
				div := elementChild(t, body, 0)
				if len(div.Children) != 1 {
					t.Fatalf("div children: %d", len(div.Children))
				}
				if got := elementChild(t, div, 0).Name; got != "p" {
					t.Fatalf("div child: %q", got)
				}
			},
		},
		{
			name:   "dt and dd close siblings",
			source: "<dl><dt>t1<dd>d1<dt>t2<dd>d2</dl>",
			check: func(t *testing.T, body *Element) {
				dl := elementChild(t, body, 0)
				if len(dl.Children) != 4 {
					t.Fatalf("dl children: %d", len(dl.Children))
				}
			},
		},
		{
			name:   "td and tr close siblings",
			source: "<table><tr><td>1<td>2<tr><td>3</table>",
			check: func(t *testing.T, body *Element) {
				table := elementChild(t, body, 0)
				if len(table.Children) != 2 {
					t.Fatalf("table children: %d", len(table.Children))
				}
				if len(elementChild(t, table, 0).Children) != 2 {
					t.Fatal("first row must have two cells")
				}
			},
		},
		{
			name:   "option closes option",
			source: "<select><option>a<option>b</select>",
			check: func(t *testing.T, body *Element) {
				selectEl := elementChild(t, body, 0)
				if len(selectEl.Children) != 2 {
					t.Fatalf("select children: %d", len(selectEl.Children))
				}
			},
		},
		{
			name:   "optgroup closes option",
			source: "<select><option>a<optgroup label=g><option>b</optgroup></select>",
			check: func(t *testing.T, body *Element) {
				selectEl := elementChild(t, body, 0)
				if len(selectEl.Children) != 2 {
					t.Fatalf("select children: %d", len(selectEl.Children))
				}
				group := elementChild(t, selectEl, 1)
				if len(group.Children) != 1 {
					t.Fatalf("optgroup children: %d", len(group.Children))
				}
			},
		},
		{
			name:   "tbody closes tr and td",
			source: "<table><tr><td>x<tbody><tr><td>y</table>",
			check: func(t *testing.T, body *Element) {
				table := elementChild(t, body, 0)
				if len(table.Children) != 2 {
					t.Fatalf("table children: %d", len(table.Children))
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			doc, err := Parse(test.source)
			if err != nil {
				t.Fatal(err)
			}
			test.check(t, bodyOf(t, doc))
		})
	}
}

func TestParseAttributes(t *testing.T) {
	t.Parallel()
	doc, err := Parse(`<div id=a class='b c' data-x="d e" hidden data-pad = "v"></div>`)
	if err != nil {
		t.Fatal(err)
	}
	body := bodyOf(t, doc)
	div := elementChild(t, body, 0)
	wantAttrs := []struct {
		name  string
		value string
	}{
		{"id", "a"},
		{"class", "b c"},
		{"data-x", "d e"},
		{"hidden", ""},
		{"data-pad", "v"},
	}
	if len(div.Attrs) != len(wantAttrs) {
		t.Fatalf("attrs: %d", len(div.Attrs))
	}
	for i, want := range wantAttrs {
		got := div.Attrs[i]
		if got.Name != want.name || got.Value != want.value {
			t.Fatalf("attr %d: got %q=%q, want %q=%q", i, got.Name, got.Value, want.name, want.value)
		}
	}
}

func TestParseRawText(t *testing.T) {
	t.Parallel()
	source := `<script>if (a </scriptx) {} var s = "</scr" + "ipt>";</script>`
	doc, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	script := elementChild(t, bodyOf(t, doc), 0)
	if len(script.Children) != 1 {
		t.Fatalf("script children: %d", len(script.Children))
	}
	raw, ok := script.Children[0].(*RawText)
	if !ok {
		t.Fatalf("script child: %T", script.Children[0])
	}
	if raw.Kind != "script" || raw.Text != `if (a </scriptx) {} var s = "</scr" + "ipt>";` {
		t.Fatalf("raw: %q %q", raw.Kind, raw.Text)
	}
	wantEnd := len(source)
	if script.End != wantEnd {
		t.Fatalf("script span end: got %d, want %d", script.End, wantEnd)
	}
}

func TestParseSpans(t *testing.T) {
	t.Parallel()
	source := `<p title="x">hi <b>bold</b></p>`
	doc, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	p := elementChild(t, bodyOf(t, doc), 0)
	if p.Span != (gast.Span{Start: 0, End: len(source)}) {
		t.Fatalf("p span: %v", p.Span)
	}
	bStart := strings.Index(source, "<b>")
	bEnd := strings.Index(source, "</b>") + len("</b>")
	if p.Children[1].Pos() != (gast.Span{Start: bStart, End: bEnd}) {
		t.Fatalf("b span: %v", p.Children[1].Pos())
	}
	if p.Children[0].Pos() != (gast.Span{Start: 13, End: 16}) {
		t.Fatalf("text span: %v", p.Children[0].Pos())
	}
	attrStart := strings.Index(source, `title=`)
	if p.Attrs[0].Span != (gast.Span{Start: attrStart, End: attrStart + len(`title="x"`)}) {
		t.Fatalf("attr span: %v", p.Attrs[0].Span)
	}
}

func TestParseDoctype(t *testing.T) {
	t.Parallel()
	tests := []struct {
		source   string
		name     string
		publicID string
		systemID string
	}{
		{source: "<!DOCTYPE html>", name: "html"},
		{source: `<!DOCTYPE html SYSTEM "about:legacy-compat">`, name: "html", systemID: `"about:legacy-compat"`},
		{
			source:   `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN" "http://www.w3.org/TR/html4/strict.dtd">`,
			name:     "HTML",
			publicID: `"-//W3C//DTD HTML 4.01//EN"`,
			systemID: `"http://www.w3.org/TR/html4/strict.dtd"`,
		},
	}
	for _, test := range tests {
		doc, err := Parse(test.source)
		if err != nil {
			t.Fatalf("%s: %v", test.source, err)
		}
		doctype, ok := doc.Children[0].(*Doctype)
		if !ok {
			t.Fatalf("%s: child 0: %T", test.source, doc.Children[0])
		}
		if doctype.Name != test.name || doctype.PublicID != test.publicID || doctype.SystemID != test.systemID {
			t.Fatalf("%s: got %q %q %q", test.source, doctype.Name, doctype.PublicID, doctype.SystemID)
		}
	}
}

func TestParseEndTagRules(t *testing.T) {
	t.Parallel()
	tolerated := []string{
		"<p>a</p></p><p>b",    // stray </p> ignored per spec recovery
		"<div><p>x</div></p>", // </p> after p already closed
	}
	for _, source := range tolerated {
		if _, err := Parse(source); err != nil {
			t.Errorf("%q: unexpected error %v", source, err)
		}
	}
	invalid := map[string]string{
		`</div>`:                         "stray end tag </div>",
		`<p>a</span></p>`:                "stray end tag </span>",
		`<p><b>x<i>y</b></i></p>`:        "end tag </b> does not match open element <i>",
		`<p>a<br></br></p>`:              "void element <br> must not have an end tag",
		`<div>a</div class="x">`:         "end tag must not have attributes",
		`<div>a</div hidden>`:            "end tag must not have attributes",
		`<div>`:                          "unclosed element <div>",
		`<div><p>x`:                      "unclosed element <div>",
		`<div id=a ID=b></div>`:          "duplicate attribute ID",
		`<p>x</p><!DOCTYPE html>`:        "doctype must appear before any content",
		`<!DOCTYPE html><!DOCTYPE html>`: "doctype must appear before any content",
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
}

func TestParseUnclosedAtEOF(t *testing.T) {
	t.Parallel()
	// Only implicitly closable elements may stay open at EOF.
	valid := []string{"<p>text", "<li>a", "<html><body><p>x", "<table><tr><td>x</table>"}
	for _, source := range valid {
		if _, err := Parse(source); err != nil {
			t.Errorf("%q: unexpected error %v", source, err)
		}
	}
	invalid := []string{
		"<div>",
		"<ul><li>one", // li is closable, ul is not
		"<span>x",
		"<table><tr><td>x",  // table itself is not closable
		"<select><option>a", // select is not closable
	}
	for _, source := range invalid {
		if _, err := Parse(source); err == nil {
			t.Errorf("%q: expected error", source)
		}
	}
}
