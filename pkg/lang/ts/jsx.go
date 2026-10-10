package ts

import (
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

// This file implements the JSX grammar available in TSX mode
// (ParseTSX): elements, fragments, attributes (string, expression,
// boolean, and spread forms), and children (raw text, nested elements,
// and expression containers). The lazy lexer's mode is switched along
// the way: JSX text - including entities like &amp; - is never decoded.

// parseJSXElement parses a JSX element or fragment as an expression.
func (p *parser) parseJSXElement() (Expr, error) {
	node, err := p.parseJSXElementNode()
	if err != nil {
		return nil, err
	}
	return node, nil
}

// parseJSXElementNode parses a JSX element or fragment; the '<' is
// current. An element without a closing tag fails at its opening '<',
// matching tsc's unclosed-element diagnostic (which is also how a
// legacy angle-bracket assertion surfaces in TSX mode, where it is
// illegal).
func (p *parser) parseJSXElementNode() (*JSXElement, error) {
	saved := p.lexMode()
	defer p.setLexMode(saved)
	lt := p.Advance()
	p.setLexMode(lexTag)
	node := &JSXElement{}
	var openName *JSXName
	if p.Match(Gt) {
		node.Fragment = true
	} else {
		name, err := p.parseJSXName(true, "element name")
		if err != nil {
			return nil, err
		}
		openName = name
		node.Name = name
		attrs, selfClosing, err := p.parseJSXAttrs()
		if err != nil {
			return nil, err
		}
		node.Attrs = attrs
		node.SelfClosing = selfClosing
		if selfClosing {
			node.Span = gast.Merge(lt.Span, p.Peek(-1).Span)
			return node, nil
		}
	}
	p.setLexMode(lexText)
	if err := p.parseJSXChildren(node, lt, openName); err != nil {
		return nil, err
	}
	node.Span = gast.Merge(lt.Span, p.Peek(-1).Span)
	return node, nil
}

// parseJSXName parses a tag or attribute name: identifier (member
// chains allowed for tags, dashes as in data-id, or a namespaced
// `svg:path` form). The joined spelling is used in diagnostics.
func (p *parser) parseJSXName(memberOK bool, what string) (*JSXName, error) {
	first, err := p.jsxIdentifier(what)
	if err != nil {
		return nil, err
	}
	node := &JSXName{Parts: []string{first.Text}}
	node.Span = first.Span
	if p.Match(Colon) {
		second, err := p.jsxIdentifier(what + " after ':'")
		if err != nil {
			return nil, err
		}
		node.Parts = append(node.Parts, second.Text)
		node.Namespace = true
		node.Span = gast.Merge(first.Span, second.Span)
		return node, nil
	}
	p.extendDashedName(node)
	for memberOK && p.Match(Dot) {
		part, err := p.jsxIdentifier(what + " after '.'")
		if err != nil {
			return nil, err
		}
		node.Parts = append(node.Parts, part.Text)
		node.Span = gast.Merge(node.Span, part.Span)
		p.extendDashedName(node)
	}
	return node, nil
}

// extendDashedName folds '-ident' runs into the last name part: JSX
// identifiers, unlike JavaScript's, may contain dashes (data-id).
func (p *parser) extendDashedName(node *JSXName) {
	for p.Check(Minus) && (p.CheckAhead(1, KindIdent) || p.CheckAhead(1, KindNum)) {
		p.Advance()
		part := p.Advance()
		last := len(node.Parts) - 1
		node.Parts[last] += "-" + part.Text
		node.Span = gast.Merge(node.Span, part.Span)
	}
}

// jsxIdentifier consumes an identifierName; reserved words are valid
// JSX names.
func (p *parser) jsxIdentifier(what string) (gast.Token, error) {
	tok := p.Current()
	if tok.Kind == KindIdent || isReservedWord(tok) {
		return p.Advance(), nil
	}
	return gast.Token{}, p.Fail("expected " + what + ", found " + gast.Describe(tok))
}

// jsxNameText renders a name for diagnostics, honoring its separator.
func jsxNameText(name *JSXName) string {
	if name == nil {
		return ""
	}
	sep := "."
	if name.Namespace {
		sep = ":"
	}
	return strings.Join(name.Parts, sep)
}

// parseJSXAttrs parses an attribute list up to the '>' or '/>' closing
// the opening tag; the bool reports the self-closing form.
func (p *parser) parseJSXAttrs() ([]*JSXAttr, bool, error) {
	var attrs []*JSXAttr
	for {
		switch p.Current().Kind {
		case Gt:
			p.Advance()
			return attrs, false, nil
		case Slash:
			p.Advance()
			if _, err := p.Expect(Gt, "'>' after '/' in self-closing element"); err != nil {
				return nil, false, err
			}
			return attrs, true, nil
		case KindEOF:
			return nil, false, p.Fail("expected '>' to close element, found end of input")
		}
		attr, err := p.parseJSXAttr()
		if err != nil {
			return nil, false, err
		}
		attrs = append(attrs, attr)
	}
}

// parseJSXAttr parses one attribute: `name`, `name="v"`, `name={expr}`,
// or a spread `{...expr}`.
func (p *parser) parseJSXAttr() (*JSXAttr, error) {
	start := p.Current().Span
	if p.Check(LBrace) {
		// {...spread}: the '{' and '}' are single characters in tag
		// mode, the expression between them is ordinary code.
		open := p.Advance()
		saved := p.lexMode()
		p.setLexMode(lexJS)
		defer p.setLexMode(saved)
		if _, err := p.Expect(Ellipsis, "'...' in spread attribute"); err != nil {
			return nil, err
		}
		expr, err := p.parseAssignExpr(false)
		if err != nil {
			return nil, err
		}
		close, err := p.Expect(RBrace, "'}' to close spread attribute")
		if err != nil {
			return nil, err
		}
		return &JSXAttr{Span: gast.Merge(open.Span, close.Span), Spread: expr}, nil
	}
	name, err := p.parseJSXName(false, "attribute name")
	if err != nil {
		return nil, err
	}
	attr := &JSXAttr{Name: *name}
	if p.Match(Assign) {
		switch p.Current().Kind {
		case KindString:
			tok := p.Advance()
			attr.Value = &StringLit{Span: tok.Span, Text: tok.Text}
		case LBrace:
			value, err := p.parseJSXContainer(true)
			if err != nil {
				return nil, err
			}
			attr.Value = value
		default:
			return nil, p.Fail("expected string or '{' after attribute name, found " + gast.Describe(p.Current()))
		}
	}
	attr.Span = p.SpanFrom(start)
	return attr, nil
}

// parseJSXContainer parses a '{' expression container; the '{' is
// current and was pulled in the enclosing mode, so the expression is
// lexed as ordinary code and the mode restored at the '}'. Empty
// containers are allowed only as children (where they hold comments),
// matching tsc's JSX attributes rule.
func (p *parser) parseJSXContainer(attribute bool) (*JSXExprContainer, error) {
	open := p.Advance()
	saved := p.lexMode()
	defer p.setLexMode(saved)
	p.setLexMode(lexJS)
	node := &JSXExprContainer{}
	if p.Check(RBrace) {
		if attribute {
			return nil, p.Fail("expected expression in JSX attribute container")
		}
	} else {
		expr, err := p.parseAssignExpr(false)
		if err != nil {
			return nil, err
		}
		node.Expr = expr
	}
	close, err := p.Expect(RBrace, "'}' to close expression container")
	if err != nil {
		return nil, err
	}
	node.Span = gast.Merge(open.Span, close.Span)
	return node, nil
}

// parseJSXChildren parses children until the matching closing tag.
// The lazy lexer is in text mode: raw runs arrive as KindJSXText
// tokens (sliced from the source below so spans stay contiguous), and
// boundaries are a '<' opening an element or closing tag, a '{'
// container, or the end of input.
func (p *parser) parseJSXChildren(node *JSXElement, lt gast.Token, openName *JSXName) error {
	for {
		textStart := p.Peek(-1).Span.End
		for !p.jsxChildBoundary() {
			tok := p.Advance()
			if i := strings.IndexByte(tok.Text, '}'); i >= 0 {
				brace := gast.Span{Start: tok.Span.Start + i, End: tok.Span.Start + i + 1}
				return p.FailAt(brace, "unexpected '}' in JSX text; write {'}'} instead")
			}
		}
		textEnd := p.Current().Span.Start
		if textEnd > textStart {
			node.Children = append(node.Children, &JSXText{
				gast.Span{Start: textStart, End: textEnd}, p.source[textStart:textEnd],
			})
		}
		switch p.Current().Kind {
		case Lt:
			p.setLexMode(lexTag)
			if p.CheckAhead(1, Slash) {
				return p.parseJSXClosingTag(node, openName)
			}
			child, err := p.parseJSXElementNode()
			if err != nil {
				return err
			}
			node.Children = append(node.Children, child)
		case LBrace:
			child, err := p.parseJSXContainer(false)
			if err != nil {
				return err
			}
			node.Children = append(node.Children, child)
		default: // end of input
			if node.Fragment {
				return p.FailAt(lt.Span, "JSX fragment has no corresponding closing tag")
			}
			return p.FailAt(lt.Span, "JSX element '"+jsxNameText(openName)+"' has no corresponding closing tag")
		}
	}
}

// jsxChildBoundary reports whether the current token ends a JSX text
// run. In lazy text mode the lexer already folds raw runs into single
// KindJSXText tokens, so only the structural tokens are boundaries.
func (p *parser) jsxChildBoundary() bool {
	switch p.Current().Kind {
	case Lt, LBrace, KindEOF:
		return true
	}
	return false
}

// parseJSXClosingTag consumes `</name>` or `</>` and checks it matches
// the opening tag, failing at the opening '<' for a mismatch, like
// tsc's corresponding-closing-tag diagnostic.
func (p *parser) parseJSXClosingTag(node *JSXElement, openName *JSXName) error {
	p.Advance()            // '<'
	closeLt := p.Advance() // '/'
	if node.Fragment {
		if _, err := p.Expect(Gt, "'>' to close JSX fragment"); err != nil {
			return err
		}
		return nil
	}
	name, err := p.parseJSXName(true, "closing element name")
	if err != nil {
		return err
	}
	if jsxNameText(name) != jsxNameText(openName) || name.Namespace != openName.Namespace {
		return p.FailAt(closeLt.Span,
			"expected corresponding JSX closing tag for '"+jsxNameText(openName)+
				"', found '"+jsxNameText(name)+"'")
	}
	if _, err := p.Expect(Gt, "'>' to close element"); err != nil {
		return err
	}
	return nil
}
