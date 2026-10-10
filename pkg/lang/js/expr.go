package js

import (
	"slices"

	"github.com/stubbedev/gast/pkg/gast"
)

// binaryLevels lists left-associative binary operator levels from
// loosest to tightest. The nullish, exponentiation, conditional,
// assignment, and comma operators have dedicated parsing.
var binaryLevels = [][]string{
	{Nullish},
	{LogicalOr},
	{LogicalAnd},
	{Pipe},
	{Caret},
	{Ampersand},
	{Equal, StrictEqual, NotEqual, StrictNotEqual},
	{Lt, Gt, LtEqual, GtEqual, KindInstanceof, KindIn},
	{ShiftLeft, ShiftRight, UnsignedShift},
	{Plus, Minus},
	{Asterisk, Slash, Percent},
}

// assignmentOps are the right-associative assignment operators; the
// plain "=" admits destructuring targets, the compounds do not.
var assignmentOps = []string{
	Assign, AddAssign, SubAssign, MulAssign, DivAssign, ModAssign,
	BitAndAssign, BitOrAssign, BitXorAssign, ShiftLeftAssign,
	ShiftRightAssign, UnsignedShiftAssign, PowAssign, LogicalAndAssign,
	LogicalOrAssign, NullishAssign,
}

// parseExpr parses a full expression, folding the comma operator into
// a sequence. noIn hides the in operator, used inside for heads.
func (p *parser) parseExpr(noIn bool) (Expr, error) {
	expr, err := p.parseAssignExpr(noIn)
	if err != nil {
		return nil, err
	}
	if !p.Check(Comma) {
		return expr, nil
	}
	node := &SequenceExpr{Exprs: []Expr{expr}}
	for p.Match(Comma) {
		next, err := p.parseAssignExpr(noIn)
		if err != nil {
			return nil, err
		}
		node.Exprs = append(node.Exprs, next)
	}
	node.Span = gast.Merge(node.Exprs[0].Pos(), node.Exprs[len(node.Exprs)-1].Pos())
	return node, nil
}

// parseAssignExpr parses arrows, yield, await, conditional, and
// assignment expressions.
func (p *parser) parseAssignExpr(noIn bool) (Expr, error) {
	switch {
	case p.inGenerator && p.isWord("yield"):
		return p.parseYield()
	case p.Check(LParen) && p.arrowAhead(0):
		return p.parseParenArrow(false)
	case p.isWord("async") && p.CheckAhead(1, KindFunction) && !p.lineBreakBetween(p.Current(), p.Peek(1)):
		return p.parseFunctionExpr(true)
	case p.isWord("async"):
		switch {
		case p.CheckAhead(1, Arrow) && !p.lineBreakBetween(p.Current(), p.Peek(1)):
			// `async => ...`: an arrow with a parameter named async
			async := p.Advance()
			binding := &IdentExpr{Span: async.Span, Name: async.Text}
			return p.parseArrowTail(async.Span, []*Param{{Span: async.Span, Binding: binding}}, false)
		case p.Peek(1).Kind == KindIdent && p.Peek(2).Kind == Arrow &&
			!p.lineBreakBetween(p.Current(), p.Peek(1)) && !p.lineBreakBetween(p.Peek(1), p.Peek(2)):
			// `async x => ...`
			async := p.Advance()
			param := p.Advance()
			binding := &IdentExpr{Span: param.Span, Name: param.Text}
			return p.parseArrowTail(async.Span, []*Param{{Span: param.Span, Binding: binding}}, true)
		case p.Peek(1).Kind == LParen && p.arrowAhead(1) &&
			!p.lineBreakBetween(p.Current(), p.Peek(1)):
			// `async (params) => ...`
			async := p.Advance()
			return p.parseParenArrowFrom(async, true)
		}
	case p.Check(KindIdent) && p.CheckAhead(1, Arrow) && !p.lineBreakBetween(p.Current(), p.Peek(1)):
		// `x => ...`
		param := p.Advance()
		binding := &IdentExpr{Span: param.Span, Name: param.Text}
		return p.parseArrowTail(param.Span, []*Param{{Span: param.Span, Binding: binding}}, false)
	}
	left, err := p.parseConditional(noIn)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(assignmentOps, p.Current().Kind) {
		if err := p.checkExprStatement(left); err != nil {
			return nil, err
		}
		return left, nil
	}
	op := p.Advance()
	if err := p.validateAssignTarget(left, op.Kind == Assign); err != nil {
		return nil, err
	}
	node := &AssignExpr{Op: op.Kind, Target: left}
	if node.Value, err = p.parseAssignExpr(false); err != nil {
		return nil, err
	}
	node.Span = gast.Merge(left.Pos(), node.Value.Pos())
	return node, nil
}

// parseYield parses yield and yield* expressions; the argument cannot
// start on a new line.
func (p *parser) parseYield() (Expr, error) {
	yieldTok := p.Advance()
	node := &YieldExpr{}
	var err error
	switch {
	case p.Match(Asterisk):
		node.Delegate = true
		if node.Arg, err = p.parseAssignExpr(false); err != nil {
			return nil, err
		}
	case !p.hadLineBreak() && !p.stmtExprEnd():
		if node.Arg, err = p.parseAssignExpr(false); err != nil {
			return nil, err
		}
	}
	node.Span = p.SpanFrom(yieldTok.Span)
	return node, nil
}

// parseConditional parses the right-associative ternary operator.
func (p *parser) parseConditional(noIn bool) (Expr, error) {
	cond, err := p.parseBinary(0, noIn)
	if err != nil {
		return nil, err
	}
	if !p.Match(Question) {
		return cond, nil
	}
	node := &CondExpr{Cond: cond}
	if node.Then, err = p.parseAssignExpr(false); err != nil {
		return nil, err
	}
	if _, err := p.Expect(Colon, "':' in conditional expression"); err != nil {
		return nil, err
	}
	if node.Else, err = p.parseAssignExpr(noIn); err != nil {
		return nil, err
	}
	node.Span = gast.Merge(cond.Pos(), node.Else.Pos())
	return node, nil
}

// parseBinary walks the precedence table: at each level it parses the
// tighter level below, then folds same-level operators
// left-associatively. noIn hides the in operator inside for heads.
func (p *parser) parseBinary(level int, noIn bool) (Expr, error) {
	if level == len(binaryLevels) {
		return p.parseExponentiation()
	}
	ops := binaryLevels[level]
	left, err := p.parseBinary(level+1, noIn)
	if err != nil {
		return nil, err
	}
	for slices.Contains(ops, p.Current().Kind) {
		if noIn && p.Check(KindIn) {
			break
		}
		op := p.Advance()
		right, err := p.parseBinary(level+1, noIn)
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Span: gast.Merge(left.Pos(), right.Pos()), Op: op.Kind, Left: left, Right: right}
	}
	return left, nil
}

// parseExponentiation parses the right-associative ** operator; a
// unary expression on its left must be parenthesized.
func (p *parser) parseExponentiation() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	if !p.Check(Pow) {
		return left, nil
	}
	if isUnaryLike(left) {
		return nil, p.FailAt(left.Pos(), "unary operand of '**' must be parenthesized")
	}
	p.Advance()
	right, err := p.parseExponentiation()
	if err != nil {
		return nil, err
	}
	return &BinaryExpr{Span: gast.Merge(left.Pos(), right.Pos()), Op: Pow, Left: left, Right: right}, nil
}

// isUnaryLike reports whether expr is a prefix unary or update
// expression, which cannot be the left operand of **.
func isUnaryLike(expr Expr) bool {
	switch node := expr.(type) {
	case *UnaryExpr:
		return true
	case *UpdateExpr:
		return node.Prefix
	case *AwaitExpr:
		return true
	}
	return false
}

// parseUnary parses prefix operators; await is one when an expression
// cannot have just ended.
func (p *parser) parseUnary() (Expr, error) {
	tok := p.Current()
	switch tok.Kind {
	case Plus, Minus, Tilde, Bang, KindDelete, KindTypeof, KindVoid:
		p.Advance()
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Span: p.SpanFrom(tok.Span), Op: tok.Kind, Expr: expr}, nil
	case Incr, Decr:
		p.Advance()
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		if !isSimpleTarget(expr) {
			return nil, p.FailAt(expr.Pos(), "invalid operand for '"+tok.Kind+"'")
		}
		return &UpdateExpr{Span: p.SpanFrom(tok.Span), Op: tok.Kind, Expr: expr, Prefix: true}, nil
	}
	if p.isWord("await") && p.awaitOperator() {
		p.Advance()
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &AwaitExpr{Span: p.SpanFrom(tok.Span), Arg: expr}, nil
	}
	return p.parsePostfix()
}

// awaitOperator reports whether a contextual await acts as the unary
// operator: always inside async functions, elsewhere only when the
// previous token cannot end an expression.
func (p *parser) awaitOperator() bool {
	if p.inAsync {
		return true
	}
	prev := p.Peek(-1)
	if prev.Span.End > p.Current().Span.Start {
		return true // start of input
	}
	return !endsExpression(prev.Kind, prev.Text)
}

// parsePostfix parses ++/-- in postfix position; a line break before
// the operator attaches it to the next statement instead.
func (p *parser) parsePostfix() (Expr, error) {
	expr, err := p.parseLeftHandSide()
	if err != nil {
		return nil, err
	}
	if (p.Check(Incr) || p.Check(Decr)) && !p.hadLineBreak() {
		if !isSimpleTarget(expr) {
			return nil, p.FailAt(expr.Pos(), "invalid operand for '"+p.Current().Kind+"'")
		}
		op := p.Advance()
		return &UpdateExpr{Span: gast.Merge(expr.Pos(), op.Span), Op: op.Kind, Expr: expr}, nil
	}
	return expr, nil
}

// parseLeftHandSide parses new expressions and the member/call chain.
func (p *parser) parseLeftHandSide() (Expr, error) {
	var expr Expr
	if p.Check(KindNew) {
		newTok := p.Advance()
		if p.Match(Dot) {
			prop, perr := p.Expect(KindIdent, "'target' after 'new.'")
			if perr != nil {
				return nil, perr
			}
			if prop.Text != "target" {
				return nil, p.FailAt(prop.Span, "expected 'target' after 'new.'")
			}
			expr = &MetaProperty{Span: gast.Merge(newTok.Span, prop.Span), Meta: "new", Prop: "target"}
		} else {
			callee, err := p.parseMemberBase()
			if err != nil {
				return nil, err
			}
			node := &NewExpr{Span: p.SpanFrom(newTok.Span), Callee: callee}
			if p.Check(LParen) {
				if node.Args, err = p.parseArgs(); err != nil {
					return nil, err
				}
				node.Span = p.SpanFrom(newTok.Span)
			}
			expr = node
		}
	} else {
		var err error
		if expr, err = p.parsePrimary(); err != nil {
			return nil, err
		}
	}
	return p.parseSubscripts(expr)
}

// parseMemberBase parses the callee of new: a primary with member
// accesses but no call arguments, so `new (a)(b)` applies the argument
// list to new itself.
func (p *parser) parseMemberBase() (Expr, error) {
	if p.Check(KindNew) {
		newTok := p.Advance()
		if p.Match(Dot) {
			prop, perr := p.Expect(KindIdent, "'target' after 'new.'")
			if perr != nil {
				return nil, perr
			}
			if prop.Text != "target" {
				return nil, p.FailAt(prop.Span, "expected 'target' after 'new.'")
			}
			return &MetaProperty{Span: gast.Merge(newTok.Span, prop.Span), Meta: "new", Prop: "target"}, nil
		}
		callee, err := p.parseMemberBase()
		if err != nil {
			return nil, err
		}
		node := &NewExpr{Span: p.SpanFrom(newTok.Span), Callee: callee}
		if p.Check(LParen) {
			if node.Args, err = p.parseArgs(); err != nil {
				return nil, err
			}
			node.Span = p.SpanFrom(newTok.Span)
		}
		return node, nil
	}
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch p.Current().Kind {
		case Dot:
			p.Advance()
			prop, err := p.propertyName("property name")
			if err != nil {
				return nil, err
			}
			expr = &MemberExpr{Span: gast.Merge(expr.Pos(), prop.Pos()), Obj: expr, Prop: prop}
		case LBracket:
			p.Advance()
			index, err := p.parseExpr(false)
			if err != nil {
				return nil, err
			}
			close, err := p.Expect(RBracket, "']' after computed member")
			if err != nil {
				return nil, err
			}
			expr = &MemberExpr{Span: gast.Merge(expr.Pos(), close.Span), Obj: expr, Prop: index, Computed: true}
		default:
			return expr, nil
		}
	}
}

// parseSubscripts parses the postfix chain: property and index access,
// calls, optional chaining, and tagged templates.
func (p *parser) parseSubscripts(expr Expr) (Expr, error) {
	for {
		switch p.Current().Kind {
		case Dot:
			p.Advance()
			prop, err := p.propertyName("property name")
			if err != nil {
				return nil, err
			}
			expr = &MemberExpr{Span: gast.Merge(expr.Pos(), prop.Pos()), Obj: expr, Prop: prop}
		case OptChain:
			p.Advance()
			switch p.Current().Kind {
			case LParen:
				args, err := p.parseArgs()
				if err != nil {
					return nil, err
				}
				expr = &CallExpr{Span: p.SpanFrom(expr.Pos()), Callee: expr, Args: args, Optional: true}
			case LBracket:
				p.Advance()
				index, err := p.parseExpr(false)
				if err != nil {
					return nil, err
				}
				close, err := p.Expect(RBracket, "']' after computed member")
				if err != nil {
					return nil, err
				}
				expr = &MemberExpr{Span: gast.Merge(expr.Pos(), close.Span), Obj: expr, Prop: index, Computed: true, Optional: true}
			default:
				prop, err := p.propertyName("property after '?.")
				if err != nil {
					return nil, err
				}
				expr = &MemberExpr{Span: gast.Merge(expr.Pos(), prop.Pos()), Obj: expr, Prop: prop, Optional: true}
			}
		case LBracket:
			p.Advance()
			index, err := p.parseExpr(false)
			if err != nil {
				return nil, err
			}
			close, err := p.Expect(RBracket, "']' after computed member")
			if err != nil {
				return nil, err
			}
			expr = &MemberExpr{Span: gast.Merge(expr.Pos(), close.Span), Obj: expr, Prop: index, Computed: true}
		case LParen:
			args, err := p.parseArgs()
			if err != nil {
				return nil, err
			}
			span := p.SpanFrom(expr.Pos())
			expr = &CallExpr{Span: span, Callee: expr, Args: args}
		case KindTemplateHead, KindNoSubTemplate:
			first := p.Advance()
			lit, err := p.parseTemplate(first)
			if err != nil {
				return nil, err
			}
			expr = &TaggedTemplate{Span: gast.Merge(expr.Pos(), lit.Span), Tag: expr, Quasis: lit.Quasis, Exprs: lit.Exprs}
		default:
			return expr, nil
		}
	}
}

// propertyName reads an identifierName or private name after "." or
// "?."; keywords are valid property names.
func (p *parser) propertyName(what string) (Expr, error) {
	tok := p.Current()
	if tok.Kind == KindIdent || tok.Kind == KindPrivateName {
		p.Advance()
		if tok.Kind == KindPrivateName {
			return &PrivateName{Span: tok.Span, Name: tok.Text[1:]}, nil
		}
		return &IdentExpr{Span: tok.Span, Name: tok.Text}, nil
	}
	if _, ok := keywords[tok.Text]; ok {
		p.Advance()
		return &IdentExpr{Span: tok.Span, Name: tok.Text}, nil
	}
	return nil, p.Fail("expected " + what + ", found " + gast.Describe(tok))
}

// parseArgs parses a call argument list with spread and a trailing
// comma; the "(" is current.
func (p *parser) parseArgs() ([]Expr, error) {
	p.Advance()
	var args []Expr
	for !p.Check(RParen) {
		if p.AtEnd() {
			return nil, p.Fail("expected ')' to close argument list")
		}
		arg, err := p.parseArg()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if !p.Match(Comma) {
			break
		}
	}
	if _, err := p.Expect(RParen, "')' to close argument list"); err != nil {
		return nil, err
	}
	if len(args) == 0 {
		args = []Expr{}
	}
	return args, nil
}

func (p *parser) parseArg() (Expr, error) {
	if p.Match(Ellipsis) {
		start := p.Peek(-1).Span
		expr, err := p.parseAssignExpr(false)
		if err != nil {
			return nil, err
		}
		return &SpreadElement{Span: gast.Merge(start, expr.Pos()), Expr: expr}, nil
	}
	return p.parseAssignExpr(false)
}

// parsePrimary parses literals, identifiers, and bracketed forms.
func (p *parser) parsePrimary() (Expr, error) {
	tok := p.Current()
	switch tok.Kind {
	case KindThis:
		p.Advance()
		return &ThisExpr{Span: tok.Span}, nil
	case KindSuper:
		p.Advance()
		return &SuperExpr{Span: tok.Span}, nil
	case KindTrue, KindFalse:
		p.Advance()
		return &BoolLit{Span: tok.Span, Value: tok.Kind == KindTrue}, nil
	case KindNull:
		p.Advance()
		return &NullLit{Span: tok.Span}, nil
	case KindIdent:
		p.Advance()
		return &IdentExpr{Span: tok.Span, Name: tok.Text}, nil
	case KindNum, KindBigInt:
		p.Advance()
		return p.litToken(tok), nil
	case KindString:
		p.Advance()
		return &StringLit{Span: tok.Span, Text: tok.Text, Value: decodeString(tok.Text)}, nil
	case KindRegex:
		p.Advance()
		return &RegexLit{Span: tok.Span, Text: tok.Text}, nil
	case KindNoSubTemplate, KindTemplateHead:
		p.Advance()
		return p.parseTemplate(tok)
	case LParen:
		p.Advance()
		inner, err := p.parseExpr(false)
		if err != nil {
			return nil, err
		}
		close, err := p.Expect(RParen, "')'")
		if err != nil {
			return nil, err
		}
		return &ParenExpr{Span: gast.Merge(tok.Span, close.Span), Expr: inner}, nil
	case LBracket:
		return p.parseArrayExpr()
	case LBrace:
		return p.parseObjectExpr()
	case KindFunction:
		return p.parseFunctionExpr(false)
	case KindClass:
		return p.parseClassExpr()
	case KindImport:
		return p.parseImportExpr()
	}
	return nil, p.Fail("unexpected " + gast.Describe(tok))
}

// litToken converts a number or BigInt token into its literal node.
func (p *parser) litToken(tok gast.Token) Expr {
	if tok.Kind == KindBigInt {
		return &BigIntLit{Span: tok.Span, Text: tok.Text}
	}
	return &NumberLit{Span: tok.Span, Text: tok.Text}
}

// parseImportExpr parses import.meta and dynamic import(...) in
// expression position.
func (p *parser) parseImportExpr() (Expr, error) {
	importTok := p.Advance()
	if p.Match(Dot) {
		meta, merr := p.Expect(KindIdent, "'meta' after 'import.'")
		if merr != nil {
			return nil, merr
		}
		if meta.Text != "meta" {
			return nil, p.FailAt(meta.Span, "expected 'meta' after 'import.'")
		}
		return &MetaProperty{Span: gast.Merge(importTok.Span, meta.Span), Meta: "import", Prop: "meta"}, nil
	}
	if !p.Check(LParen) {
		return nil, p.Fail("expected '(' after 'import' in expression")
	}
	p.Advance()
	source, err := p.parseAssignExpr(false)
	if err != nil {
		return nil, err
	}
	for p.Match(Comma) {
		if p.Check(RParen) {
			break
		}
		if _, err := p.parseAssignExpr(false); err != nil {
			return nil, err
		}
	}
	close, err := p.Expect(RParen, "')' after dynamic import")
	if err != nil {
		return nil, err
	}
	return &ImportCall{Span: gast.Merge(importTok.Span, close.Span), Source: source}, nil
}

// parseArrayExpr parses an array literal; a nil element is a hole.
func (p *parser) parseArrayExpr() (Expr, error) {
	open := p.Advance()
	var elements []Expr
	for !p.Check(RBracket) {
		if p.AtEnd() {
			return nil, p.Fail("expected ']' to close array literal")
		}
		if p.Match(Comma) {
			elements = append(elements, nil)
			continue
		}
		elem, err := p.parseArg()
		if err != nil {
			return nil, err
		}
		elements = append(elements, elem)
		if !p.Match(Comma) {
			break
		}
	}
	close, err := p.Expect(RBracket, "']' to close array literal")
	if err != nil {
		return nil, err
	}
	return &ArrayExpr{Span: gast.Merge(open.Span, close.Span), Elements: elements}, nil
}

// parseObjectExpr parses an object literal with shorthand, methods,
// accessors, computed keys, and spread. Pattern initializers
// (`{a = 1}`) parse as the cover grammar for destructuring assignment.
func (p *parser) parseObjectExpr() (Expr, error) {
	open := p.Advance()
	node := &ObjectExpr{}
	for !p.Check(RBrace) {
		if p.AtEnd() {
			return nil, p.Fail("expected '}' to close object literal")
		}
		prop, err := p.parseObjectProperty()
		if err != nil {
			return nil, err
		}
		node.Props = append(node.Props, prop)
		if !p.Match(Comma) {
			break
		}
	}
	close, err := p.Expect(RBrace, "'}' to close object literal")
	if err != nil {
		return nil, err
	}
	node.Span = gast.Merge(open.Span, close.Span)
	return node, nil
}

func (p *parser) parseObjectProperty() (Expr, error) {
	if p.Match(Ellipsis) {
		start := p.Peek(-1).Span
		expr, err := p.parseAssignExpr(false)
		if err != nil {
			return nil, err
		}
		return &SpreadProperty{Span: gast.Merge(start, expr.Pos()), Expr: expr}, nil
	}
	start := p.Current().Span
	async, generator, accessor := false, false, ""
	if p.isWord("async") && isModifier("async", p.Peek(1).Kind, p.Peek(2).Kind) &&
		!p.lineBreakBetween(p.Current(), p.Peek(1)) {
		async = true
		p.Advance()
	}
	if p.Match(Asterisk) {
		generator = true
	}
	if (p.isWord("get") || p.isWord("set")) && isModifier(p.Current().Text, p.Peek(1).Kind, p.Peek(2).Kind) {
		accessor = p.Advance().Text
	}
	key, computed, err := p.parsePropertyKey()
	if err != nil {
		return nil, err
	}
	if p.Match(Colon) {
		value, err := p.parseAssignExpr(false)
		if err != nil {
			return nil, err
		}
		prop := &Property{Span: p.SpanFrom(start), Key: key, Value: value, Kind: "init", Computed: computed}
		if assign, ok := value.(*AssignExpr); ok && assign.Op == Assign {
			// cover grammar: {a: b = 1} is a destructuring default
			prop.Value = assign.Target
			prop.Default = assign.Value
		}
		if async || generator || accessor != "" {
			return nil, p.FailAt(prop.Span, "expected '(' after property name")
		}
		return prop, nil
	}
	if p.Check(LParen) {
		savedAsync, savedGen, savedFn := p.inAsync, p.inGenerator, p.inFunction
		p.inAsync, p.inGenerator, p.inFunction = async, generator, true
		var params []*Param
		if params, err = p.parseParams(); err != nil {
			return nil, err
		}
		var body *BlockStmt
		if body, err = p.parseBlockStmt(); err != nil {
			return nil, err
		}
		p.inAsync, p.inGenerator, p.inFunction = savedAsync, savedGen, savedFn
		kind := "method"
		switch accessor {
		case "get":
			kind = "get"
			if len(params) != 0 {
				return nil, p.FailAt(p.SpanFrom(start), "getter cannot take parameters")
			}
		case "set":
			kind = "set"
			if len(params) != 1 {
				return nil, p.FailAt(p.SpanFrom(start), "setter takes exactly one parameter")
			}
		}
		fn := &FunctionExpr{Span: p.SpanFrom(start), Async: async, Generator: generator, Params: params, Body: body}
		return &Property{Span: p.SpanFrom(start), Key: key, Value: fn, Kind: kind, Computed: computed}, nil
	}
	if accessor != "" || async || generator {
		return nil, p.FailAt(p.SpanFrom(start), "expected '(' after property name")
	}
	ident, ok := key.(*IdentExpr)
	if !ok || computed {
		return nil, p.FailAt(p.SpanFrom(start), "expected ':' after property name")
	}
	prop := &Property{
		Span: p.SpanFrom(start), Key: key,
		Value: &IdentExpr{Span: ident.Span, Name: ident.Name},
		Kind:  "init", Shorthand: true,
	}
	if p.Match(Assign) {
		if prop.Default, err = p.parseAssignExpr(false); err != nil {
			return nil, err
		}
		prop.Span = p.SpanFrom(start)
	}
	return prop, nil
}

// parseTemplate assembles a template literal from the head/middle/tail
// token sequence; first is already consumed.
func (p *parser) parseTemplate(first gast.Token) (*TemplateLit, error) {
	node := &TemplateLit{Quasis: []string{templateQuasiText(first)}}
	if first.Kind == KindNoSubTemplate {
		node.Span = first.Span
		return node, nil
	}
	for {
		expr, err := p.parseExpr(false)
		if err != nil {
			return nil, err
		}
		node.Exprs = append(node.Exprs, expr)
		cur := p.Current()
		if cur.Kind != KindTemplateMiddle && cur.Kind != KindTemplateTail {
			return nil, p.Fail("expected '}' to continue template literal, found " + gast.Describe(cur))
		}
		p.Advance()
		node.Quasis = append(node.Quasis, templateQuasiText(cur))
		if cur.Kind == KindTemplateTail {
			break
		}
	}
	node.Span = gast.Merge(first.Span, p.Peek(-1).Span)
	return node, nil
}

// templateQuasiText strips the delimiters of a template token.
func templateQuasiText(tok gast.Token) string {
	switch tok.Kind {
	case KindTemplateHead:
		return tok.Text[1 : len(tok.Text)-2]
	case KindTemplateMiddle:
		return tok.Text[1 : len(tok.Text)-2]
	case KindTemplateTail:
		return tok.Text[1 : len(tok.Text)-1]
	default:
		return tok.Text[1 : len(tok.Text)-1]
	}
}

// arrowAhead scans ahead from the "(" token at offset for the matching
// ")" followed by "=>" on the same line; bounded, linear lookahead
// with no backtracking.
func (p *parser) arrowAhead(offset int) bool {
	depth := 0
	for i := offset; ; i++ {
		tok := p.Peek(i)
		switch tok.Kind {
		case LParen, LBracket, LBrace:
			depth++
		case RParen:
			depth--
			if depth <= 0 {
				next := p.Peek(i + 1)
				return next.Kind == Arrow && !p.lineBreakBetween(tok, next)
			}
		case RBracket, RBrace:
			depth--
			if depth < 0 {
				return false
			}
		case KindEOF, Semicolon:
			return false
		}
	}
}

// parseParenArrow parses a parenthesized arrow function; the "(" is
// current and arrowAhead(0) has confirmed the "=>".
func (p *parser) parseParenArrow(async bool) (Expr, error) {
	open := p.Current() // the "(", consumed by parseParams
	params, err := p.parseParams()
	if err != nil {
		return nil, err
	}
	return p.parseArrowTail(open.Span, params, async)
}

// parseParenArrowFrom parses an async arrow whose "async" token is
// already consumed.
func (p *parser) parseParenArrowFrom(asyncTok gast.Token, async bool) (Expr, error) {
	params, err := p.parseParams()
	if err != nil {
		return nil, err
	}
	return p.parseArrowTail(asyncTok.Span, params, async)
}

// parseArrowTail consumes "=>" and the arrow body: a block or a single
// expression.
func (p *parser) parseArrowTail(start gast.Span, params []*Param, async bool) (Expr, error) {
	if _, err := p.Expect(Arrow, "'=>'"); err != nil {
		return nil, err
	}
	node := &ArrowExpr{Async: async, Params: params}
	savedAsync, savedGen, savedFn := p.inAsync, p.inGenerator, p.inFunction
	p.inAsync, p.inGenerator, p.inFunction = async, false, true
	var err error
	if p.Check(LBrace) {
		var block *BlockStmt
		if block, err = p.parseBlockStmt(); err != nil {
			p.inAsync, p.inGenerator, p.inFunction = savedAsync, savedGen, savedFn
			return nil, err
		}
		node.Body = block
	} else {
		node.Expression = true
		if node.Body, err = p.parseAssignExpr(false); err != nil {
			p.inAsync, p.inGenerator, p.inFunction = savedAsync, savedGen, savedFn
			return nil, err
		}
	}
	p.inAsync, p.inGenerator, p.inFunction = savedAsync, savedGen, savedFn
	node.Span = gast.Merge(start, node.Body.Pos())
	return node, nil
}

// validateAssignTarget rejects expressions that cannot be assigned
// to; patternOK admits the destructuring cover forms for plain "=".
func (p *parser) validateAssignTarget(expr Expr, patternOK bool) error {
	switch node := expr.(type) {
	case *IdentExpr, *MemberExpr:
		return nil
	case *ParenExpr:
		return p.validateAssignTarget(node.Expr, patternOK)
	case *ObjectExpr:
		if !patternOK {
			return p.FailAt(expr.Pos(), "invalid assignment target")
		}
		for i, prop := range node.Props {
			switch entry := prop.(type) {
			case *SpreadProperty:
				if i != len(node.Props)-1 {
					return p.FailAt(entry.Span, "rest property must be last")
				}
			case *Property:
				if entry.Kind != "init" {
					return p.FailAt(entry.Span, "invalid assignment target")
				}
				if err := p.validateAssignTarget(entry.Value, true); err != nil {
					return err
				}
			}
		}
		return nil
	case *ArrayExpr:
		if !patternOK {
			return p.FailAt(expr.Pos(), "invalid assignment target")
		}
		for i, elem := range node.Elements {
			if elem == nil {
				continue
			}
			if spread, ok := elem.(*SpreadElement); ok {
				if i != len(node.Elements)-1 {
					return p.FailAt(spread.Span, "rest element must be last")
				}
				continue
			}
			if assign, ok := elem.(*AssignExpr); ok && assign.Op == Assign {
				if err := p.validateAssignTarget(assign.Target, true); err != nil {
					return err
				}
				continue
			}
			if err := p.validateAssignTarget(elem, true); err != nil {
				return err
			}
		}
		return nil
	}
	return p.FailAt(expr.Pos(), "invalid assignment target")
}

// isSimpleTarget reports whether expr is a valid operand of ++/-- and
// compound assignment.
func isSimpleTarget(expr Expr) bool {
	switch node := expr.(type) {
	case *IdentExpr, *MemberExpr:
		return true
	case *ParenExpr:
		return isSimpleTarget(node.Expr)
	}
	return false
}

// objectHasDefault reports whether an object literal carries a pattern
// initializer, legal only in destructuring position.
func objectHasDefault(object *ObjectExpr) bool {
	for _, prop := range object.Props {
		entry, ok := prop.(*Property)
		if !ok {
			continue
		}
		if entry.Default != nil {
			return true
		}
		if nested, ok := entry.Value.(*ObjectExpr); ok && objectHasDefault(nested) {
			return true
		}
	}
	return false
}
