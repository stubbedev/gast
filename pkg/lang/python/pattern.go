package python

import (
	"github.com/stubbedev/gast/pkg/gast"
)

// parsePattern parses the full pattern grammar of a match case;
// comma-separated patterns form an open sequence pattern.
func (p *parser) parsePattern() (Pattern, error) {
	first, err := p.parseAsPattern()
	if err != nil {
		return nil, err
	}
	if !p.Check(Comma) {
		return first, nil
	}
	patterns := []Pattern{first}
	for p.Match(Comma) {
		if p.Check(Colon) {
			break
		}
		pattern, err := p.parseAsPattern()
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, pattern)
	}
	node := &MatchSequence{Patterns: patterns}
	node.Span = gast.Merge(first.Pos(), patterns[len(patterns)-1].Pos())
	return node, nil
}

// parseAsPattern parses an or-pattern with an optional `as name`
// binding; it is also one element of a sequence pattern.
func (p *parser) parseAsPattern() (Pattern, error) {
	or, err := p.parseOrPattern()
	if err != nil {
		return nil, err
	}
	if !p.Check(KindAs) {
		return or, nil
	}
	p.Advance()
	name, err := p.Expect(KindName, "capture name after 'as'")
	if err != nil {
		return nil, err
	}
	node := &MatchAs{Pattern: or, Name: name.Text}
	node.Span = gast.Merge(or.Pos(), name.Span)
	return node, nil
}

func (p *parser) parseOrPattern() (Pattern, error) {
	first, err := p.parseClosedPattern()
	if err != nil {
		return nil, err
	}
	if !p.Check(Pipe) {
		return first, nil
	}
	patterns := []Pattern{first}
	for p.Match(Pipe) {
		pattern, err := p.parseClosedPattern()
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, pattern)
	}
	node := &MatchOr{Patterns: patterns}
	node.Span = gast.Merge(first.Pos(), patterns[len(patterns)-1].Pos())
	return node, nil
}

func (p *parser) parseClosedPattern() (Pattern, error) {
	switch p.Current().Kind {
	case KindInt, KindFloat, KindComplex, KindString, KindBytes, KindTrue, KindFalse, KindNone, Minus, Plus:
		return p.parseLiteralPattern()
	case KindName:
		switch p.Peek(1).Kind {
		case LParen:
			return p.parseClassPattern()
		case Dot:
			return p.parseValuePattern()
		}
		return p.parseCapturePattern()
	case LParen:
		return p.parseParenSequencePattern()
	case LBracket:
		return p.parseBracketSequencePattern()
	case LBrace:
		return p.parseMappingPattern()
	}
	return nil, p.Fail("expected a pattern, found " + gast.Describe(p.Current()))
}

// parseLiteralPattern parses signed numbers, complex sums, strings,
// bytes, and the three singletons.
func (p *parser) parseLiteralPattern() (Pattern, error) {
	start := p.Current().Span
	value, err := p.parseSignedLiteral()
	if err != nil {
		return nil, err
	}
	if (p.Check(Plus) || p.Check(Minus)) && p.Peek(1).Kind == KindComplex {
		op := p.Advance()
		imag, err := p.parseSignedLiteral()
		if err != nil {
			return nil, err
		}
		sum := &BinOpExpr{Op: op.Kind, Left: value, Right: imag}
		sum.Span = p.SpanFrom(start)
		value = sum
	}
	switch lit := value.(type) {
	case *BoolLit, *NoneLit:
		node := &MatchSingleton{Value: value}
		node.Span = p.SpanFrom(start)
		return node, nil
	case *StringLit, *BytesLit, *IntLit, *FloatLit, *ComplexLit, *BinOpExpr, *UnaryOpExpr:
		node := &MatchValue{Value: value}
		node.Span = p.SpanFrom(start)
		return node, nil
	default:
		_ = lit
		return nil, p.FailAt(value.Pos(), "invalid literal pattern")
	}
}

// parseSignedLiteral parses a number or string literal with an
// optional sign.
func (p *parser) parseSignedLiteral() (Expr, error) {
	start := p.Current().Span
	if p.Check(Plus) || p.Check(Minus) {
		op := p.Advance()
		operand, err := p.parseSignedLiteral()
		if err != nil {
			return nil, err
		}
		node := &UnaryOpExpr{Op: op.Kind, Operand: operand}
		node.Span = p.SpanFrom(start)
		return node, nil
	}
	switch p.Current().Kind {
	case KindInt, KindFloat, KindComplex, KindString, KindBytes, KindFStringStart:
		return p.parseStringOrNumber()
	case KindTrue, KindFalse, KindNone:
		return p.parseAtom()
	}
	return nil, p.Fail("expected a literal pattern, found " + gast.Describe(p.Current()))
}

func (p *parser) parseStringOrNumber() (Expr, error) {
	switch p.Current().Kind {
	case KindString, KindBytes, KindFStringStart:
		return p.parseStringChain()
	default:
		return p.parseAtom()
	}
}

// parseCapturePattern parses a bare name; the underscore is the
// wildcard.
func (p *parser) parseCapturePattern() (Pattern, error) {
	tok := p.Advance()
	node := &MatchAs{Name: tok.Text}
	node.Span = tok.Span
	return node, nil
}

// parseValuePattern parses a dotted value pattern such as Color.RED.
func (p *parser) parseValuePattern() (Pattern, error) {
	start := p.Current().Span
	value, err := p.parseDottedValueName()
	if err != nil {
		return nil, err
	}
	node := &MatchValue{Value: value}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// parseDottedValueName parses name (. name)+ as an attribute chain.
func (p *parser) parseDottedValueName() (Expr, error) {
	tok := p.Advance()
	expr := Expr(&NameExpr{Span: tok.Span, ID: tok.Text})
	for p.Match(Dot) {
		name, err := p.Expect(KindName, "attribute name in value pattern")
		if err != nil {
			return nil, err
		}
		node := &AttributeExpr{Value: expr, Attr: name.Text}
		node.Span = gast.Merge(expr.Pos(), name.Span)
		expr = node
	}
	return expr, nil
}

// parseParenSequencePattern parses a parenthesized pattern: a group
// when it holds a single closed pattern, otherwise a sequence.
func (p *parser) parseParenSequencePattern() (Pattern, error) {
	open := p.Advance()
	if closeTok, ok := p.matchToken(RParen); ok {
		node := &MatchSequence{}
		node.Span = gast.Merge(open.Span, closeTok.Span)
		return node, nil
	}
	starred := p.Check(Star)
	first, err := p.parseMaybeStarPattern()
	if err != nil {
		return nil, err
	}
	if _, ok := p.matchToken(RParen); ok && !starred {
		return first, nil
	}
	patterns := []Pattern{first}
	for p.Match(Comma) {
		if p.Check(RParen) {
			break
		}
		pattern, err := p.parseMaybeStarPattern()
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, pattern)
	}
	closeTok, err := p.Expect(RParen, "')' to close sequence pattern")
	if err != nil {
		return nil, err
	}
	node := &MatchSequence{Patterns: patterns}
	node.Span = gast.Merge(open.Span, closeTok.Span)
	return node, nil
}

func (p *parser) parseBracketSequencePattern() (Pattern, error) {
	open := p.Advance()
	var patterns []Pattern
	for !p.Check(RBracket) && !p.AtEnd() {
		pattern, err := p.parseMaybeStarPattern()
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, pattern)
		if !p.Match(Comma) {
			break
		}
	}
	closeTok, err := p.Expect(RBracket, "']' to close sequence pattern")
	if err != nil {
		return nil, err
	}
	node := &MatchSequence{Patterns: patterns}
	node.Span = gast.Merge(open.Span, closeTok.Span)
	return node, nil
}

// parseMaybeStarPattern parses a pattern or a *name star pattern.
func (p *parser) parseMaybeStarPattern() (Pattern, error) {
	if starTok, ok := p.matchToken(Star); ok {
		name, err := p.Expect(KindName, "name after '*' in sequence pattern")
		if err != nil {
			return nil, err
		}
		if name.Text == wildcard {
			return nil, p.FailAt(name.Span, "cannot use '_' as a star target")
		}
		node := &MatchStar{Name: name.Text}
		node.Span = gast.Merge(starTok.Span, name.Span)
		return node, nil
	}
	return p.parseAsPattern()
}

// parseMappingPattern parses a {key: pattern, ..., **rest} pattern.
func (p *parser) parseMappingPattern() (Pattern, error) {
	start := p.Current().Span
	p.Advance()
	node := &MatchMapping{}
	for !p.Check(RBrace) && !p.AtEnd() {
		if p.Match(DoubleStar) {
			name, err := p.Expect(KindName, "name after '**' in mapping pattern")
			if err != nil {
				return nil, err
			}
			node.Rest = name.Text
			if !p.Match(Comma) {
				break
			}
			continue
		}
		key, err := p.parseMappingKey()
		if err != nil {
			return nil, err
		}
		if _, err := p.Expect(Colon, "':' in mapping pattern"); err != nil {
			return nil, err
		}
		pattern, err := p.parseAsPattern()
		if err != nil {
			return nil, err
		}
		node.Keys = append(node.Keys, key)
		node.Patterns = append(node.Patterns, pattern)
		if !p.Match(Comma) {
			break
		}
	}
	closeTok, err := p.Expect(RBrace, "'}' to close mapping pattern")
	if err != nil {
		return nil, err
	}
	node.Span = gast.Merge(start, closeTok.Span)
	return node, nil
}

// parseMappingKey parses a literal or dotted-value mapping key.
func (p *parser) parseMappingKey() (Expr, error) {
	switch p.Current().Kind {
	case KindName:
		if p.Peek(1).Kind == Dot {
			return p.parseDottedValueName()
		}
		return nil, p.Fail("mapping pattern keys must be literals or values")
	case KindInt, KindFloat, KindComplex, KindString, KindBytes, KindTrue, KindFalse, KindNone, Minus, Plus:
		return p.parseSignedLiteral()
	}
	return nil, p.Fail("mapping pattern keys must be literals or values")
}

// parseClassPattern parses Cls(positionals, keyword=pattern).
func (p *parser) parseClassPattern() (Pattern, error) {
	start := p.Current().Span
	cls, err := p.parseDottedValueName()
	if err != nil {
		return nil, err
	}
	p.Advance()
	node := &MatchClass{Cls: cls}
	for !p.Check(RParen) && !p.AtEnd() {
		if p.Check(KindName) && p.CheckAhead(1, Assign) {
			name := p.Advance()
			p.Advance()
			pattern, err := p.parseAsPattern()
			if err != nil {
				return nil, err
			}
			node.KwdNames = append(node.KwdNames, name.Text)
			node.KwdPatterns = append(node.KwdPatterns, pattern)
		} else {
			pattern, err := p.parseAsPattern()
			if err != nil {
				return nil, err
			}
			if len(node.KwdNames) > 0 {
				return nil, p.FailAt(pattern.Pos(), "positional pattern follows keyword pattern")
			}
			node.Patterns = append(node.Patterns, pattern)
		}
		if !p.Match(Comma) {
			break
		}
	}
	closeTok, err := p.Expect(RParen, "')' to close class pattern")
	if err != nil {
		return nil, err
	}
	node.Span = gast.Merge(start, closeTok.Span)
	return node, nil
}
