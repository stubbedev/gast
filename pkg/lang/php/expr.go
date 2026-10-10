package php

import (
	"slices"
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

// binaryPrecedence lists left-associative binary operator levels from
// loosest to tightest. The keyword logical operators sit above
// assignment and are listed separately in keywordBinaryPrecedence;
// assignment, ternary, coalesce, instanceof, and power have dedicated
// parsing.
var binaryPrecedence = [][]string{
	{LogicalOr},
	{LogicalAnd},
	{Pipe},
	{Caret},
	{Ampersand},
	{Equal, NotEqual, Identical, NotIdentical, AngleNotEqual, Spaceship},
	{Lt, LtEqual, Gt, GtEqual},
	{Concat},
	{ShiftLeft, ShiftRight},
	{Plus, Minus},
	{Asterisk, Slash, Percent},
}

// keywordBinaryPrecedence lists the keyword logical operators from
// loosest to tightest; they bind looser than assignment.
var keywordBinaryPrecedence = [][]string{
	{KindOr},
	{KindXor},
	{KindAnd},
}

// assignmentOps are the right-associative assignment operators.
var assignmentOps = []string{
	Assign, AddAssign, SubAssign, MulAssign, DivAssign, ModAssign,
	ConcatAssign, BitAndAssign, BitOrAssign, BitXorAssign,
	ShiftLeftAssign, ShiftRightAssign, PowAssign, CoalesceAssign,
}

// parseExpr is the entry point; precedence climbs from the keyword
// logical operators down to postfixes.
func (p *parser) parseExpr() (Expr, error) {
	return p.parseBinaryTable(keywordBinaryPrecedence, 0, p.parseLowExpr)
}

// parseBinaryTable walks a precedence table: at each level it parses
// the tighter level below, then folds same-level operators
// left-associatively.
func (p *parser) parseBinaryTable(levels [][]string, level int, bottom func() (Expr, error)) (Expr, error) {
	if level == len(levels) {
		return bottom()
	}
	ops := levels[level]
	left, err := p.parseBinaryTable(levels, level+1, bottom)
	if err != nil {
		return nil, err
	}
	for slices.Contains(ops, p.Current().Kind) {
		op := p.Advance()
		right, err := p.parseBinaryTable(levels, level+1, bottom)
		if err != nil {
			return nil, err
		}
		left = binaryExpr(op.Kind, left, right)
	}
	return left, nil
}

func binaryExpr(op string, left, right Expr) Expr {
	return &BinaryExpr{Span: gast.Merge(left.Pos(), right.Pos()), Op: op, Left: left, Right: right}
}

// parseLowExpr handles the prefix operators that sit between the
// keyword logical operators and assignment: throw, print, the include
// and require family, and yield.
func (p *parser) parseLowExpr() (Expr, error) {
	tok := p.Current()
	switch tok.Kind {
	case KindThrow:
		p.Advance()
		expr, err := p.parseLowExpr()
		if err != nil {
			return nil, err
		}
		return &ThrowExpr{Span: p.SpanFrom(tok.Span), Expr: expr}, nil
	case KindPrint, KindInclude, KindIncludeOnce, KindRequire, KindRequireOnce:
		p.Advance()
		expr, err := p.parseLowExpr()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Span: p.SpanFrom(tok.Span), Op: tok.Kind, Expr: expr}, nil
	case KindYield:
		return p.parseYield()
	}
	return p.parseAssign()
}

func (p *parser) parseYield() (Expr, error) {
	yieldTok := p.Advance()
	node := &YieldExpr{}
	if p.isContextualIdent(wordFrom) {
		p.Advance()
		node.From = true
		value, err := p.parseLowExpr()
		if err != nil {
			return nil, err
		}
		node.Value = value
		node.Span = p.SpanFrom(yieldTok.Span)
		return node, nil
	} else if !p.isExprTerminator() {
		first, err := p.parseLowExpr()
		if err != nil {
			return nil, err
		}
		if p.Match(DoubleArrow) {
			key := first
			value, err := p.parseLowExpr()
			if err != nil {
				return nil, err
			}
			node.Key = key
			node.Value = value
		} else {
			node.Value = first
		}
	}
	node.Span = p.SpanFrom(yieldTok.Span)
	return node, nil
}

func (p *parser) isExprTerminator() bool {
	switch p.Current().Kind {
	case Semicolon, Comma, RParen, RBracket, RBrace, Colon, Question, KindCloseTag:
		return true
	}
	return p.AtEnd()
}

func (p *parser) parseAssign() (Expr, error) {
	left, err := p.parseTernary()
	if err != nil {
		return nil, err
	}
	if !slices.Contains(assignmentOps, p.Current().Kind) {
		return left, nil
	}
	op := p.Advance()
	node := &AssignExpr{Op: op.Kind, Target: left}
	node.Ref = op.Kind == Assign && p.Match(Ampersand)
	if node.Value, err = p.parseLowExpr(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(left.Pos())
	return node, nil
}

func (p *parser) parseTernary() (Expr, error) {
	cond, err := p.parseCoalesce()
	if err != nil {
		return nil, err
	}
	if !p.Match(Question) {
		return cond, nil
	}
	node := &TernaryExpr{Cond: cond}
	if !p.Match(Colon) {
		if node.Then, err = p.parseExpr(); err != nil {
			return nil, err
		}
		if _, err := p.Expect(Colon, "':' in ternary expression"); err != nil {
			return nil, err
		}
	}
	if node.Else, err = p.parseLowExpr(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(cond.Pos())
	return node, nil
}

// parseCoalesce handles the right-associative ?? operator.
func (p *parser) parseCoalesce() (Expr, error) {
	left, err := p.parseBinaryTable(binaryPrecedence, 0, p.parseInstanceof)
	if err != nil {
		return nil, err
	}
	if p.Match(Coalesce) {
		right, err := p.parseCoalesce()
		if err != nil {
			return nil, err
		}
		return binaryExpr(Coalesce, left, right), nil
	}
	return left, nil
}

func (p *parser) parseInstanceof() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.Match(KindInstanceof) {
		class, err := p.parseNewTarget()
		if err != nil {
			return nil, err
		}
		left = &InstanceofExpr{Span: gast.Merge(left.Pos(), class.Pos()), Expr: left, Class: class}
	}
	return left, nil
}

func (p *parser) parseUnary() (Expr, error) {
	tok := p.Current()
	switch tok.Kind {
	case Incr, Decr, Minus, Plus, Tilde, Bang, At:
		p.Advance()
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Span: p.SpanFrom(tok.Span), Op: tok.Kind, Expr: expr}, nil
	case KindCast:
		p.Advance()
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		cast := casts[strings.ToLower(strings.Trim(tok.Text, "()"))]
		return &CastExpr{Span: p.SpanFrom(tok.Span), Cast: cast, Expr: expr}, nil
	case KindClone:
		p.Advance()
		expr, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Span: p.SpanFrom(tok.Span), Op: KindClone, Expr: expr}, nil
	}
	return p.parsePow()
}

// parsePow handles the right-associative ** operator, whose right side
// admits unary operators: 2 ** -1.
func (p *parser) parsePow() (Expr, error) {
	base, err := p.parsePostfix()
	if err != nil {
		return nil, err
	}
	if p.Match(Pow) {
		exponent, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return binaryExpr(Pow, base, exponent), nil
	}
	return base, nil
}

func (p *parser) parsePostfix() (Expr, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch p.Current().Kind {
		case Arrow, NullsafeArrow:
			nullsafe := p.Advance().Kind == NullsafeArrow
			name, err := p.parseMemberName()
			if err != nil {
				return nil, err
			}
			expr, err = p.parseCallOrFetch(expr, name, nullsafe)
			if err != nil {
				return nil, err
			}
		case DoubleColon:
			expr, err = p.parseStaticAccess(expr)
			if err != nil {
				return nil, err
			}
		case LBracket:
			p.Advance()
			var index Expr
			if !p.Check(RBracket) {
				if index, err = p.parseExpr(); err != nil {
					return nil, err
				}
			}
			if _, err := p.Expect(RBracket, RBracket); err != nil {
				return nil, err
			}
			expr = &IndexExpr{Span: p.SpanFrom(expr.Pos()), Target: expr, Index: index}
		case LParen:
			args, err := p.parseArgs()
			if err != nil {
				return nil, err
			}
			expr = &CallExpr{Span: p.SpanFrom(expr.Pos()), Func: expr, Args: args}
		case Incr, Decr:
			op := p.Advance()
			expr = &UnaryExpr{Span: p.SpanFrom(expr.Pos()), Op: op.Kind, Expr: expr, Post: true}
		default:
			return expr, nil
		}
	}
}

// parseCallOrFetch builds the ->name / ?->name access, distinguishing
// method calls from property fetches by the following parenthesis.
func (p *parser) parseCallOrFetch(target Expr, name Expr, nullsafe bool) (Expr, error) {
	if !p.Check(LParen) {
		if nullsafe {
			return &NullsafePropertyFetch{Span: p.SpanFrom(target.Pos()), Target: target, Name: name}, nil
		}
		return &PropertyFetch{Span: p.SpanFrom(target.Pos()), Target: target, Name: name}, nil
	}
	args, err := p.parseArgs()
	if err != nil {
		return nil, err
	}
	if nullsafe {
		return &NullsafeMethodCall{Span: p.SpanFrom(target.Pos()), Target: target, Method: name, Args: args}, nil
	}
	return &MethodCall{Span: p.SpanFrom(target.Pos()), Target: target, Method: name, Args: args}, nil
}

func (p *parser) parseStaticAccess(class Expr) (Expr, error) {
	p.Advance()
	switch p.Current().Kind {
	case KindVariable:
		prop := variable(p.Advance())
		return &StaticPropertyFetch{Span: p.SpanFrom(class.Pos()), Class: class, Prop: prop}, nil
	case Dollar:
		prop, err := p.parseVariableTarget()
		if err != nil {
			return nil, err
		}
		variableProp, ok := prop.(*Variable)
		if !ok {
			return nil, p.Fail("expected variable after '::'")
		}
		return &StaticPropertyFetch{Span: p.SpanFrom(class.Pos()), Class: class, Prop: variableProp}, nil
	}
	name, err := p.memberName("constant or method name")
	if err != nil {
		return nil, err
	}
	if p.Check(LParen) {
		args, err := p.parseArgs()
		if err != nil {
			return nil, err
		}
		return &StaticCall{Span: p.SpanFrom(class.Pos()), Class: class, Method: name.Text, Args: args}, nil
	}
	return &ClassConstFetch{Span: p.SpanFrom(class.Pos()), Class: class, Const: name.Text}, nil
}

func (p *parser) parseMemberName() (Expr, error) {
	switch p.Current().Kind {
	case KindVariable:
		return variable(p.Advance()), nil
	case Dollar:
		return p.parseVariableTarget()
	case LBrace:
		p.Advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.Expect(RBrace, "'}' after member name"); err != nil {
			return nil, err
		}
		return expr, nil
	}
	tok, err := p.memberName("member name")
	if err != nil {
		return nil, err
	}
	return &Identifier{Span: tok.Span, Name: tok.Text}, nil
}

func (p *parser) parseVariableTarget() (Expr, error) {
	switch p.Current().Kind {
	case KindVariable:
		return variable(p.Advance()), nil
	case Dollar:
		tok := p.Advance()
		if p.Match(LBrace) {
			expr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.Expect(RBrace, "'}' after variable name"); err != nil {
				return nil, err
			}
			return &VariableVariable{Span: p.SpanFrom(tok.Span), Expr: expr}, nil
		}
		inner, err := p.parseVariableTarget()
		if err != nil {
			return nil, err
		}
		return &VariableVariable{Span: p.SpanFrom(tok.Span), Expr: inner}, nil
	}
	return nil, p.Fail("expected variable, found " + gast.Describe(p.Current()))
}

func (p *parser) parseArgs() ([]*Arg, error) {
	if _, err := p.Expect(LParen, LParen); err != nil {
		return nil, err
	}
	var args []*Arg
	for !p.Check(RParen) {
		start := p.Current().Span
		arg := &Arg{Name: p.parseNamedArg()}
		arg.ByRef = p.Match(Ampersand)
		if p.Match(Ellipsis) {
			arg.Spread = true
			if !p.Check(RParen) {
				expr, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				arg.Expr = expr
			}
		} else {
			expr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			arg.Expr = expr
		}
		arg.Span = p.SpanFrom(start)
		args = append(args, arg)
		if !p.Match(Comma) {
			break
		}
	}
	if _, err := p.Expect(RParen, "')' to close argument list"); err != nil {
		return nil, err
	}
	return args, nil
}

// parseNamedArg consumes a PHP 8 named-argument label, `name:`,
// returning the label or an empty string.
func (p *parser) parseNamedArg() string {
	if p.Check(KindIdent) && p.CheckAhead(1, Colon) {
		name := p.Advance().Text
		p.Advance()
		return name
	}
	return ""
}

func (p *parser) parseArrayItems(end string, allowEmpty bool) ([]*ArrayItem, error) {
	var items []*ArrayItem
	for !p.Check(end) {
		if allowEmpty && p.Check(Comma) {
			if p.CheckAhead(1, end) {
				break
			}
			comma := p.Advance()
			items = append(items, &ArrayItem{Span: comma.Span})
			continue
		}
		start := p.Current().Span
		spread := p.Match(Ellipsis)
		byRef := p.Match(Ampersand)
		first, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		item := &ArrayItem{Value: first, ByRef: byRef, Spread: spread}
		if p.Match(DoubleArrow) {
			item.Key = first
			item.ByRef = p.Match(Ampersand)
			if item.Value, err = p.parseExpr(); err != nil {
				return nil, err
			}
		}
		item.Span = p.SpanFrom(start)
		items = append(items, item)
		if !p.Match(Comma) {
			break
		}
	}
	if _, err := p.Expect(end, "'"+end+"' to close array"); err != nil {
		return nil, err
	}
	return items, nil
}

func (p *parser) parsePrimary() (Expr, error) {
	tok := p.Current()
	switch tok.Kind {
	case KindInt:
		p.Advance()
		return &IntegerLit{Span: tok.Span, Text: tok.Text}, nil
	case KindFloat:
		p.Advance()
		return &FloatLit{Span: tok.Span, Text: tok.Text}, nil
	case KindString:
		p.Advance()
		return stringLit(tok), nil
	case KindTrue, KindFalse:
		p.Advance()
		return &BooleanLit{Span: tok.Span, Value: tok.Kind == KindTrue}, nil
	case KindNull:
		p.Advance()
		return &NullLit{Span: tok.Span}, nil
	case KindVariable:
		p.Advance()
		return variable(tok), nil
	case Dollar:
		return p.parseVariableTarget()
	case LParen:
		p.Advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.Expect(RParen, RParen); err != nil {
			return nil, err
		}
		return expr, nil
	case LBracket:
		p.Advance()
		items, err := p.parseArrayItems(RBracket, false)
		if err != nil {
			return nil, err
		}
		return &ArrayExpr{Span: p.SpanFrom(tok.Span), Items: items}, nil
	case KindArray:
		p.Advance()
		if _, err := p.Expect(LParen, "'(' after 'array'"); err != nil {
			return nil, err
		}
		items, err := p.parseArrayItems(RParen, false)
		if err != nil {
			return nil, err
		}
		return &ArrayExpr{Span: p.SpanFrom(tok.Span), Items: items, Legacy: true}, nil
	case KindList:
		p.Advance()
		if _, err := p.Expect(LParen, "'(' after 'list'"); err != nil {
			return nil, err
		}
		items, err := p.parseArrayItems(RParen, true)
		if err != nil {
			return nil, err
		}
		return &ArrayExpr{Span: p.SpanFrom(tok.Span), Items: items, Legacy: true}, nil
	case KindFunction:
		return p.parseClosure(nil, false)
	case KindStatic:
		if ahead := p.Peek(1).Kind; ahead != KindFunction && ahead != KindFn {
			return nil, p.Fail("unexpected 'static' in expression")
		}
		p.Advance()
		if p.Check(KindFunction) {
			return p.parseClosure(nil, true)
		}
		return p.parseArrowFn(nil, true)
	case KindFn:
		return p.parseArrowFn(nil, false)
	case KindNew:
		return p.parseNewExpr()
	case KindMatch:
		return p.parseMatchExpr()
	case KindIsset:
		p.Advance()
		if _, err := p.Expect(LParen, "'(' after 'isset'"); err != nil {
			return nil, err
		}
		args, err := p.parseExprList()
		if err != nil {
			return nil, err
		}
		close, err := p.Expect(RParen, "')' to close isset")
		if err != nil {
			return nil, err
		}
		return &IssetExpr{Span: gast.Merge(tok.Span, close.Span), Args: args}, nil
	case KindEmpty:
		p.Advance()
		if _, err := p.Expect(LParen, "'(' after 'empty'"); err != nil {
			return nil, err
		}
		arg, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		close, err := p.Expect(RParen, "')' to close empty")
		if err != nil {
			return nil, err
		}
		return &EmptyExpr{Span: gast.Merge(tok.Span, close.Span), Arg: arg}, nil
	case KindAttributeOpen:
		attrs, err := p.parseAttributeGroups()
		if err != nil {
			return nil, err
		}
		switch p.Current().Kind {
		case KindFunction:
			return p.parseClosure(attrs, false)
		case KindStatic:
			p.Advance()
			return p.parseClosure(attrs, true)
		case KindFn:
			return p.parseArrowFn(attrs, false)
		}
		return nil, p.Fail("expected closure after attributes")
	}
	if p.isTypeStart() {
		name, err := p.parseName()
		if err != nil {
			return nil, err
		}
		return &ConstFetch{Span: name.Span, Name: name}, nil
	}
	return nil, p.Fail("unexpected " + gast.Describe(tok) + " in expression")
}

func stringLit(tok gast.Token) *StringLit {
	text := tok.Text
	style := "double"
	switch text[0] {
	case '\'':
		style = "single"
	case '`':
		style = "shell"
	case '<':
		style = "heredoc"
	}
	return &StringLit{Span: tok.Span, Kind: style, Text: text, Value: decodeString(text)}
}

// variable converts a variable token into a Variable node; the name
// excludes the $.
func variable(tok gast.Token) *Variable {
	return &Variable{Span: tok.Span, Name: tok.Text[1:]}
}

func (p *parser) parseClosure(attrs []*AttributeGroup, isStatic bool) (Expr, error) {
	fnTok := p.Advance()
	node := &ClosureExpr{Attrs: attrs, Static: isStatic}
	node.Ref = p.Match(Ampersand)
	var err error
	if node.Params, err = p.parseParams(); err != nil {
		return nil, err
	}
	if p.Match(KindUse) {
		if _, err := p.Expect(LParen, "'(' after 'use'"); err != nil {
			return nil, err
		}
		for !p.Check(RParen) {
			use := ClosureUse{ByRef: p.Match(Ampersand)}
			v, err := p.parseVariableTarget()
			if err != nil {
				return nil, err
			}
			variableUse, ok := v.(*Variable)
			if !ok {
				return nil, p.Fail("expected variable in closure use list")
			}
			use.Var = variableUse
			use.Span = variableUse.Span
			node.Uses = append(node.Uses, use)
			if !p.Match(Comma) {
				break
			}
		}
		if _, err := p.Expect(RParen, "')' to close closure use list"); err != nil {
			return nil, err
		}
	}
	if p.Match(Colon) {
		if node.ReturnType, err = p.parseType(); err != nil {
			return nil, err
		}
	}
	if node.Body, err = p.parseBlockStmt(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(fnTok.Span)
	return node, nil
}

func (p *parser) parseArrowFn(attrs []*AttributeGroup, isStatic bool) (Expr, error) {
	fnTok := p.Advance()
	node := &ArrowFnExpr{Attrs: attrs, Static: isStatic}
	node.Ref = p.Match(Ampersand)
	var err error
	if node.Params, err = p.parseParams(); err != nil {
		return nil, err
	}
	if p.Match(Colon) {
		if node.ReturnType, err = p.parseType(); err != nil {
			return nil, err
		}
	}
	if _, err := p.Expect(DoubleArrow, "'=>' in arrow function"); err != nil {
		return nil, err
	}
	if node.Expr, err = p.parseExpr(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(fnTok.Span)
	return node, nil
}

// parseNewTarget parses the class reference of new and instanceof: a
// name, a variable (possibly dereferenced), or a parenthesized
// expression. Call parentheses belong to the new, not the target.
func (p *parser) parseNewTarget() (Expr, error) {
	switch p.Current().Kind {
	case LParen:
		p.Advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.Expect(RParen, RParen); err != nil {
			return nil, err
		}
		return p.parseTargetDeref(expr)
	case KindVariable, Dollar:
		expr, err := p.parseVariableTarget()
		if err != nil {
			return nil, err
		}
		return p.parseTargetDeref(expr)
	}
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	return &ConstFetch{Span: name.Span, Name: name}, nil
}

func (p *parser) parseTargetDeref(expr Expr) (Expr, error) {
	start := expr.Pos()
	for {
		switch p.Current().Kind {
		case Arrow:
			p.Advance()
			name, err := p.parseMemberName()
			if err != nil {
				return nil, err
			}
			expr = &PropertyFetch{Span: p.SpanFrom(start), Target: expr, Name: name}
		case LBracket:
			p.Advance()
			var index Expr
			if !p.Check(RBracket) {
				inner, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				index = inner
			}
			if _, err := p.Expect(RBracket, RBracket); err != nil {
				return nil, err
			}
			expr = &IndexExpr{Span: p.SpanFrom(start), Target: expr, Index: index}
		case DoubleColon:
			return p.parseStaticAccess(expr)
		default:
			return expr, nil
		}
	}
}

func (p *parser) parseNewExpr() (Expr, error) {
	newTok := p.Advance()
	if p.Check(KindClass) || p.isContextualIdent(wordReadonly) {
		return p.parseAnonymousClass(newTok)
	}
	class, err := p.parseNewTarget()
	if err != nil {
		return nil, err
	}
	node := &NewExpr{Class: class}
	if p.Check(LParen) {
		if node.Args, err = p.parseArgs(); err != nil {
			return nil, err
		}
	}
	node.Span = p.SpanFrom(newTok.Span)
	return node, nil
}

func (p *parser) parseAnonymousClass(newTok gast.Token) (Expr, error) {
	node := &AnonymousClassExpr{}
	for p.isContextualIdent(wordReadonly) {
		node.Modifiers = append(node.Modifiers, wordReadonly)
		p.Advance()
	}
	if _, err := p.Expect(KindClass, "'class' after 'new'"); err != nil {
		return nil, err
	}
	var err error
	if p.Check(LParen) {
		if node.Args, err = p.parseArgs(); err != nil {
			return nil, err
		}
	}
	if p.Match(KindExtends) {
		var extends Name
		if extends, err = p.parseName(); err != nil {
			return nil, err
		}
		node.Extends = &extends
	}
	if node.Implements, err = p.parseNameListAfter(KindImplements); err != nil {
		return nil, err
	}
	if node.Members, err = p.parseMemberBlock(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(newTok.Span)
	return node, nil
}

func (p *parser) parseMatchExpr() (Expr, error) {
	matchTok := p.Advance()
	cond, err := p.parenExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(LBrace, "'{' to open match arms"); err != nil {
		return nil, err
	}
	node := &MatchExpr{Cond: cond}
	for !p.Check(RBrace) {
		arm := MatchArm{}
		if p.Match(KindDefault) {
			if _, err := p.Expect(DoubleArrow, "'=>' after 'default'"); err != nil {
				return nil, err
			}
		} else {
			if arm.Conds, err = p.parseExprList(); err != nil {
				return nil, err
			}
			if _, err := p.Expect(DoubleArrow, "'=>' after match condition"); err != nil {
				return nil, err
			}
		}
		if arm.Body, err = p.parseExpr(); err != nil {
			return nil, err
		}
		node.Arms = append(node.Arms, arm)
		if !p.Match(Comma) {
			break
		}
	}
	if _, err := p.Expect(RBrace, "'}' to close match arms"); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(matchTok.Span)
	return node, nil
}

// parseName parses a possibly qualified identifier. self, parent,
// static, array, callable, true, false, null, and namespace are
// accepted as name parts because PHP uses them as type and constant
// names.
func (p *parser) parseName() (Name, error) {
	start := p.Current().Span
	qualified := p.Match(Backslash)
	var parts []string
	for {
		tok := p.Current()
		switch tok.Kind {
		case KindIdent, KindArray, KindCallable, KindStatic, KindTrue, KindFalse, KindNull, KindNamespace:
			parts = append(parts, tok.Text)
		default:
			if len(parts) == 0 {
				return Name{}, p.Fail("expected name, found " + gast.Describe(tok))
			}
			return Name{}, p.Fail("expected name after '\\', found " + gast.Describe(tok))
		}
		p.Advance()
		if !p.Match(Backslash) {
			break
		}
	}
	return Name{Span: p.SpanFrom(start), Qualified: qualified, Parts: parts}, nil
}

func (p *parser) parseNameList() ([]Name, error) {
	return p.parseNameListWith(Comma)
}

func (p *parser) parseNameListWith(sep string) ([]Name, error) {
	var names []Name
	for {
		name, err := p.parseName()
		if err != nil {
			return nil, err
		}
		names = append(names, name)
		if !p.Match(sep) {
			return names, nil
		}
	}
}

// isTypeStart reports whether a type can begin at the current token.
func (p *parser) isTypeStart() bool { return p.isTypeStartAt(0) }

func (p *parser) isTypeStartAt(ahead int) bool {
	switch p.Peek(ahead).Kind {
	case KindIdent, KindArray, KindCallable, KindStatic, KindTrue, KindFalse, KindNull, KindNamespace, Backslash, Question:
		return true
	}
	return false
}

func (p *parser) parseType() (*Type, error) {
	start := p.Current().Span
	node := &Type{Nullable: p.Match(Question)}
	first, err := p.parseTypeName()
	if err != nil {
		return nil, err
	}
	node.Names = append(node.Names, first)
	for {
		var separator byte
		if p.Match(Pipe) {
			separator = '|'
		} else if p.Check(Ampersand) && p.Peek(1).Kind != KindVariable {
			p.Advance()
			separator = '&'
		} else {
			break
		}
		node.Separator = separator
		name, err := p.parseTypeName()
		if err != nil {
			return nil, err
		}
		node.Names = append(node.Names, name)
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// parseTypeName parses one member of a type: a name or a parenthesized
// intersection as used in DNF types.
func (p *parser) parseTypeName() (Name, error) {
	if !p.Match(LParen) {
		return p.parseName()
	}
	start := p.Current().Span
	var parts []string
	for {
		name, err := p.parseName()
		if err != nil {
			return Name{}, err
		}
		parts = append(parts, name.Parts...)
		if !p.Match(Ampersand) {
			break
		}
	}
	if _, err := p.Expect(RParen, "')' to close intersection type"); err != nil {
		return Name{}, err
	}
	return Name{Span: p.SpanFrom(start), Parts: parts}, nil
}

func (p *parser) parseAttributeGroups() ([]*AttributeGroup, error) {
	var groups []*AttributeGroup
	for p.Check(KindAttributeOpen) {
		open := p.Advance()
		group := &AttributeGroup{}
		for !p.Check(RBracket) {
			name, err := p.parseName()
			if err != nil {
				return nil, err
			}
			attribute := &Attribute{Name: name}
			if p.Check(LParen) {
				if attribute.Args, err = p.parseArgs(); err != nil {
					return nil, err
				}
			}
			attribute.Span = p.SpanFrom(name.Span)
			group.Attrs = append(group.Attrs, attribute)
			if !p.Match(Comma) {
				break
			}
		}
		if _, err := p.Expect(RBracket, "']' to close attribute group"); err != nil {
			return nil, err
		}
		group.Span = p.SpanFrom(open.Span)
		groups = append(groups, group)
	}
	return groups, nil
}
