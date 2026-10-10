package html

import (
	"slices"
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

// frame is one open element on the tree builder stack.
type frame struct {
	el        *Element
	startSpan gast.Span // span of the start tag, for unclosed-element errors
	lastEnd   int       // end offset of the last node inserted into the element
	ns        string    // namespace: HTML, or foreign svg/math content
}

// parser is a tree builder over the token stream. Nodes are appended
// to the nearest open element, or to the document while the stack is
// empty.
type parser struct {
	*gast.TokenCursor
	source     string
	children   []Node
	stack      []*frame
	sawContent bool
}

func newParser(tokens []gast.Token, source string) *parser {
	if len(tokens) > 0 && tokens[len(tokens)-1].Kind == KindEOF {
		tokens = tokens[:len(tokens)-1]
	}
	return &parser{TokenCursor: gast.NewTokenCursor(tokens, KindEOF), source: source}
}

func (p *parser) parseFile() (*File, error) {
	for !p.AtEnd() {
		if err := p.parseNode(); err != nil {
			return nil, err
		}
	}
	for len(p.stack) > 0 {
		top := p.stack[len(p.stack)-1]
		if !implicitlyClosable[strings.ToLower(top.el.Name)] {
			return nil, p.FailAt(top.startSpan, "unclosed element <"+top.el.Name+">")
		}
		p.pop(top.lastEnd)
	}
	file := &File{Children: p.children}
	file.Span = gast.Span{Start: 0, End: len(p.source)}
	return file.wrap(), nil
}

func (p *parser) parseNode() error {
	tok := p.Current()
	switch tok.Kind {
	case KindDoctype:
		if p.sawContent {
			return p.FailAt(tok.Span, "doctype must appear before any content")
		}
		p.sawContent = true
		p.Advance()
		name, publicID, systemID := parseDoctypeText(tok.Text)
		p.insert(&Doctype{Span: tok.Span, Name: name, PublicID: publicID, SystemID: systemID})
	case KindComment:
		p.Advance()
		p.insert(&Comment{Span: tok.Span, Text: commentText(tok.Text)})
	case KindCDATA:
		p.sawContent = true
		p.Advance()
		p.insert(&CDATA{Span: tok.Span, Text: tok.Text[len(cdataStart) : len(tok.Text)-len(cdataEnd)]})
	case KindText:
		if strings.TrimSpace(tok.Text) != "" {
			p.sawContent = true
		}
		p.Advance()
		value, err := decodeEntities(tok.Text, tok.Span.Start, false)
		if err != nil {
			return err
		}
		p.insert(&Text{Span: tok.Span, Text: tok.Text, Value: value})
	case KindTagOpen:
		return p.startTag()
	case KindEndTagOpen:
		return p.endTag()
	default:
		return p.Fail("unexpected " + gast.Describe(tok))
	}
	return nil
}

// startTag builds an element from a start tag. The element is appended
// after implicitly closable siblings have been closed; void and
// self-closed elements stay childless, and raw-text or rcdata elements
// take their content immediately. Inside foreign content no implicit
// closing or void rules apply, self-closing works for any element, and
// known SVG tag names are case-adjusted.
func (p *parser) startTag() error {
	tok := p.Current()
	p.sawContent = true
	name := strings.TrimPrefix(tok.Text, lt)
	lower := strings.ToLower(name)
	parentNS := p.ns()
	if parentNS == nsSVG {
		if adjusted := svgTagAdjust[lower]; adjusted != "" {
			name = adjusted
		}
	}
	p.Advance()
	attrs, closeSpan, selfClosed, err := p.attributes(parentNS)
	if err != nil {
		return err
	}
	if parentNS == nsHTML {
		p.closeImplicit(lower)
	}
	el := &Element{Name: name, Attrs: attrs, Span: gast.Merge(tok.Span, closeSpan)}
	p.insert(el)
	switch {
	case selfClosed:
		return nil
	case parentNS == nsHTML && voidElements[lower]:
		return nil
	case parentNS == nsHTML && (rawTextElements[lower] || rcdataElements[lower]):
		return p.fillRaw(el)
	}
	p.stack = append(p.stack, &frame{
		el:        el,
		startSpan: tok.Span,
		lastEnd:   closeSpan.End,
		ns:        childNamespace(parentNS, lower, attrs),
	})
	return nil
}

// ns returns the namespace of the nearest open element - the insertion
// mode for the next node. An empty stack means HTML content.
func (p *parser) ns() string {
	if len(p.stack) > 0 {
		return p.stack[len(p.stack)-1].ns
	}
	return nsHTML
}

// attributes consumes attribute tokens and the tag-closing token.
// Duplicate attribute names are an error. Inside foreign content
// known attribute names are case-adjusted; values are always decoded.
func (p *parser) attributes(ns string) ([]Attribute, gast.Span, bool, error) {
	var attrs []Attribute
	seen := make(map[string]bool)
	for p.Check(KindAttr) {
		tok := p.Advance()
		name, value, valueOffset := splitAttribute(tok.Text)
		if ns != nsHTML {
			if adjusted := attributeAdjustment(ns, name); adjusted != "" {
				name = adjusted
			}
		}
		value, err := decodeEntities(value, tok.Span.Start+valueOffset, true)
		if err != nil {
			return nil, gast.Span{}, false, err
		}
		lower := strings.ToLower(name)
		if seen[lower] {
			return nil, gast.Span{}, false, p.FailAt(tok.Span, "duplicate attribute "+name)
		}
		seen[lower] = true
		attrs = append(attrs, Attribute{Span: tok.Span, Name: name, Value: value})
	}
	closeTok := p.Advance()
	if closeTok.Kind != KindTagClose && closeTok.Kind != KindSelfClose {
		return nil, gast.Span{}, false, p.FailAt(closeTok.Span, "expected '>' to close tag, found "+gast.Describe(closeTok))
	}
	return attrs, closeTok.Span, closeTok.Kind == KindSelfClose, nil
}

// fillRaw consumes the raw or rcdata content and the end tag of a
// script, style, title, or textarea element.
func (p *parser) fillRaw(el *Element) error {
	lower := strings.ToLower(el.Name)
	switch {
	case p.Check(KindRawText):
		tok := p.Advance()
		el.Children = append(el.Children, &RawText{Span: tok.Span, Kind: lower, Text: tok.Text})
	case p.Check(KindRCDATA):
		tok := p.Advance()
		value, err := decodeEntities(tok.Text, tok.Span.Start, false)
		if err != nil {
			return err
		}
		el.Children = append(el.Children, &Text{Span: tok.Span, Text: tok.Text, Value: value})
	}
	name, span, err := p.consumeEndTag()
	if err != nil {
		return err
	}
	if !strings.EqualFold(name, el.Name) {
		return p.FailAt(span, "expected </"+el.Name+">")
	}
	el.End = span.End
	return nil
}

// endTag closes the nearest matching open element, implicitly closing
// intervening closable elements. Any other end tag is an error, except
// a stray </p>, which is tolerated (and ignored) per the spec's
// recovery rule. Inside foreign content there are no implicit closes:
// the end tag must match the current element, case-insensitively.
func (p *parser) endTag() error {
	name, span, err := p.consumeEndTag()
	if err != nil {
		return err
	}
	if p.ns() != nsHTML {
		top := p.stack[len(p.stack)-1]
		if !strings.EqualFold(top.el.Name, name) {
			return p.FailAt(span, "end tag </"+name+"> does not match open element <"+top.el.Name+">")
		}
		p.pop(span.End)
		return nil
	}
	lower := strings.ToLower(name)
	if voidElements[lower] {
		return p.FailAt(span, "void element <"+name+"> must not have an end tag")
	}
	for i, candidate := range slices.Backward(p.stack) {
		if !strings.EqualFold(candidate.el.Name, name) {
			continue
		}
		for _, open := range p.stack[i+1:] {
			if !implicitlyClosable[strings.ToLower(open.el.Name)] {
				return p.FailAt(span, "end tag </"+name+"> does not match open element <"+open.el.Name+">")
			}
		}
		for len(p.stack)-1 > i {
			p.pop(p.stack[len(p.stack)-1].lastEnd)
		}
		p.pop(span.End)
		return nil
	}
	if lower == elementP {
		return nil
	}
	return p.FailAt(span, "stray end tag </"+name+">")
}

// consumeEndTag consumes an end tag, rejecting attributes, and returns
// its name and span.
func (p *parser) consumeEndTag() (string, gast.Span, error) {
	tok := p.Advance()
	if p.Check(KindAttr) {
		first := p.Advance()
		span := first.Span
		for p.Check(KindAttr) {
			span = gast.Merge(span, p.Advance().Span)
		}
		return "", gast.Span{}, p.FailAt(span, "end tag must not have attributes")
	}
	if !p.Check(KindTagClose) {
		return "", gast.Span{}, p.Fail("expected " + tagCloseText + " to close end tag, found " + gast.Describe(p.Current()))
	}
	return strings.TrimPrefix(tok.Text, endTagPrefix), gast.Merge(tok.Span, p.Advance().Span), nil
}

// closeImplicit closes open elements that lower implicitly closes
// when it starts.
func (p *parser) closeImplicit(lower string) {
	for closes := implicitClose[lower]; len(p.stack) > 0; {
		top := p.stack[len(p.stack)-1]
		if !slices.Contains(closes, strings.ToLower(top.el.Name)) {
			return
		}
		p.pop(top.lastEnd)
	}
}

// pop closes the top frame, ending its span at end.
func (p *parser) pop(end int) {
	f := p.stack[len(p.stack)-1]
	p.stack = p.stack[:len(p.stack)-1]
	f.el.End = end
	if len(p.stack) > 0 {
		p.stack[len(p.stack)-1].lastEnd = end
	}
}

// insert appends node to the nearest open element, or to the document.
func (p *parser) insert(node Node) {
	if len(p.stack) > 0 {
		top := p.stack[len(p.stack)-1]
		top.el.Children = append(top.el.Children, node)
		top.lastEnd = node.Pos().End
		return
	}
	p.children = append(p.children, node)
}

// wrap synthesizes the html/head/body wrappers when the document opens
// no html element itself. Leading comments, doctypes, and whitespace
// stay at the file level; leading head-eligible elements move into a
// synthesized head, the rest into a synthesized body. Synthesized
// elements carry no span.
func (f *File) wrap() *File {
	for _, node := range f.Children {
		if el, ok := node.(*Element); ok && strings.EqualFold(el.Name, elementHTML) {
			return f
		}
	}
	split := len(f.Children)
	for i, node := range f.Children {
		if isContent(node) {
			split = i
			break
		}
	}
	rest := f.Children[split:]
	headEnd := len(rest)
	for i, node := range rest {
		if !headEligible(node) {
			headEnd = i
			break
		}
	}
	lead := f.Children[:split]
	children := make([]Node, 0, len(lead)+1)
	children = append(children, lead...)
	children = append(children, &Element{Name: elementHTML, Children: []Node{
		&Element{Name: elementHead, Children: rest[:headEnd]},
		&Element{Name: elementBody, Children: rest[headEnd:]},
	}})
	f.Children = children
	return f
}

// isContent reports whether node starts document content: an element,
// a CDATA section, or non-whitespace text.
func isContent(node Node) bool {
	switch n := node.(type) {
	case *Element:
		return true
	case *CDATA:
		return true
	case *Text:
		return strings.TrimSpace(n.Text) != ""
	}
	return false
}

// headEligible reports whether node may continue the leading run of a
// synthesized head.
func headEligible(node Node) bool {
	switch n := node.(type) {
	case *Comment:
		return true
	case *Text:
		return strings.TrimSpace(n.Text) == ""
	case *Element:
		return headElements[strings.ToLower(n.Name)]
	}
	return false
}

// commentText strips comment delimiters, for both real <!-- --> and
// bogus <! comments.
func commentText(text string) string {
	if strings.HasPrefix(text, commentStart) {
		return text[len(commentStart) : len(text)-len(commentEnd)]
	}
	return text[2 : len(text)-1]
}

// splitAttribute splits an attribute token's raw text into name,
// value, and the byte offset of the value within the raw text (used to
// position decoding errors). The value keeps its raw source text minus
// the surrounding quotes; entities are not decoded here. A valueless
// attribute has an empty value.
func splitAttribute(text string) (string, string, int) {
	name, rest, found := strings.Cut(text, "=")
	if !found {
		return name, "", len(text)
	}
	offset := len(name) + 1
	value := strings.TrimLeft(rest, " \t\n\r\f")
	offset += len(rest) - len(value)
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' || first == '\'') && first == last {
			value = value[1 : len(value)-1]
			offset++
		}
	}
	return strings.TrimRight(name, " \t\n\r\f"), value, offset
}

// attributeAdjustment returns the case-adjusted spelling of a foreign
// attribute name, or "" when the name has no adjustment.
func attributeAdjustment(ns, name string) string {
	if ns == nsMath {
		return mathAttrAdjust[strings.ToLower(name)]
	}
	return svgAttrAdjust[strings.ToLower(name)]
}
