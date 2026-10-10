package sasscore

import (
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

// Parse tokenizes and parses source for the dialect and returns the
// root stylesheet node.
func Parse(source string, dialect Dialect) (*Stylesheet, error) {
	tokens, err := Tokenize(source, dialect)
	if err != nil {
		return nil, err
	}
	if len(tokens) > 0 && tokens[len(tokens)-1].Kind == KindEOF {
		tokens = tokens[:len(tokens)-1]
	}
	p := &parser{TokenCursor: gast.NewTokenCursor(tokens, KindEOF), dialect: dialect, source: source}
	return p.parseStylesheet()
}

type parser struct {
	*gast.TokenCursor
	dialect Dialect
	source  string
}

func (p *parser) parseStylesheet() (*Stylesheet, error) {
	nodes, err := p.parseItems(false)
	if err != nil {
		return nil, err
	}
	file := &Stylesheet{Nodes: nodes}
	file.Span = gast.Span{Start: 0, End: len(p.source)}
	return file, nil
}

// parseItems parses rules, declarations, and at-rules until the end of
// input or the enclosing block's terminator (} or dedent), which is
// left for the caller.
func (p *parser) parseItems(inBlock bool) ([]Node, error) {
	var nodes []Node
	for !p.AtEnd() {
		switch p.Current().Kind {
		case KindNewline, Semicolon:
			p.Advance()
			continue
		case KindDedent:
			if !inBlock {
				return nil, p.Fail("unexpected dedent")
			}
			return nodes, nil
		case KindIndent:
			return nil, p.Fail("unexpected indent")
		case RBrace:
			if inBlock && !p.dialect.Indented {
				return nodes, nil
			}
			return nil, p.Fail("unexpected '}'")
		case KindCDO, KindCDC:
			if inBlock || p.dialect.Indented {
				return nil, p.Fail("unexpected " + gast.Describe(p.Current()))
			}
			p.Advance()
			continue
		}
		if p.dialect.SCSS && p.Check(KindAtKeyword) && strings.EqualFold(p.Current().Text, AtElse) {
			if err := p.attachElse(nodes, inBlock); err != nil {
				return nil, err
			}
			continue
		}
		node, err := p.parseItem(inBlock, len(nodes) == 0)
		if err != nil {
			return nil, err
		}
		if node != nil {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

// AtElse is the @else keyword spelling.
const AtElse = At + "else"

func (p *parser) parseItem(inBlock, first bool) (Node, error) {
	if p.Check(KindAtKeyword) {
		return p.parseAtRule(inBlock, first)
	}
	if p.dialect.SCSS && p.Check(KindVariable) {
		return p.parseVariableDecl(inBlock)
	}
	custom := p.Check(KindIdent) && strings.HasPrefix(p.Current().Text, CustomPropStart)
	prelude, term, err := p.scanPrelude(inBlock, custom)
	if err != nil {
		return nil, err
	}
	switch {
	case term == LBrace:
		return p.parseQualifiedRule(prelude, p.parseBracedBlock)
	case p.dialect.Indented && term == KindNewline:
		p.Advance()
		if p.Check(KindIndent) {
			return p.parseQualifiedRule(prelude, p.parseIndentedBlock)
		}
		return p.parseDeclaration(prelude, !inBlock)
	case term == Semicolon:
		p.Advance()
		return p.parseDeclaration(prelude, !inBlock)
	case term == RBrace:
		return p.parseDeclaration(prelude, false)
	case term == KindDedent:
		return nil, nil
	case term == KindEOF:
		if !inBlock {
			return nil, p.Fail("unexpected end of input")
		}
		return p.parseDeclaration(prelude, false)
	}
	return nil, p.Fail("unexpected " + gast.Describe(p.Current()))
}

// scanPrelude collects the token stream of a selector or value up to
// the structural terminator, without consuming the terminator.
// Terminators are '{', ';', '}' (only inside a block), newline and
// dedent in the indented dialect, and end of input. Unbalanced closing
// tokens are errors. In raw custom-property mode braces are part of
// the value and only ';' or '}' at brace depth zero terminate.
func (p *parser) scanPrelude(inBlock, raw bool) ([]gast.Token, string, error) {
	var tokens []gast.Token
	depth := 0
	for !p.AtEnd() {
		kind := p.Current().Kind
		if depth == 0 {
			switch kind {
			case LBrace:
				if !raw {
					return tokens, kind, nil
				}
			case Semicolon:
				return tokens, kind, nil
			case RBrace:
				if inBlock {
					return tokens, kind, nil
				}
				return nil, "", p.Fail("unexpected '}'")
			case KindNewline, KindDedent:
				if p.dialect.Indented {
					// A trailing comma continues the prelude on the next line.
					if kind == KindNewline && len(tokens) > 0 && tokens[len(tokens)-1].Kind == Comma {
						p.Advance()
						for p.Match(KindIndent) {
						}
						continue
					}
					return tokens, kind, nil
				}
			}
		}
		switch kind {
		case LParen, LBracket, KindFunction, KindInterpOpen:
			depth++
		case LBrace:
			if raw {
				depth++
			}
		case RParen, RBracket, RBrace:
			depth--
			if depth < 0 {
				return nil, "", p.Fail("unexpected " + gast.Describe(p.Current()))
			}
		}
		tokens = append(tokens, p.Advance())
	}
	if depth > 0 {
		return nil, "", p.Fail("unclosed group in value")
	}
	return tokens, KindEOF, nil
}

func (p *parser) parseQualifiedRule(prelude []gast.Token, parseBlock func() (*Block, error)) (Node, error) {
	if len(prelude) == 0 {
		return nil, p.Fail("empty selector")
	}
	selectors, err := parseSelectorList(prelude)
	if err != nil {
		return nil, err
	}
	block, err := parseBlock()
	if err != nil {
		return nil, err
	}
	rule := &QualifiedRule{Selectors: selectors, Block: block}
	rule.Span = gast.Merge(selectors.Span, block.Span)
	return rule, nil
}

func (p *parser) parseDeclaration(prelude []gast.Token, top bool) (Node, error) {
	if len(prelude) == 0 {
		return nil, p.Fail("expected declaration")
	}
	if top {
		return nil, p.FailAt(gast.Merge(prelude[0].Span, prelude[len(prelude)-1].Span),
			"declarations are not allowed at the top level")
	}
	// The name is an identifier, optionally continued by interpolated
	// parts, or a bare interpolation.
	var name []gast.Token
	if prelude[0].Kind == KindIdent || prelude[0].Kind == KindInterpOpen {
		nameQ := gast.NewTokenCursor(prelude, KindEOF)
		if prelude[0].Kind == KindIdent {
			name = append(name, nameQ.Advance())
		}
	nameLoop:
		for {
			switch {
			case nameQ.Check(KindInterpOpen):
				name = append(name, collectInterpolation(nameQ)...)
			case nameQ.Check(KindIdent) &&
				nameQ.Peek(-1).Span.End == nameQ.Current().Span.Start:
				name = append(name, nameQ.Advance())
			default:
				break nameLoop
			}
		}
	}
	if len(name) == 0 {
		name = []gast.Token{prelude[0]}
	}
	rest := prelude[len(name):]
	custom := prelude[0].Kind == KindIdent && strings.HasPrefix(prelude[0].Text, CustomPropStart)
	if len(rest) == 0 || rest[0].Kind != Colon {
		return nil, p.FailAt(prelude[0].Span, "expected ':' after property name")
	}
	value, important := splitImportant(rest[1:])
	if len(value) == 0 && !custom {
		return nil, p.FailAt(prelude[0].Span, "expected property value")
	}
	decl := &Declaration{Name: name, Custom: custom, Value: value, Important: important}
	decl.Span = gast.Merge(prelude[0].Span, prelude[len(prelude)-1].Span)
	return decl, nil
}

func (p *parser) parseBracedBlock() (*Block, error) {
	brace := p.Advance() // {
	nodes, err := p.parseItems(true)
	if err != nil {
		return nil, err
	}
	if !p.Match(RBrace) {
		return nil, p.FailAt(brace.Span, "unclosed block")
	}
	block := &Block{Nodes: nodes}
	block.Span = p.SpanFrom(brace.Span)
	return block, nil
}

func (p *parser) parseIndentedBlock() (*Block, error) {
	indent := p.Advance() // indent
	nodes, err := p.parseItems(true)
	if err != nil {
		return nil, err
	}
	if !p.Match(KindDedent) && !p.AtEnd() {
		return nil, p.FailAt(indent.Span, "unclosed block")
	}
	block := &Block{Nodes: nodes}
	block.Span = p.SpanFrom(indent.Span)
	return block, nil
}

// parseBlockAfterPrelude parses the block that follows an at-rule,
// include, or else prelude, based on how scanPrelude stopped.
func (p *parser) parseBlockAfterPrelude(term string) (*Block, error) {
	switch term {
	case LBrace:
		return p.parseBracedBlock()
	case KindNewline:
		p.Advance()
		if p.Check(KindIndent) {
			return p.parseIndentedBlock()
		}
	}
	return nil, p.Fail("expected block")
}

// splitImportant strips a trailing !important from a value token
// stream and reports whether it was present.
func splitImportant(value []gast.Token) ([]gast.Token, bool) {
	if len(value) >= 2 && value[len(value)-1].Kind == KindIdent &&
		strings.EqualFold(value[len(value)-1].Text, "important") &&
		value[len(value)-2].Kind == Bang {
		return value[:len(value)-2], true
	}
	return value, false
}

// splitVarFlags strips trailing !default and !global flags from a
// variable value token stream.
func splitVarFlags(value []gast.Token) ([]gast.Token, bool, bool) {
	isDefault, isGlobal := false, false
	for len(value) >= 2 && value[len(value)-1].Kind == KindIdent &&
		value[len(value)-2].Kind == Bang {
		switch strings.ToLower(value[len(value)-1].Text) {
		case "default":
			isDefault = true
		case "global":
			isGlobal = true
		default:
			return value, isDefault, isGlobal
		}
		value = value[:len(value)-2]
	}
	return value, isDefault, isGlobal
}

func (p *parser) parseVariableDecl(inBlock bool) (Node, error) {
	name := p.Advance()
	if !p.Match(Colon) {
		return nil, p.Fail("expected ':' after variable name")
	}
	value, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	if err := p.endStatement(name, term, "$"+strings.TrimPrefix(name.Text, Dollar)); err != nil {
		return nil, err
	}
	value, isDefault, isGlobal := splitVarFlags(value)
	if len(value) == 0 {
		return nil, p.FailAt(name.Span, "expected variable value")
	}
	decl := &VariableDecl{
		Name:    strings.TrimPrefix(name.Text, Dollar),
		Value:   value,
		Default: isDefault,
		Global:  isGlobal,
	}
	decl.Span = p.SpanFrom(name.Span)
	return decl, nil
}
