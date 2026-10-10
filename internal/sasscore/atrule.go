package sasscore

import (
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

// parseAtRule dispatches an at-rule to dedicated structure for the
// Sass control rules and to generic block or statement form otherwise.
func (p *parser) parseAtRule(inBlock, first bool) (Node, error) {
	at := p.Advance()
	name := strings.ToLower(strings.TrimPrefix(at.Text, At))
	if p.dialect.SCSS {
		switch name {
		case "if":
			return p.parseIf(at, inBlock)
		case "else":
			return nil, p.FailAt(at.Span, "@else must follow @if")
		case "each":
			return p.parseEach(at, inBlock)
		case "for":
			return p.parseFor(at, inBlock)
		case "while":
			return p.parseWhile(at, inBlock)
		case "mixin":
			return p.parseMixin(at, inBlock)
		case "include":
			return p.parseInclude(at, inBlock)
		case "function":
			return p.parseFunction(at, inBlock)
		case "return":
			return p.parseReturn(at, inBlock)
		case "extend":
			return p.parseExtend(at, inBlock)
		case "use":
			return p.parseUse(at, inBlock)
		case "forward":
			return p.parseForward(at, inBlock)
		case "at-root":
			return p.parseAtRoot(at, inBlock)
		}
	}
	switch name {
	case "charset":
		return p.parseCharset(at, inBlock, first)
	case "import":
		return p.parseImport(at, inBlock)
	}
	return p.parseGenericAtRule(at, name, inBlock)
}

// parseGenericAtRule parses known and unknown at-rules in either their
// block form or their semicolon statement form.
func (p *parser) parseGenericAtRule(at gast.Token, name string, inBlock bool) (Node, error) {
	prelude, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	if len(prelude) == 0 && preludeAtRules[name] {
		return nil, p.FailAt(at.Span, "expected prelude after @"+name)
	}
	rule := &AtRule{Name: name, Prelude: prelude}
	switch term {
	case LBrace:
		if keyframesAtRules[name] {
			rule.Block, err = p.parseKeyframesBody()
		} else {
			rule.Block, err = p.parseBracedBlock()
		}
		if err != nil {
			return nil, err
		}
	case Semicolon:
		p.Advance()
		if blockAtRules[name] {
			return nil, p.FailAt(at.Span, "expected block after @"+name)
		}
	case KindNewline:
		p.Advance()
		if p.Check(KindIndent) {
			if keyframesAtRules[name] {
				rule.Block, err = p.parseIndentedKeyframesBody()
			} else {
				rule.Block, err = p.parseIndentedBlock()
			}
			if err != nil {
				return nil, err
			}
		} else if blockAtRules[name] {
			return nil, p.FailAt(at.Span, "expected block after @"+name)
		}
	case KindDedent:
		if blockAtRules[name] {
			return nil, p.FailAt(at.Span, "expected block after @"+name)
		}
	default:
		return nil, p.FailAt(at.Span, "unclosed at-rule")
	}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

// parseKeyframesBody parses the braced body of a @keyframes rule.
func (p *parser) parseKeyframesBody() (*Block, error) {
	brace := p.Advance() // {
	nodes, err := p.parseKeyframeItems()
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

// parseIndentedKeyframesBody parses the indented body of a @keyframes
// rule; the newline has been consumed and an indent token is current.
func (p *parser) parseIndentedKeyframesBody() (*Block, error) {
	indent := p.Advance()
	nodes, err := p.parseKeyframeItems()
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

// parseKeyframeItems parses `from, 50% { ... }` rules until the block
// terminator.
func (p *parser) parseKeyframeItems() ([]Node, error) {
	var nodes []Node
	for !p.AtEnd() {
		switch p.Current().Kind {
		case KindNewline, Semicolon:
			p.Advance()
			continue
		case KindDedent, RBrace:
			return nodes, nil
		}
		prelude, term, err := p.scanPrelude(true, false)
		if err != nil {
			return nil, err
		}
		if len(prelude) == 0 && term == KindDedent {
			return nodes, nil
		}
		if err := validateKeyframeSelectors(prelude); err != nil {
			return nil, err
		}
		rule := &KeyframeRule{Selectors: prelude}
		switch term {
		case LBrace:
			rule.Block, err = p.parseBracedBlock()
		case KindNewline:
			p.Advance()
			if p.Check(KindIndent) {
				rule.Block, err = p.parseIndentedBlock()
			} else {
				return nil, p.Fail("expected block after keyframe selector")
			}
		default:
			return nil, p.Fail("expected block after keyframe selector")
		}
		if err != nil {
			return nil, err
		}
		rule.Span = gast.Merge(prelude[0].Span, rule.Block.Span)
		nodes = append(nodes, rule)
	}
	return nodes, nil
}

// validateKeyframeSelectors checks that a keyframe prelude is a
// comma-separated list of from, to, or percentage selectors;
// interpolated selectors pass through as raw token groups.
func validateKeyframeSelectors(prelude []gast.Token) error {
	if len(prelude) == 0 {
		return &gast.ParseError{Message: "expected keyframe selector"}
	}
	interelem := false
	for i, tok := range prelude {
		if tok.Kind == Comma {
			if i == 0 || i == len(prelude)-1 || prelude[i-1].Kind == Comma {
				return &gast.ParseError{Message: "expected keyframe selector", Pos: tok.Span}
			}
			interelem = false
			continue
		}
		switch tok.Kind {
		case KindInterpOpen, KindVariable, RBrace, Percent:
			interelem = true
		case KindIdent:
			if !interelem && !strings.EqualFold(tok.Text, "from") && !strings.EqualFold(tok.Text, "to") {
				return &gast.ParseError{Message: "invalid keyframe selector", Pos: tok.Span}
			}
		case KindPercentage, KindNumber:
		default:
			return &gast.ParseError{Message: "invalid keyframe selector", Pos: tok.Span}
		}
	}
	return nil
}

func (p *parser) parseCharset(at gast.Token, inBlock, first bool) (Node, error) {
	if inBlock || !first {
		return nil, p.FailAt(at.Span, "@charset must be the first rule in the stylesheet")
	}
	encoding, err := p.Expect(KindString, "encoding string")
	if err != nil {
		return nil, err
	}
	if !p.Match(Semicolon) {
		return nil, p.Fail("expected ';' after @charset")
	}
	rule := &AtRule{Name: "charset", Prelude: []gast.Token{encoding}}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

func (p *parser) parseImport(at gast.Token, inBlock bool) (Node, error) {
	prelude, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	if len(prelude) == 0 {
		return nil, p.FailAt(at.Span, "expected URL after @import")
	}
	switch term {
	case Semicolon:
		p.Advance()
	case KindNewline, KindDedent:
	default:
		return nil, p.FailAt(at.Span, "expected ';' after @import")
	}
	rule := &AtRule{Name: "import", Prelude: prelude}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

func (p *parser) parseIf(at gast.Token, inBlock bool) (Node, error) {
	cond, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	if len(cond) == 0 {
		return nil, p.FailAt(at.Span, "expected condition after @if")
	}
	block, err := p.parseBlockAfterPrelude(term)
	if err != nil {
		return nil, err
	}
	rule := &IfRule{Cond: cond, Then: block}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

// attachElse folds an @else or @else if clause into the preceding @if
// rule at the end of nodes.
func (p *parser) attachElse(nodes []Node, inBlock bool) error {
	at := p.Advance()
	if len(nodes) == 0 {
		return p.FailAt(at.Span, "@else must follow @if")
	}
	ifRule, ok := nodes[len(nodes)-1].(*IfRule)
	if !ok || ifRule.Else != nil {
		return p.FailAt(at.Span, "@else must follow @if")
	}
	prelude, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return err
	}
	block, err := p.parseBlockAfterPrelude(term)
	if err != nil {
		return err
	}
	if len(prelude) > 0 {
		if prelude[0].Kind != KindIdent || !strings.EqualFold(prelude[0].Text, "if") || len(prelude) == 1 {
			return p.FailAt(at.Span, "expected 'if' or block after @else")
		}
		clause := ElseIf{
			Span:  gast.Merge(prelude[0].Span, block.Span),
			Cond:  prelude[1:],
			Block: block,
		}
		ifRule.ElseIfs = append(ifRule.ElseIfs, clause)
	} else {
		ifRule.Else = block
	}
	ifRule.Span = gast.Merge(ifRule.Span, block.Span)
	return nil
}

func (p *parser) parseEach(at gast.Token, inBlock bool) (Node, error) {
	prelude, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	q := gast.NewTokenCursor(prelude, KindEOF)
	rule := &EachRule{}
	first := q.Advance()
	for {
		if first.Kind != KindVariable {
			return nil, p.FailAt(first.Span, "expected variable in @each")
		}
		rule.Vars = append(rule.Vars, strings.TrimPrefix(first.Text, Dollar))
		if q.Check(Dot) && q.CheckAhead(1, Dot) && q.CheckAhead(2, Dot) {
			q.Advance()
			q.Advance()
			q.Advance()
			rule.Rest = true
		}
		if !q.Match(Comma) {
			break
		}
		first = q.Advance()
	}
	inTok := q.Advance()
	if inTok.Kind != KindIdent || !strings.EqualFold(inTok.Text, "in") {
		return nil, p.FailAt(inTok.Span, "expected 'in' in @each")
	}
	for !q.AtEnd() {
		rule.Expr = append(rule.Expr, q.Advance())
	}
	if len(rule.Expr) == 0 {
		return nil, p.FailAt(at.Span, "expected expression after 'in' in @each")
	}
	rule.Block, err = p.parseBlockAfterPrelude(term)
	if err != nil {
		return nil, err
	}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

func (p *parser) parseFor(at gast.Token, inBlock bool) (Node, error) {
	prelude, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	q := gast.NewTokenCursor(prelude, KindEOF)
	rule := &ForRule{}
	name := q.Advance()
	if name.Kind != KindVariable {
		return nil, p.FailAt(name.Span, "expected variable in @for")
	}
	rule.Var = strings.TrimPrefix(name.Text, Dollar)
	from := q.Advance()
	if from.Kind != KindIdent || !strings.EqualFold(from.Text, "from") {
		return nil, p.FailAt(from.Span, "expected 'from' in @for")
	}
	depth := 0
	for !q.AtEnd() {
		tok := q.Current()
		if depth == 0 && tok.Kind == KindIdent &&
			(strings.EqualFold(tok.Text, "to") || strings.EqualFold(tok.Text, "through")) {
			rule.Inclusive = strings.EqualFold(tok.Text, "through")
			break
		}
		switch tok.Kind {
		case LParen, LBracket, KindFunction, KindInterpOpen:
			depth++
		case RParen, RBracket, RBrace:
			depth--
		}
		rule.From = append(rule.From, q.Advance())
	}
	keyword := q.Advance()
	if keyword.Kind != KindIdent {
		return nil, p.FailAt(at.Span, "expected 'to' or 'through' in @for")
	}
	for !q.AtEnd() {
		rule.To = append(rule.To, q.Advance())
	}
	if len(rule.From) == 0 || len(rule.To) == 0 {
		return nil, p.FailAt(at.Span, "expected bounds in @for")
	}
	rule.Block, err = p.parseBlockAfterPrelude(term)
	if err != nil {
		return nil, err
	}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

func (p *parser) parseWhile(at gast.Token, inBlock bool) (Node, error) {
	cond, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	if len(cond) == 0 {
		return nil, p.FailAt(at.Span, "expected condition after @while")
	}
	block, err := p.parseBlockAfterPrelude(term)
	if err != nil {
		return nil, err
	}
	rule := &WhileRule{Cond: cond, Block: block}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

// parseMixinPrelude splits an @mixin or @function prelude into its
// name and parameter tokens. The name token is a function token when
// the parameter list follows the name without whitespace.
func (p *parser) parseMixinPrelude(prelude []gast.Token, what string) (string, []gast.Token, error) {
	q := gast.NewTokenCursor(prelude, KindEOF)
	name := q.Advance()
	switch name.Kind {
	case KindIdent:
		if !q.Check(LParen) {
			return name.Text, nil, nil
		}
		var params []gast.Token
		for !q.AtEnd() {
			params = append(params, q.Advance())
		}
		return name.Text, params, nil
	case KindFunction:
		params := []gast.Token{{
			Kind: LParen,
			Text: LParen,
			Span: gast.Span{Start: name.Span.End - 1, End: name.Span.End},
		}}
		for !q.AtEnd() {
			params = append(params, q.Advance())
		}
		return strings.TrimSuffix(name.Text, LParen), params, nil
	}
	return "", nil, p.FailAt(name.Span, "expected "+what+" name")
}

// parseParams parses a parenthesized parameter list, without the outer
// parentheses.
func (p *parser) parseParams(tokens []gast.Token) ([]Param, error) {
	if len(tokens) < 2 || tokens[0].Kind != LParen || tokens[len(tokens)-1].Kind != RParen {
		return nil, p.Fail("expected parameter list")
	}
	var params []Param
	for _, part := range splitTopLevel(tokens[1:len(tokens)-1], Comma) {
		if len(part) == 0 {
			return nil, p.Fail("expected parameter")
		}
		q := gast.NewTokenCursor(part, KindEOF)
		param := Param{}
		name := q.Advance()
		if name.Kind != KindVariable {
			return nil, p.FailAt(name.Span, "expected parameter name")
		}
		param.Name = strings.TrimPrefix(name.Text, Dollar)
		if q.Check(Dot) && q.CheckAhead(1, Dot) && q.CheckAhead(2, Dot) {
			q.Advance()
			q.Advance()
			q.Advance()
			param.Rest = true
		}
		if q.Match(Colon) {
			for !q.AtEnd() {
				param.Default = append(param.Default, q.Advance())
			}
		}
		if !q.AtEnd() {
			return nil, p.FailAt(q.Current().Span, "unexpected token in parameter")
		}
		param.Span = gast.Merge(part[0].Span, part[len(part)-1].Span)
		params = append(params, param)
	}
	return params, nil
}

func (p *parser) parseMixin(at gast.Token, inBlock bool) (Node, error) {
	prelude, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	name, paramTokens, err := p.parseMixinPrelude(prelude, "mixin")
	if err != nil {
		return nil, err
	}
	rule := &MixinRule{Name: name}
	if paramTokens != nil {
		rule.Params, err = p.parseParams(paramTokens)
		if err != nil {
			return nil, err
		}
	}
	rule.Block, err = p.parseBlockAfterPrelude(term)
	if err != nil {
		return nil, err
	}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

func (p *parser) parseFunction(at gast.Token, inBlock bool) (Node, error) {
	prelude, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	name, paramTokens, err := p.parseMixinPrelude(prelude, "function")
	if err != nil {
		return nil, err
	}
	rule := &FunctionRule{Name: name}
	if paramTokens != nil {
		rule.Params, err = p.parseParams(paramTokens)
		if err != nil {
			return nil, err
		}
	}
	rule.Block, err = p.parseBlockAfterPrelude(term)
	if err != nil {
		return nil, err
	}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

func (p *parser) parseInclude(at gast.Token, inBlock bool) (Node, error) {
	prelude, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	q := gast.NewTokenCursor(prelude, KindEOF)
	first := q.Advance()
	rule := &IncludeRule{}
	switch first.Kind {
	case KindIdent:
		rule.Name = first.Text
	case KindFunction:
		rule.Name = strings.TrimSuffix(first.Text, LParen)
		rule.Args = append(rule.Args, gast.Token{
			Kind: LParen,
			Text: LParen,
			Span: gast.Span{Start: first.Span.End - 1, End: first.Span.End},
		})
	default:
		return nil, p.FailAt(first.Span, "expected mixin name")
	}
	for q.Check(Dot) && q.CheckAhead(1, KindIdent) {
		q.Advance()
		rule.Name += Dot + q.Advance().Text
	}
	for !q.AtEnd() {
		rule.Args = append(rule.Args, q.Advance())
	}
	if len(rule.Args) > 0 && (rule.Args[0].Kind != LParen || rule.Args[len(rule.Args)-1].Kind != RParen) {
		return nil, p.FailAt(rule.Args[0].Span, "expected argument list")
	}
	switch term {
	case LBrace:
		rule.Block, err = p.parseBracedBlock()
	case KindNewline:
		p.Advance()
		if p.Check(KindIndent) {
			rule.Block, err = p.parseIndentedBlock()
		}
	case Semicolon:
		p.Advance()
	case KindDedent, RBrace:
	default:
		return nil, p.FailAt(at.Span, "expected ';' after @include")
	}
	if err != nil {
		return nil, err
	}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

// endStatement consumes the statement terminator of a prelude that has
// already been scanned: a semicolon is consumed, newline, dedent, and
// an omitted semicolon before a closing brace are left for the caller,
// and anything else is an error.
func (p *parser) endStatement(at gast.Token, term, what string) error {
	switch term {
	case Semicolon:
		p.Advance()
	case KindNewline, KindDedent, RBrace:
	default:
		return p.FailAt(at.Span, "expected ';' after "+what)
	}
	return nil
}

func (p *parser) parseReturn(at gast.Token, inBlock bool) (Node, error) {
	value, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	if len(value) == 0 {
		return nil, p.FailAt(at.Span, "expected value after @return")
	}
	if err := p.endStatement(at, term, "@return"); err != nil {
		return nil, err
	}
	rule := &ReturnRule{Value: value}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

func (p *parser) parseExtend(at gast.Token, inBlock bool) (Node, error) {
	prelude, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	if len(prelude) == 0 {
		return nil, p.FailAt(at.Span, "expected selector after @extend")
	}
	if err := p.endStatement(at, term, "@extend"); err != nil {
		return nil, err
	}
	rule := &ExtendRule{Selector: prelude}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

func (p *parser) parseAtRoot(at gast.Token, inBlock bool) (Node, error) {
	prelude, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	rule := &AtRootRule{Query: prelude}
	switch term {
	case LBrace:
		rule.Block, err = p.parseBracedBlock()
	case KindNewline:
		p.Advance()
		if p.Check(KindIndent) {
			rule.Block, err = p.parseIndentedBlock()
		}
	case Semicolon:
		p.Advance()
	}
	if err != nil {
		return nil, err
	}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

func (p *parser) parseUse(at gast.Token, inBlock bool) (Node, error) {
	prelude, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	if err := p.endStatement(at, term, "@use"); err != nil {
		return nil, err
	}
	q := gast.NewTokenCursor(prelude, KindEOF)
	url := q.Advance()
	if url.Kind != KindString {
		return nil, p.FailAt(url.Span, "expected URL string after @use")
	}
	rule := &UseRule{URL: url}
	if keyword(q, "as") {
		if q.Match(Asterisk) {
			rule.Namespace = Asterisk
		} else {
			ns, err := q.Expect(KindIdent, "namespace")
			if err != nil {
				return nil, err
			}
			rule.Namespace = ns.Text
		}
	}
	if keyword(q, "with") {
		rule.Config, err = p.parseConfig(q)
		if err != nil {
			return nil, err
		}
	}
	if !q.AtEnd() {
		return nil, p.FailAt(q.Current().Span, "unexpected token in @use")
	}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

func (p *parser) parseForward(at gast.Token, inBlock bool) (Node, error) {
	prelude, term, err := p.scanPrelude(inBlock, false)
	if err != nil {
		return nil, err
	}
	if err := p.endStatement(at, term, "@forward"); err != nil {
		return nil, err
	}
	q := gast.NewTokenCursor(prelude, KindEOF)
	url := q.Advance()
	if url.Kind != KindString {
		return nil, p.FailAt(url.Span, "expected URL string after @forward")
	}
	rule := &ForwardRule{URL: url}
	if keyword(q, "as") {
		var prefix strings.Builder
		for !q.AtEnd() && !keywordPeek(q, "show") && !keywordPeek(q, "hide") && !keywordPeek(q, "with") {
			prefix.WriteString(q.Advance().Text)
		}
		rule.Prefix = prefix.String()
	}
	if keyword(q, "show") {
		rule.Show, err = parseMemberList(q)
		if err != nil {
			return nil, err
		}
	}
	if keyword(q, "hide") {
		rule.Hide, err = parseMemberList(q)
		if err != nil {
			return nil, err
		}
	}
	if keyword(q, "with") {
		rule.Config, err = p.parseConfig(q)
		if err != nil {
			return nil, err
		}
	}
	if !q.AtEnd() {
		return nil, p.FailAt(q.Current().Span, "unexpected token in @forward")
	}
	rule.Span = p.SpanFrom(at.Span)
	return rule, nil
}

// keyword consumes the current identifier when it matches word
// case-insensitively.
func keyword(q *gast.TokenCursor, word string) bool {
	if q.Check(KindIdent) && strings.EqualFold(q.Current().Text, word) {
		q.Advance()
		return true
	}
	return false
}

func keywordPeek(q *gast.TokenCursor, word string) bool {
	return q.Check(KindIdent) && strings.EqualFold(q.Current().Text, word)
}

// parseMemberList parses a comma-separated member list of a @forward
// show or hide clause; members are identifiers or variables.
func parseMemberList(q *gast.TokenCursor) ([]string, error) {
	var members []string
	for {
		member := q.Advance()
		if member.Kind != KindIdent && member.Kind != KindVariable {
			return nil, q.FailAt(member.Span, "expected member name")
		}
		members = append(members, member.Text)
		if !q.Match(Comma) {
			return members, nil
		}
	}
}

// parseConfig parses the parenthesized `$name: value` configuration of
// a @use or @forward with clause.
func (p *parser) parseConfig(q *gast.TokenCursor) ([]ConfigPair, error) {
	if !q.Match(LParen) {
		return nil, q.Fail("expected '(' after 'with'")
	}
	var tokens []gast.Token
	depth := 1
	for !q.AtEnd() {
		tok := q.Advance()
		switch tok.Kind {
		case LParen, LBracket, KindFunction, KindInterpOpen:
			depth++
		case RParen, RBracket, RBrace:
			depth--
			if depth == 0 {
				tokens = append(tokens, tok)
				return p.parseConfigPairs(tokens[:len(tokens)-1])
			}
		}
		tokens = append(tokens, tok)
	}
	return nil, q.Fail("expected ')' after configuration")
}

func (p *parser) parseConfigPairs(tokens []gast.Token) ([]ConfigPair, error) {
	var pairs []ConfigPair
	for _, part := range splitTopLevel(tokens, Comma) {
		if len(part) < 3 || part[0].Kind != KindVariable || part[1].Kind != Colon {
			return nil, p.FailAt(part[0].Span, "expected $name: value configuration")
		}
		pair := ConfigPair{
			Span:  gast.Merge(part[0].Span, part[len(part)-1].Span),
			Name:  strings.TrimPrefix(part[0].Text, Dollar),
			Value: part[2:],
		}
		pairs = append(pairs, pair)
	}
	return pairs, nil
}

// splitTopLevel splits tokens on a delimiter that occurs at zero group
// depth; empty parts are dropped.
func splitTopLevel(tokens []gast.Token, delim string) [][]gast.Token {
	var parts [][]gast.Token
	start, depth := 0, 0
	for i, tok := range tokens {
		switch tok.Kind {
		case LParen, LBracket, KindFunction, KindInterpOpen:
			depth++
		case RParen, RBracket, RBrace:
			depth--
		}
		if tok.Kind == delim && depth == 0 {
			if part := tokens[start:i]; len(part) > 0 {
				parts = append(parts, part)
			}
			start = i + 1
		}
	}
	if part := tokens[start:]; len(part) > 0 {
		parts = append(parts, part)
	}
	return parts
}
