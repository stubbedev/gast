package python

import (
	"slices"

	"github.com/stubbedev/gast/pkg/gast"
)

// binaryLevels lists Python's left-associative binary operator
// precedence levels from loosest to tightest.
var binaryLevels = [][]string{
	{Pipe},
	{Caret},
	{Ampersand},
	{LeftShift, RightShift},
	{Plus, Minus},
	{Star, At, Slash, DoubleSlash, Percent},
}

// augmentedOps are the operators that may appear in an augmented
// assignment.
var augmentedOps = []string{
	AddAssign, SubAssign, MulAssign, DivAssign, FloorDivAssign, ModAssign,
	MatMulAssign, BitAndAssign, BitOrAssign, BitXorAssign, LeftShiftAssign,
	RightShiftAssign, PowAssign,
}

// matchToken consumes the current token when it has kind, returning
// the token so callers can use its span.
func (p *parser) matchToken(kind string) (gast.Token, bool) {
	if !p.Check(kind) {
		return gast.Token{}, false
	}
	return p.Advance(), true
}

// parseExprList parses a comma-separated expression list, allowing
// starred elements, and folds it into a tuple when a comma appears.
func (p *parser) parseExprList() (Expr, error) {
	if p.Check(KindYield) {
		return p.parseYield()
	}
	start := p.Current().Span
	first, err := p.parseStarElt()
	if err != nil {
		return nil, err
	}
	if !p.Check(Comma) {
		return first, nil
	}
	elts := []Expr{first}
	for p.Match(Comma) {
		if p.atExprListEnd() {
			break
		}
		elt, err := p.parseStarElt()
		if err != nil {
			return nil, err
		}
		elts = append(elts, elt)
	}
	tuple := &TupleExpr{Elts: elts}
	tuple.Span = p.SpanFrom(start)
	return tuple, nil
}

func (p *parser) atExprListEnd() bool {
	switch p.Current().Kind {
	case KindNewline, Semicolon, RParen, RBracket, RBrace, Assign, Colon, KindEOF:
		return true
	}
	return false
}

// parseYield parses yield and yield from at expression-list position.
func (p *parser) parseYield() (Expr, error) {
	start := p.Current().Span
	p.Advance()
	if p.Match(KindFrom) {
		value, err := p.parseTest()
		if err != nil {
			return nil, err
		}
		node := &YieldFromExpr{Value: value}
		node.Span = p.SpanFrom(start)
		return node, nil
	}
	var value Expr
	if !p.atExprListEnd() {
		var err error
		if value, err = p.parseExprList(); err != nil {
			return nil, err
		}
	}
	node := &YieldExpr{Value: value}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// parseStarElt parses one expression-list element: a starred
// expression or a possibly-walrused expression.
func (p *parser) parseStarElt() (Expr, error) {
	if starTok, ok := p.matchToken(Star); ok {
		value, err := p.parseBinary(0)
		if err != nil {
			return nil, err
		}
		node := &StarredExpr{Value: value}
		node.Span = p.SpanFrom(starTok.Span)
		return node, nil
	}
	return p.parseNamedExpr()
}

// parseNamedExpr parses a test with an optional walrus suffix, which
// Python allows only at specific element positions.
func (p *parser) parseNamedExpr() (Expr, error) {
	expr, err := p.parseTest()
	if err != nil {
		return nil, err
	}
	if !p.Match(Walrus) {
		return expr, nil
	}
	name, ok := expr.(*NameExpr)
	if !ok {
		return nil, p.FailAt(expr.Pos(), "assignment expression target must be a name")
	}
	value, err := p.parseTest()
	if err != nil {
		return nil, err
	}
	node := &NamedExpr{Target: name, Value: value}
	node.Span = gast.Merge(name.Span, value.Pos())
	return node, nil
}

// parseTest parses a lambda or a conditional expression.
func (p *parser) parseTest() (Expr, error) {
	if p.Check(KindLambda) {
		return p.parseLambda()
	}
	expr, err := p.parseOrTest()
	if err != nil {
		return nil, err
	}
	if _, ok := p.matchToken(KindIf); !ok {
		return expr, nil
	}
	test, err := p.parseOrTest()
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(KindElse, "'else' in conditional expression"); err != nil {
		return nil, err
	}
	orelse, err := p.parseTest()
	if err != nil {
		return nil, err
	}
	node := &IfExpExpr{Test: test, Body: expr, Orelse: orelse}
	node.Span = gast.Merge(expr.Pos(), orelse.Pos())
	return node, nil
}

func (p *parser) parseLambda() (Expr, error) {
	start := p.Current().Span
	p.Advance()
	node := &LambdaExpr{}
	var err error
	if node.Params, err = p.parseParameters(false, Colon); err != nil {
		return nil, err
	}
	if _, err := p.Expect(Colon, "':' after lambda parameters"); err != nil {
		return nil, err
	}
	if node.Body, err = p.parseTest(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

func (p *parser) parseOrTest() (Expr, error) {
	first, err := p.parseAndTest()
	if err != nil {
		return nil, err
	}
	if !p.Check(KindOr) {
		return first, nil
	}
	values := []Expr{first}
	for p.Match(KindOr) {
		value, err := p.parseAndTest()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	node := &BoolOpExpr{Op: KindOr, Values: values}
	node.Span = gast.Merge(first.Pos(), values[len(values)-1].Pos())
	return node, nil
}

func (p *parser) parseAndTest() (Expr, error) {
	first, err := p.parseNotTest()
	if err != nil {
		return nil, err
	}
	if !p.Check(KindAnd) {
		return first, nil
	}
	values := []Expr{first}
	for p.Match(KindAnd) {
		value, err := p.parseNotTest()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	node := &BoolOpExpr{Op: KindAnd, Values: values}
	node.Span = gast.Merge(first.Pos(), values[len(values)-1].Pos())
	return node, nil
}

func (p *parser) parseNotTest() (Expr, error) {
	if p.Check(KindNot) {
		start := p.Advance().Span
		operand, err := p.parseNotTest()
		if err != nil {
			return nil, err
		}
		node := &UnaryOpExpr{Op: KindNot, Operand: operand}
		node.Span = p.SpanFrom(start)
		return node, nil
	}
	return p.parseComparison()
}

// parseComparison parses a comparison chain into a single node with
// one op and comparator per link: a < b <= c is one CompareExpr.
func (p *parser) parseComparison() (Expr, error) {
	left, err := p.parseBinary(0)
	if err != nil {
		return nil, err
	}
	var ops []string
	var comparators []Expr
	for {
		op, ok := p.comparisonOp()
		if !ok {
			break
		}
		right, err := p.parseBinary(0)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
		comparators = append(comparators, right)
	}
	if len(ops) == 0 {
		return left, nil
	}
	node := &CompareExpr{Left: left, Ops: ops, Comparators: comparators}
	node.Span = gast.Merge(left.Pos(), comparators[len(comparators)-1].Pos())
	return node, nil
}

func (p *parser) comparisonOp() (string, bool) {
	switch p.Current().Kind {
	case Less, Greater, LessEqual, GreaterEqual, EqualEqual, NotEqual, KindIn:
		return p.Advance().Kind, true
	case KindIs:
		p.Advance()
		if p.Match(KindNot) {
			return opIsNot, true
		}
		return opIs, true
	case KindNot:
		if p.CheckAhead(1, KindIn) {
			p.Advance()
			p.Advance()
			return opNotIn, true
		}
	}
	return "", false
}

func (p *parser) parseBinary(level int) (Expr, error) {
	if level == len(binaryLevels) {
		return p.parseUnary()
	}
	left, err := p.parseBinary(level + 1)
	if err != nil {
		return nil, err
	}
	for slices.Contains(binaryLevels[level], p.Current().Kind) {
		op := p.Advance()
		right, err := p.parseBinary(level + 1)
		if err != nil {
			return nil, err
		}
		node := &BinOpExpr{Op: op.Kind, Left: left, Right: right}
		node.Span = gast.Merge(left.Pos(), right.Pos())
		left = node
	}
	return left, nil
}

// parseUnary parses prefix unary operators, which bind looser than **
// binds on its right: -2**2 is -(2**2).
func (p *parser) parseUnary() (Expr, error) {
	switch p.Current().Kind {
	case Plus, Minus, Tilde:
		tok := p.Advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		node := &UnaryOpExpr{Op: tok.Kind, Operand: operand}
		node.Span = p.SpanFrom(tok.Span)
		return node, nil
	}
	return p.parsePower()
}

func (p *parser) parsePower() (Expr, error) {
	base, err := p.parseAwaitPrimary()
	if err != nil {
		return nil, err
	}
	if _, ok := p.matchToken(DoubleStar); !ok {
		return base, nil
	}
	// The exponent is a factor, so ** is right-associative and admits
	// a unary exponent: 2**-1.
	exponent, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	node := &BinOpExpr{Op: DoubleStar, Left: base, Right: exponent}
	node.Span = gast.Merge(base.Pos(), exponent.Pos())
	return node, nil
}

func (p *parser) parseAwaitPrimary() (Expr, error) {
	if p.Check(KindAwait) {
		tok := p.Advance()
		value, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		node := &AwaitExpr{Value: value}
		node.Span = gast.Merge(tok.Span, value.Pos())
		return node, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Expr, error) {
	expr, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	for {
		switch p.Current().Kind {
		case Dot:
			p.Advance()
			name, err := p.Expect(KindName, "attribute name")
			if err != nil {
				return nil, err
			}
			node := &AttributeExpr{Value: expr, Attr: name.Text}
			node.Span = gast.Merge(expr.Pos(), name.Span)
			expr = node
		case LParen:
			p.Advance()
			args, closeTok, err := p.parseCallArgs()
			if err != nil {
				return nil, err
			}
			node := &CallExpr{Func: expr, Args: args}
			node.Span = gast.Merge(expr.Pos(), closeTok.Span)
			expr = node
		case LBracket:
			p.Advance()
			index, closeTok, err := p.parseSubscriptContents()
			if err != nil {
				return nil, err
			}
			node := &SubscriptExpr{Value: expr, Index: index}
			node.Span = gast.Merge(expr.Pos(), closeTok.Span)
			expr = node
		default:
			return expr, nil
		}
	}
}

func (p *parser) parseAtom() (Expr, error) {
	tok := p.Current()
	switch tok.Kind {
	case KindInt:
		p.Advance()
		return &IntLit{Span: tok.Span, Text: tok.Text}, nil
	case KindFloat:
		p.Advance()
		return &FloatLit{Span: tok.Span, Text: tok.Text}, nil
	case KindComplex:
		p.Advance()
		return &ComplexLit{Span: tok.Span, Text: tok.Text}, nil
	case KindString, KindBytes, KindFStringStart:
		return p.parseStringChain()
	case KindTrue, KindFalse:
		p.Advance()
		return &BoolLit{Span: tok.Span, Value: tok.Kind == KindTrue}, nil
	case KindNone:
		p.Advance()
		return &NoneLit{Span: tok.Span}, nil
	case Ellipsis:
		p.Advance()
		return &EllipsisLit{Span: tok.Span}, nil
	case KindName:
		p.Advance()
		return &NameExpr{Span: tok.Span, ID: tok.Text}, nil
	case LParen:
		return p.parseParenAtom()
	case LBracket:
		return p.parseListAtom()
	case LBrace:
		return p.parseDictSetAtom()
	}
	return nil, p.Fail("expected an expression, found " + gast.Describe(tok))
}

// isCompStart reports whether a comprehension's `for` (or `async
// for`) follows.
func (p *parser) isCompStart() bool {
	return p.Check(KindFor) || (p.Check(KindAsync) && p.CheckAhead(1, KindFor))
}

func (p *parser) parseParenAtom() (Expr, error) {
	open := p.Advance()
	if closeTok, ok := p.matchToken(RParen); ok {
		node := &TupleExpr{Parenthesized: true}
		node.Span = gast.Merge(open.Span, closeTok.Span)
		return node, nil
	}
	if p.Check(KindYield) {
		expr, err := p.parseYield()
		if err != nil {
			return nil, err
		}
		closeTok, err := p.Expect(RParen, "')'")
		if err != nil {
			return nil, err
		}
		switch node := expr.(type) {
		case *YieldExpr:
			node.Span = gast.Merge(open.Span, closeTok.Span)
		case *YieldFromExpr:
			node.Span = gast.Merge(open.Span, closeTok.Span)
		}
		return expr, nil
	}
	first, err := p.parseStarElt()
	if err != nil {
		return nil, err
	}
	if p.isCompStart() {
		return p.finishComprehension("generator", open.Span, first)
	}
	if p.Match(Comma) {
		return p.finishTuple(open.Span, first)
	}
	if _, err := p.Expect(RParen, "')'"); err != nil {
		return nil, err
	}
	markParenthesizedStar(first)
	return first, nil
}

func (p *parser) parseListAtom() (Expr, error) {
	open := p.Advance()
	if closeTok, ok := p.matchToken(RBracket); ok {
		node := &ListExpr{}
		node.Span = gast.Merge(open.Span, closeTok.Span)
		return node, nil
	}
	first, err := p.parseStarElt()
	if err != nil {
		return nil, err
	}
	if p.isCompStart() {
		return p.finishComprehension("list", open.Span, first)
	}
	elts := []Expr{first}
	for p.Match(Comma) {
		if p.Check(RBracket) {
			break
		}
		elt, err := p.parseStarElt()
		if err != nil {
			return nil, err
		}
		elts = append(elts, elt)
	}
	closeTok, err := p.Expect(RBracket, "']'")
	if err != nil {
		return nil, err
	}
	node := &ListExpr{Elts: elts}
	node.Span = gast.Merge(open.Span, closeTok.Span)
	return node, nil
}

func (p *parser) parseDictSetAtom() (Expr, error) {
	open := p.Advance()
	if closeTok, ok := p.matchToken(RBrace); ok {
		node := &DictExpr{}
		node.Span = gast.Merge(open.Span, closeTok.Span)
		return node, nil
	}
	if p.Check(DoubleStar) {
		return p.finishDict(open.Span)
	}
	first, err := p.parseStarElt()
	if err != nil {
		return nil, err
	}
	if p.Match(Colon) {
		if _, ok := first.(*StarredExpr); ok {
			return nil, p.FailAt(first.Pos(), "invalid dict display")
		}
		value, err := p.parseTest()
		if err != nil {
			return nil, err
		}
		if p.isCompStart() {
			return p.finishComprehension("dict", open.Span, value, first)
		}
		return p.finishDictFrom(open.Span, first, value)
	}
	if p.isCompStart() {
		return p.finishComprehension("set", open.Span, first)
	}
	elts := []Expr{first}
	for p.Match(Comma) {
		if p.Check(RBrace) {
			break
		}
		elt, err := p.parseStarElt()
		if err != nil {
			return nil, err
		}
		elts = append(elts, elt)
	}
	closeTok, err := p.Expect(RBrace, "'}'")
	if err != nil {
		return nil, err
	}
	node := &SetExpr{Elts: elts}
	node.Span = gast.Merge(open.Span, closeTok.Span)
	return node, nil
}

func (p *parser) finishTuple(open gast.Span, first Expr) (Expr, error) {
	elts := []Expr{first}
	for !p.Check(RParen) && !p.AtEnd() {
		elt, err := p.parseStarElt()
		if err != nil {
			return nil, err
		}
		elts = append(elts, elt)
		if !p.Match(Comma) {
			break
		}
	}
	closeTok, err := p.Expect(RParen, "')'")
	if err != nil {
		return nil, err
	}
	node := &TupleExpr{Elts: elts, Parenthesized: true}
	node.Span = gast.Merge(open, closeTok.Span)
	return node, nil
}

// finishDict parses a dict display whose first item is a ** unpack.
func (p *parser) finishDict(open gast.Span) (Expr, error) {
	item, err := p.parseDictItem()
	if err != nil {
		return nil, err
	}
	items := []*DictItem{item}
	for p.Match(Comma) {
		if p.Check(RBrace) {
			break
		}
		if item, err = p.parseDictItem(); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	closeTok, err := p.Expect(RBrace, "'}'")
	if err != nil {
		return nil, err
	}
	node := &DictExpr{Items: items}
	node.Span = gast.Merge(open, closeTok.Span)
	return node, nil
}

func (p *parser) finishDictFrom(open gast.Span, key, value Expr) (Expr, error) {
	items := []*DictItem{dictItem(key, value)}
	for p.Match(Comma) {
		if p.Check(RBrace) {
			break
		}
		item, err := p.parseDictItem()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	closeTok, err := p.Expect(RBrace, "'}'")
	if err != nil {
		return nil, err
	}
	node := &DictExpr{Items: items}
	node.Span = gast.Merge(open, closeTok.Span)
	return node, nil
}

func (p *parser) parseDictItem() (*DictItem, error) {
	if starTok, ok := p.matchToken(DoubleStar); ok {
		value, err := p.parseBinary(0)
		if err != nil {
			return nil, err
		}
		item := &DictItem{Value: value}
		item.Span = gast.Merge(starTok.Span, value.Pos())
		return item, nil
	}
	key, err := p.parseTest()
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(Colon, "':' in dict display"); err != nil {
		return nil, err
	}
	value, err := p.parseTest()
	if err != nil {
		return nil, err
	}
	return dictItem(key, value), nil
}

func dictItem(key, value Expr) *DictItem {
	item := &DictItem{Key: key, Value: value}
	item.Span = gast.Merge(key.Pos(), value.Pos())
	return item
}

// finishComprehension parses the clause list of a comprehension whose
// first element (and key, for dict comprehensions) is already parsed.
func (p *parser) finishComprehension(kind string, open gast.Span, elt Expr, key ...Expr) (Expr, error) {
	generators, err := p.parseComprehension()
	if err != nil {
		return nil, err
	}
	closer := RBracket
	switch kind {
	case "generator":
		closer = RParen
	case "set", "dict":
		closer = RBrace
	}
	closeTok, err := p.Expect(closer, "'"+closer+"' to close comprehension")
	if err != nil {
		return nil, err
	}
	node := &ComprehensionExpr{Kind: kind, Elt: elt, Generators: generators}
	if len(key) > 0 {
		node.Key = key[0]
	}
	node.Span = gast.Merge(open, closeTok.Span)
	return node, nil
}

func (p *parser) parseComprehension() ([]*ComprehensionClause, error) {
	var clauses []*ComprehensionClause
	for {
		async := p.Match(KindAsync)
		if _, err := p.Expect(KindFor, "'for' in comprehension"); err != nil {
			return nil, err
		}
		clause := &ComprehensionClause{Async: async}
		var err error
		if clause.Target, err = p.parseTargetList(); err != nil {
			return nil, err
		}
		if _, err := p.Expect(KindIn, "'in' in comprehension"); err != nil {
			return nil, err
		}
		if clause.Iter, err = p.parseOrTest(); err != nil {
			return nil, err
		}
		for p.Match(KindIf) {
			cond, err := p.parseOrTest()
			if err != nil {
				return nil, err
			}
			clause.Ifs = append(clause.Ifs, cond)
		}
		clause.Span = p.SpanFrom(clause.Target.Pos())
		clauses = append(clauses, clause)
		if !p.isCompStart() {
			return clauses, nil
		}
	}
}

// parseCallArgs parses call arguments; the '(' is already consumed
// and the ')' is consumed here, its token returned for spans.
func (p *parser) parseCallArgs() ([]*Arg, gast.Token, error) {
	var args []*Arg
	sawKeyword := false
	sawDoubleStar := false
	for !p.Check(RParen) && !p.AtEnd() {
		arg, err := p.parseArg()
		if err != nil {
			return nil, gast.Token{}, err
		}
		if arg.Star == "" && arg.Name == "" {
			if sawKeyword {
				return nil, gast.Token{}, p.FailAt(arg.Span, "positional argument follows keyword argument")
			}
			if sawDoubleStar {
				return nil, gast.Token{}, p.FailAt(arg.Span, "positional argument follows keyword argument unpacking")
			}
		}
		if arg.Star == Star && sawDoubleStar {
			return nil, gast.Token{}, p.FailAt(arg.Span, "iterable argument unpacking follows keyword argument unpacking")
		}
		if arg.Name != "" {
			sawKeyword = true
		}
		sawDoubleStar = sawDoubleStar || arg.Star == DoubleStar
		if p.isCompStart() && arg.Name == "" && arg.Star == "" && len(args) == 0 {
			generators, err := p.parseComprehension()
			if err != nil {
				return nil, gast.Token{}, err
			}
			comp := &ComprehensionExpr{Kind: "generator", Elt: arg.Expr, Generators: generators}
			comp.Span = arg.Expr.Pos()
			arg.Expr = comp
		}
		args = append(args, arg)
		if !p.Match(Comma) {
			break
		}
	}
	closeTok, err := p.Expect(RParen, "')' to close arguments")
	if err != nil {
		return nil, gast.Token{}, err
	}
	return args, closeTok, nil
}

func (p *parser) parseArg() (*Arg, error) {
	arg := &Arg{}
	if starTok, ok := p.matchToken(DoubleStar); ok {
		value, err := p.parseTest()
		if err != nil {
			return nil, err
		}
		arg.Star = DoubleStar
		arg.Expr = value
		arg.Span = gast.Merge(starTok.Span, value.Pos())
		return arg, nil
	}
	if starTok, ok := p.matchToken(Star); ok {
		value, err := p.parseTest()
		if err != nil {
			return nil, err
		}
		arg.Star = Star
		arg.Expr = value
		arg.Span = gast.Merge(starTok.Span, value.Pos())
		return arg, nil
	}
	expr, err := p.parseNamedExpr()
	if err != nil {
		return nil, err
	}
	arg.Expr = expr
	arg.Span = expr.Pos()
	if p.Match(Assign) {
		name, ok := expr.(*NameExpr)
		if !ok {
			return nil, p.FailAt(expr.Pos(), "keyword argument name must be a name")
		}
		value, err := p.parseTest()
		if err != nil {
			return nil, err
		}
		arg.Name = name.ID
		arg.Expr = value
		arg.Span = gast.Merge(expr.Pos(), value.Pos())
	}
	return arg, nil
}

// parseSubscriptContents parses subscript items with slices; the '['
// is already consumed and the ']' is consumed here, its token returned
// for spans.
func (p *parser) parseSubscriptContents() (Expr, gast.Token, error) {
	var items []Expr
	for !p.Check(RBracket) && !p.AtEnd() {
		item, err := p.parseSubscriptItem()
		if err != nil {
			return nil, gast.Token{}, err
		}
		items = append(items, item)
		if !p.Match(Comma) {
			break
		}
	}
	closeTok, err := p.Expect(RBracket, "']'")
	if err != nil {
		return nil, gast.Token{}, err
	}
	if len(items) == 1 {
		return items[0], closeTok, nil
	}
	node := &TupleExpr{Elts: items}
	node.Span = gast.Merge(items[0].Pos(), closeTok.Span)
	return node, closeTok, nil
}

func (p *parser) parseSubscriptItem() (Expr, error) {
	if p.Check(Colon) {
		return p.parseSlice(nil)
	}
	first, err := p.parseStarElt()
	if err != nil {
		return nil, err
	}
	if !p.Check(Colon) {
		return first, nil
	}
	if _, ok := first.(*StarredExpr); ok {
		return nil, p.FailAt(first.Pos(), "invalid starred expression in slice")
	}
	return p.parseSlice(first)
}

func (p *parser) parseSlice(lower Expr) (Expr, error) {
	var start gast.Span
	if lower != nil {
		start = lower.Pos()
	} else {
		start = p.Current().Span
	}
	if _, err := p.Expect(Colon, "':' in subscript"); err != nil {
		return nil, err
	}
	node := &SliceExpr{Lower: lower}
	var err error
	if !p.sliceBoundEnd() {
		if node.Upper, err = p.parseTest(); err != nil {
			return nil, err
		}
	}
	if p.Match(Colon) && !p.sliceBoundEnd() {
		if node.Step, err = p.parseTest(); err != nil {
			return nil, err
		}
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

func (p *parser) sliceBoundEnd() bool {
	switch p.Current().Kind {
	case Colon, Comma, RBracket, KindEOF:
		return true
	}
	return false
}

// parseTargetList parses assignment targets of for headers,
// comprehensions, and with items; it stops before keywords such as
// `in`.
func (p *parser) parseTargetList() (Expr, error) {
	first, err := p.parseTargetExpr()
	if err != nil {
		return nil, err
	}
	if !p.Check(Comma) {
		return first, nil
	}
	elts := []Expr{first}
	for p.Match(Comma) {
		if !p.atTargetStart() {
			break
		}
		elt, err := p.parseTargetExpr()
		if err != nil {
			return nil, err
		}
		elts = append(elts, elt)
	}
	node := &TupleExpr{Elts: elts}
	node.Span = gast.Merge(first.Pos(), elts[len(elts)-1].Pos())
	return node, nil
}

func (p *parser) atTargetStart() bool {
	switch p.Current().Kind {
	case KindName, LParen, LBracket, Star:
		return true
	}
	return false
}

func (p *parser) parseTargetExpr() (Expr, error) {
	var start gast.Span
	star := false
	if starTok, ok := p.matchToken(Star); ok {
		star = true
		start = starTok.Span
	} else {
		start = p.Current().Span
	}
	var base Expr
	var err error
	switch p.Current().Kind {
	case KindName:
		tok := p.Advance()
		base = &NameExpr{Span: tok.Span, ID: tok.Text}
	case LParen:
		open := p.Advance()
		if closeTok, ok := p.matchToken(RParen); ok {
			tuple := &TupleExpr{Parenthesized: true}
			tuple.Span = gast.Merge(open.Span, closeTok.Span)
			base = tuple
			break
		}
		if base, err = p.parseTargetList(); err != nil {
			return nil, err
		}
		if _, err := p.Expect(RParen, "')'"); err != nil {
			return nil, err
		}
		if tuple, ok := base.(*TupleExpr); ok {
			tuple.Parenthesized = true
		}
		markParenthesizedStar(base)
	case LBracket:
		open := p.Advance()
		var elts []Expr
		for !p.Check(RBracket) && !p.AtEnd() {
			elt, err := p.parseTargetExpr()
			if err != nil {
				return nil, err
			}
			elts = append(elts, elt)
			if !p.Match(Comma) {
				break
			}
		}
		closeTok, err := p.Expect(RBracket, "']'")
		if err != nil {
			return nil, err
		}
		list := &ListExpr{Elts: elts}
		list.Span = gast.Merge(open.Span, closeTok.Span)
		base = list
	default:
		return nil, p.Fail("expected a target, found " + gast.Describe(p.Current()))
	}
	for {
		switch p.Current().Kind {
		case Dot:
			p.Advance()
			name, err := p.Expect(KindName, "attribute name")
			if err != nil {
				return nil, err
			}
			node := &AttributeExpr{Value: base, Attr: name.Text}
			node.Span = gast.Merge(base.Pos(), name.Span)
			base = node
		case LBracket:
			p.Advance()
			index, closeTok, err := p.parseSubscriptContents()
			if err != nil {
				return nil, err
			}
			node := &SubscriptExpr{Value: base, Index: index}
			node.Span = gast.Merge(base.Pos(), closeTok.Span)
			base = node
		default:
			if star {
				node := &StarredExpr{Value: base}
				node.Span = p.SpanFrom(start)
				return node, nil
			}
			return base, nil
		}
	}
}

// markParenthesizedStar flags a lone starred expression that was
// wrapped in parentheses, as in (*b): the symbol-table checks reject
// such stars in assignment and for targets.
func markParenthesizedStar(expr Expr) {
	if star, ok := expr.(*StarredExpr); ok {
		star.Parenthesized = true
	}
}

// parseParameters parses a comma-separated parameter list, stopping at
// terminator without consuming it; the caller handles the surrounding
// parentheses of a def or the colon of a lambda. Lambda parameters
// reject annotations.
func (p *parser) parseParameters(allowAnnotations bool, terminator string) (*Parameters, error) {
	start := p.Current().Span
	var err error
	params := &Parameters{}
	posOnlySeen := false
	starSeen := false
	sawDefault := false
	for !p.Check(terminator) && !p.AtEnd() {
		switch {
		case p.Check(Slash):
			if posOnlySeen || len(params.PosOrKeyword) == 0 {
				return nil, p.Fail("at least one parameter must precede '/'")
			}
			p.Advance()
			params.PosOnly = params.PosOrKeyword
			params.PosOrKeyword = nil
			posOnlySeen = true
		case p.Check(Star) && !p.CheckAhead(1, KindName):
			p.Advance()
			if starSeen {
				return nil, p.Fail("only one '*' is allowed in parameters")
			}
			starSeen = true
			params.StarMarker = true
		case p.Check(DoubleStar):
			if params.Kwarg != nil {
				return nil, p.Fail("only one '**' parameter is allowed")
			}
			if params.Kwarg, err = p.parseParam(allowAnnotations); err != nil {
				return nil, err
			}
		default:
			isVararg := p.Check(Star)
			param, err := p.parseParam(allowAnnotations)
			if err != nil {
				return nil, err
			}
			switch {
			case isVararg:
				if param.Default != nil {
					return nil, p.FailAt(param.Span, "variadic parameter cannot have a default")
				}
				params.Vararg = param
				starSeen = true
			case starSeen:
				params.KwOnly = append(params.KwOnly, param)
			default:
				if sawDefault && param.Default == nil {
					return nil, p.FailAt(param.Span, "parameter without a default follows a parameter with a default")
				}
				params.PosOrKeyword = append(params.PosOrKeyword, param)
			}
			if param.Default != nil {
				sawDefault = true
			}
		}
		if !p.Match(Comma) {
			break
		}
	}
	params.Span = p.SpanFrom(start)
	return params, nil
}

// parseParam parses one parameter, including the * or ** prefix when
// present.
func (p *parser) parseParam(allowAnnotations bool) (*Param, error) {
	start := p.Current().Span
	variadic := ""
	if tok, ok := p.matchToken(DoubleStar); ok {
		variadic = tok.Kind
	} else if tok, ok := p.matchToken(Star); ok {
		variadic = tok.Kind
	}
	name, err := p.Expect(KindName, "parameter name")
	if err != nil {
		return nil, err
	}
	param := &Param{Span: name.Span, Name: name.Text}
	if allowAnnotations && p.Match(Colon) {
		if param.Annotation, err = p.parseTest(); err != nil {
			return nil, err
		}
	}
	if p.Match(Assign) {
		if variadic != "" {
			return nil, p.FailAt(name.Span, "variadic parameter cannot have a default")
		}
		if param.Default, err = p.parseTest(); err != nil {
			return nil, err
		}
	}
	param.Span = p.SpanFrom(start)
	return param, nil
}
