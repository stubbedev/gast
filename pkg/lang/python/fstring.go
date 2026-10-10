package python

import (
	"github.com/stubbedev/gast/pkg/gast"
)

// parseStringChain parses adjacent string literals, including f-string
// and bytes tokens, folding them into one literal or JoinedStr.
func (p *parser) parseStringChain() (Expr, error) {
	start := p.Current().Span
	var parts []Expr
	sawF, sawBytes, sawStr := false, false, false
loop:
	for {
		switch p.Current().Kind {
		case KindString:
			tok := p.Advance()
			sawStr = true
			parts = append(parts, &StringLit{Span: tok.Span, Text: tok.Text, Value: decodeString(tok.Text)})
		case KindBytes:
			tok := p.Advance()
			sawBytes = true
			parts = append(parts, &BytesLit{Span: tok.Span, Text: tok.Text, Value: decodeString(tok.Text)})
		case KindFStringStart:
			tok := p.Advance()
			sawF = true
			joined, err := p.parseFString(tok)
			if err != nil {
				return nil, err
			}
			parts = append(parts, joined.Parts...)
		default:
			break loop
		}
	}
	if sawBytes && (sawF || sawStr) {
		return nil, p.FailAt(start, "cannot mix bytes and nonbytes literals")
	}
	if !sawF {
		if sawBytes {
			node := &BytesLit{Span: p.SpanFrom(start)}
			for _, part := range parts {
				if lit, ok := part.(*BytesLit); ok {
					node.Text += lit.Text
					node.Value += lit.Value
				}
			}
			if len(parts) == 1 {
				return parts[0], nil
			}
			return node, nil
		}
		node := &StringLit{Span: p.SpanFrom(start)}
		for _, part := range parts {
			if lit, ok := part.(*StringLit); ok {
				node.Text += lit.Text
				node.Value += lit.Value
			}
		}
		if len(parts) == 1 {
			return parts[0], nil
		}
		return node, nil
	}
	node := &JoinedStr{Parts: mergeLiteralParts(parts)}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// mergeLiteralParts joins runs of adjacent literal parts so implicit
// concatenation yields one Constant per run.
func mergeLiteralParts(parts []Expr) []Expr {
	var merged []Expr
	for _, part := range parts {
		lit, ok := part.(*StringLit)
		if !ok {
			merged = append(merged, part)
			continue
		}
		if last := len(merged) - 1; last >= 0 {
			if prev, ok := merged[last].(*StringLit); ok {
				prev.Text += lit.Text
				prev.Value += lit.Value
				prev.Span = gast.Merge(prev.Span, lit.Span)
				continue
			}
		}
		merged = append(merged, lit)
	}
	return merged
}

// fstringIsRaw reports whether an f-string start token carries the r
// prefix.
func fstringIsRaw(text string) bool {
	for i := range len(text) {
		switch text[i] {
		case 'r', 'R':
			return true
		case '"', '\'':
			return false
		}
	}
	return false
}

// parseFString parses the parts between an FStringStart token and its
// closing FStringEnd.
func (p *parser) parseFString(start gast.Token) (*JoinedStr, error) {
	raw := fstringIsRaw(start.Text)
	var parts []Expr
	for {
		tok := p.Current()
		switch tok.Kind {
		case KindFStringMiddle:
			p.Advance()
			parts = append(parts, &StringLit{
				Span:  tok.Span,
				Text:  tok.Text,
				Value: decodeFStringMiddle(tok.Text, raw),
			})
		case LBrace:
			p.Advance()
			field, err := p.parseFormattedField(raw)
			if err != nil {
				return nil, err
			}
			parts = append(parts, field)
		case KindFStringEnd:
			end := p.Advance()
			node := &JoinedStr{Parts: mergeLiteralParts(parts)}
			node.Span = gast.Merge(start.Span, end.Span)
			return node, nil
		default:
			return nil, p.Fail("unexpected " + gast.Describe(tok) + " in f-string")
		}
	}
}

// parseFormattedField parses one `{value!conv:spec}` interpolation;
// the '{' is already consumed and the '}' is consumed here.
func (p *parser) parseFormattedField(raw bool) (*FormattedValue, error) {
	value, err := p.parseTest()
	if err != nil {
		return nil, err
	}
	node := &FormattedValue{Value: value}
	node.Debug = p.Match(Assign)
	if tok, ok := p.matchToken(KindConversion); ok {
		node.Conversion = string(tok.Text[1])
	}
	if p.Match(Colon) {
		var specParts []Expr
		for !p.Check(RBrace) {
			tok := p.Current()
			switch tok.Kind {
			case KindFStringMiddle:
				p.Advance()
				specParts = append(specParts, &StringLit{
					Span:  tok.Span,
					Text:  tok.Text,
					Value: decodeFStringMiddle(tok.Text, raw),
				})
			case LBrace:
				p.Advance()
				nested, err := p.parseFormattedField(raw)
				if err != nil {
					return nil, err
				}
				specParts = append(specParts, nested)
			default:
				return nil, p.Fail("unexpected " + gast.Describe(tok) + " in format spec")
			}
		}
		if len(specParts) > 0 {
			spec := &JoinedStr{Parts: mergeLiteralParts(specParts)}
			spec.Span = gast.Merge(specParts[0].Pos(), specParts[len(specParts)-1].Pos())
			node.FormatSpec = spec
		}
	}
	closeTok, err := p.Expect(RBrace, "'}' to close f-string field")
	if err != nil {
		return nil, err
	}
	node.Span = gast.Merge(value.Pos(), closeTok.Span)
	return node, nil
}
