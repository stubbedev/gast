package python

import (
	"slices"

	"github.com/stubbedev/gast/pkg/gast"
)

// parser is a recursive-descent parser over the Python token stream.
type parser struct {
	*gast.TokenCursor
	source string
}

func newParser(tokens []gast.Token, source string) *parser {
	if len(tokens) > 0 && tokens[len(tokens)-1].Kind == KindEOF {
		tokens = tokens[:len(tokens)-1]
	}
	return &parser{TokenCursor: gast.NewTokenCursor(tokens, KindEOF), source: source}
}

// parseModule parses statements until end of input.
func (p *parser) parseModule() (*File, error) {
	stmts, err := p.parseStmtList()
	if err != nil {
		return nil, err
	}
	file := &File{Stmts: stmts}
	file.Span = gast.Span{Start: 0, End: len(p.source)}
	return file, nil
}

// parseStmtList parses statements until EOF or one of terminators is
// current (the terminator is left for the caller).
func (p *parser) parseStmtList(terminators ...string) ([]Stmt, error) {
	var stmts []Stmt
	for !p.AtEnd() && !slices.Contains(terminators, p.Current().Kind) {
		if p.Match(KindNewline) {
			continue
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, stmt)
	}
	return stmts, nil
}

// compoundStartKinds are the tokens that begin a compound statement;
// they are rejected in an inline suite.
var compoundStartKinds = []string{
	KindIf, KindWhile, KindFor, KindTry, KindWith, KindDef, KindClass, KindAsync, At,
}

func (p *parser) parseStatement() (Stmt, error) {
	switch p.Current().Kind {
	case KindIf:
		return p.parseIfStmt()
	case KindWhile:
		return p.parseWhileStmt()
	case KindFor:
		return p.parseForStmt(false)
	case KindTry:
		return p.parseTryStmt()
	case KindWith:
		return p.parseWithStmt(false)
	case KindDef:
		return p.parseFunctionDef(nil, false)
	case KindClass:
		return p.parseClassDef(nil)
	case At:
		return p.parseDecorated()
	case KindAsync:
		switch p.Peek(1).Kind {
		case KindDef:
			p.Advance()
			return p.parseFunctionDef(nil, true)
		case KindFor:
			p.Advance()
			return p.parseForStmt(true)
		case KindWith:
			p.Advance()
			return p.parseWithStmt(true)
		}
		return nil, p.Fail("expected 'def', 'for', or 'with' after 'async'")
	}
	if p.isMatchStmt() {
		return p.parseMatchStmt()
	}
	return p.parseSimpleLine()
}

// parseSimpleLine parses one full logical line of simple statements,
// including its terminating NEWLINE.
func (p *parser) parseSimpleLine() (Stmt, error) {
	start := p.Current().Span
	var stmts []Stmt
	for {
		stmt, err := p.parseSimpleStmt()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, stmt)
		if !p.Match(Semicolon) {
			break
		}
		if p.Check(KindNewline) || p.AtEnd() {
			break
		}
	}
	if err := p.endLine(); err != nil {
		return nil, err
	}
	if len(stmts) == 1 {
		return stmts[0], nil
	}
	seq := &SeqStmt{Stmts: stmts}
	seq.Span = p.SpanFrom(start)
	return seq, nil
}

func (p *parser) endLine() error {
	if p.Match(KindNewline) || p.AtEnd() {
		return nil
	}
	return p.Fail("expected newline, found " + gast.Describe(p.Current()))
}

func (p *parser) parseSimpleStmt() (Stmt, error) {
	start := p.Current().Span
	switch p.Current().Kind {
	case KindPass, KindBreak, KindContinue:
		tok := p.Advance()
		switch tok.Kind {
		case KindPass:
			return &PassStmt{Span: tok.Span}, nil
		case KindBreak:
			return &BreakStmt{Span: tok.Span}, nil
		default:
			return &ContinueStmt{Span: tok.Span}, nil
		}
	case KindReturn:
		p.Advance()
		var value Expr
		var err error
		if !p.atLineEnd() {
			if value, err = p.parseExprList(); err != nil {
				return nil, err
			}
		}
		return &ReturnStmt{Span: p.SpanFrom(start), Value: value}, nil
	case KindRaise:
		p.Advance()
		var exc, from Expr
		var err error
		if !p.atLineEnd() {
			if exc, err = p.parseTest(); err != nil {
				return nil, err
			}
			if p.Match(KindFrom) {
				if from, err = p.parseTest(); err != nil {
					return nil, err
				}
			}
		}
		return &RaiseStmt{Span: p.SpanFrom(start), Exc: exc, From: from}, nil
	case KindDel:
		p.Advance()
		targets, err := p.parseExprList()
		if err != nil {
			return nil, err
		}
		if tuple, ok := targets.(*TupleExpr); ok {
			for _, target := range tuple.Elts {
				if err := p.checkDelTarget(target); err != nil {
					return nil, err
				}
			}
			return &DelStmt{Span: p.SpanFrom(start), Targets: tuple.Elts}, nil
		}
		if err := p.checkDelTarget(targets); err != nil {
			return nil, err
		}
		return &DelStmt{Span: p.SpanFrom(start), Targets: []Expr{targets}}, nil
	case KindAssert:
		p.Advance()
		test, err := p.parseTest()
		if err != nil {
			return nil, err
		}
		var msg Expr
		if p.Match(Comma) {
			if msg, err = p.parseTest(); err != nil {
				return nil, err
			}
		}
		return &AssertStmt{Span: p.SpanFrom(start), Test: test, Msg: msg}, nil
	case KindGlobal, KindNonlocal:
		kw := p.Advance()
		var names []string
		for {
			name, err := p.Expect(KindName, "name")
			if err != nil {
				return nil, err
			}
			names = append(names, name.Text)
			if !p.Match(Comma) {
				break
			}
		}
		if kw.Kind == KindGlobal {
			return &GlobalStmt{Span: p.SpanFrom(start), Names: names}, nil
		}
		return &NonlocalStmt{Span: p.SpanFrom(start), Names: names}, nil
	case KindImport:
		p.Advance()
		names, err := p.parseAliasList()
		if err != nil {
			return nil, err
		}
		return &ImportStmt{Span: p.SpanFrom(start), Names: names}, nil
	case KindFrom:
		return p.parseImportFromStmt()
	}
	if p.isTypeAliasStmt() {
		return p.parseTypeAliasStmt()
	}
	return p.parseAssignLine()
}

func (p *parser) atLineEnd() bool {
	return p.Check(KindNewline) || p.Check(Semicolon) || p.AtEnd()
}

func (p *parser) parseAliasList() ([]Alias, error) {
	var names []Alias
	for {
		name, err := p.parseDottedName()
		if err != nil {
			return nil, err
		}
		alias := Alias{Span: name.Span, Name: name.Text}
		if p.Match(KindAs) {
			asName, err := p.Expect(KindName, "alias name")
			if err != nil {
				return nil, err
			}
			alias.AsName = asName.Text
			alias.Span = gast.Merge(alias.Span, asName.Span)
		}
		names = append(names, alias)
		if !p.Match(Comma) {
			return names, nil
		}
	}
}

// dottedName parses a module path such as os.path.
func (p *parser) parseDottedName() (gast.Token, error) {
	name, err := p.Expect(KindName, "module name")
	if err != nil {
		return gast.Token{}, err
	}
	for p.Match(Dot) {
		part, err := p.Expect(KindName, "module name after '.'")
		if err != nil {
			return gast.Token{}, err
		}
		name.Text += "." + part.Text
		name.Span = gast.Merge(name.Span, part.Span)
	}
	return name, nil
}

func (p *parser) parseImportFromStmt() (Stmt, error) {
	start := p.Current().Span
	p.Advance()
	dots := 0
	for p.Check(Dot) || p.Check(Ellipsis) {
		dots += len(p.Current().Text)
		p.Advance()
	}
	module := ""
	if p.Check(KindName) {
		name, err := p.parseDottedName()
		if err != nil {
			return nil, err
		}
		module = name.Text
	}
	if _, err := p.Expect(KindImport, "'import'"); err != nil {
		return nil, err
	}
	stmt := &ImportFromStmt{Dots: dots, Module: module}
	if p.Match(Star) {
		stmt.Star = true
	} else if p.Match(LParen) {
		for !p.Check(RParen) && !p.AtEnd() {
			alias, err := p.parseImportAlias()
			if err != nil {
				return nil, err
			}
			stmt.Names = append(stmt.Names, alias)
			if !p.Match(Comma) {
				break
			}
		}
		if _, err := p.Expect(RParen, "')' to close import list"); err != nil {
			return nil, err
		}
	} else {
		for {
			alias, err := p.parseImportAlias()
			if err != nil {
				return nil, err
			}
			stmt.Names = append(stmt.Names, alias)
			if !p.Match(Comma) {
				break
			}
		}
	}
	stmt.Span = p.SpanFrom(start)
	return stmt, nil
}

// parseImportAlias parses one imported name with an optional alias.
func (p *parser) parseImportAlias() (Alias, error) {
	name, err := p.Expect(KindName, "imported name")
	if err != nil {
		return Alias{}, err
	}
	alias := Alias{Span: name.Span, Name: name.Text}
	if p.Match(KindAs) {
		asName, err := p.Expect(KindName, "alias name")
		if err != nil {
			return Alias{}, err
		}
		alias.AsName = asName.Text
		alias.Span = gast.Merge(alias.Span, asName.Span)
	}
	return alias, nil
}

// parseBlock parses a suite: either an indented block or simple
// statements on the header line.
func (p *parser) parseBlock() ([]Stmt, error) {
	if p.Match(KindNewline) {
		if _, err := p.Expect(KindIndent, "an indented block"); err != nil {
			return nil, err
		}
		stmts, err := p.parseStmtList(KindDedent)
		if err != nil {
			return nil, err
		}
		p.Match(KindDedent)
		return stmts, nil
	}
	if slices.Contains(compoundStartKinds, p.Current().Kind) {
		return nil, p.Fail("expected a simple statement on this line, found " + gast.Describe(p.Current()))
	}
	stmt, err := p.parseSimpleLine()
	if err != nil {
		return nil, err
	}
	return []Stmt{stmt}, nil
}

func (p *parser) parseIfStmt() (Stmt, error) {
	return p.parseIfTail(p.Advance())
}

func (p *parser) parseIfTail(ifTok gast.Token) (Stmt, error) {
	node := &IfStmt{}
	var err error
	if node.Cond, err = p.parseNamedExpr(); err != nil {
		return nil, err
	}
	if _, err := p.Expect(Colon, "':' after if condition"); err != nil {
		return nil, err
	}
	if node.Body, err = p.parseBlock(); err != nil {
		return nil, err
	}
	elifTok := p.Current()
	if p.Match(KindElif) {
		elif, err := p.parseIfTail(elifTok)
		if err != nil {
			return nil, err
		}
		node.Orelse = []Stmt{elif}
	} else if p.Match(KindElse) {
		if _, err := p.Expect(Colon, "':' after 'else'"); err != nil {
			return nil, err
		}
		if node.Orelse, err = p.parseBlock(); err != nil {
			return nil, err
		}
	}
	node.Span = p.SpanFrom(ifTok.Span)
	return node, nil
}

func (p *parser) parseWhileStmt() (Stmt, error) {
	start := p.Current().Span
	p.Advance()
	node := &WhileStmt{}
	var err error
	if node.Cond, err = p.parseNamedExpr(); err != nil {
		return nil, err
	}
	if _, err := p.Expect(Colon, "':' after while condition"); err != nil {
		return nil, err
	}
	if node.Body, err = p.parseBlock(); err != nil {
		return nil, err
	}
	if p.Match(KindElse) {
		if _, err := p.Expect(Colon, "':' after 'else'"); err != nil {
			return nil, err
		}
		if node.Orelse, err = p.parseBlock(); err != nil {
			return nil, err
		}
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

func (p *parser) parseForStmt(async bool) (Stmt, error) {
	start := p.Current().Span
	p.Advance()
	node := &ForStmt{Async: async}
	var err error
	if node.Target, err = p.parseTargetList(); err != nil {
		return nil, err
	}
	if _, err := p.Expect(KindIn, "'in' after for target"); err != nil {
		return nil, err
	}
	if node.Iter, err = p.parseExprList(); err != nil {
		return nil, err
	}
	if _, err := p.Expect(Colon, "':' after for header"); err != nil {
		return nil, err
	}
	if node.Body, err = p.parseBlock(); err != nil {
		return nil, err
	}
	if p.Match(KindElse) {
		if _, err := p.Expect(Colon, "':' after 'else'"); err != nil {
			return nil, err
		}
		if node.Orelse, err = p.parseBlock(); err != nil {
			return nil, err
		}
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

func (p *parser) parseTryStmt() (Stmt, error) {
	start := p.Current().Span
	p.Advance()
	node := &TryStmt{}
	var err error
	if _, err := p.Expect(Colon, "':' after try"); err != nil {
		return nil, err
	}
	if node.Body, err = p.parseBlock(); err != nil {
		return nil, err
	}
	for p.Check(KindExcept) {
		handlerTok := p.Advance()
		handler := &ExceptHandler{Star: p.Match(Star)}
		if !p.Check(Colon) {
			if handler.Type, err = p.parseTest(); err != nil {
				return nil, err
			}
			if p.Match(KindAs) {
				name, err := p.Expect(KindName, "exception name")
				if err != nil {
					return nil, err
				}
				handler.Name = name.Text
			}
		}
		if _, err := p.Expect(Colon, "':' after except clause"); err != nil {
			return nil, err
		}
		if handler.Body, err = p.parseBlock(); err != nil {
			return nil, err
		}
		handler.Span = p.SpanFrom(handlerTok.Span)
		node.Handlers = append(node.Handlers, handler)
	}
	if p.Match(KindElse) {
		if _, err := p.Expect(Colon, "':' after 'else'"); err != nil {
			return nil, err
		}
		if node.Orelse, err = p.parseBlock(); err != nil {
			return nil, err
		}
	}
	if p.Match(KindFinally) {
		if _, err := p.Expect(Colon, "':' after 'finally'"); err != nil {
			return nil, err
		}
		if node.Finalbody, err = p.parseBlock(); err != nil {
			return nil, err
		}
	}
	if len(node.Handlers) == 0 && node.Finalbody == nil {
		return nil, p.Fail("expected 'except' or 'finally' block")
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

func (p *parser) parseWithStmt(async bool) (Stmt, error) {
	start := p.Current().Span
	p.Advance()
	node := &WithStmt{Async: async}
	var err error
	if p.Check(LParen) && p.parenItemsFollow() {
		if err := p.parseParenWithItems(node); err != nil {
			return nil, err
		}
	} else {
		for {
			item, err := p.parseWithItem()
			if err != nil {
				return nil, err
			}
			node.Items = append(node.Items, item)
			if !p.Match(Comma) {
				break
			}
		}
	}
	if _, err := p.Expect(Colon, "':' after with items"); err != nil {
		return nil, err
	}
	if node.Body, err = p.parseBlock(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// parenItemsFollow reports whether a '(' directly after `with` opens a
// parenthesized item list: the token after the matching ')' is the
// suite colon.
func (p *parser) parenItemsFollow() bool {
	depth := 0
	for i := 1; ; i++ {
		switch p.Peek(i).Kind {
		case LParen, LBracket, LBrace:
			depth++
		case RParen, RBracket, RBrace:
			depth--
			if depth < 0 {
				return p.Peek(i+1).Kind == Colon
			}
		case KindNewline, KindEOF:
			return false
		}
	}
}

// parseParenWithItems parses `(item, item, ...)` context managers.
func (p *parser) parseParenWithItems(node *WithStmt) error {
	p.Advance()
	for !p.Check(RParen) && !p.AtEnd() {
		item, err := p.parseWithItem()
		if err != nil {
			return err
		}
		node.Items = append(node.Items, item)
		if !p.Match(Comma) {
			break
		}
	}
	_, err := p.Expect(RParen, "')' to close with items")
	return err
}

func (p *parser) parseWithItem() (*WithItem, error) {
	item := &WithItem{}
	var err error
	if item.Context, err = p.parseTest(); err != nil {
		return nil, err
	}
	if p.Match(KindAs) {
		if item.OptionalVars, err = p.parseTargetExpr(); err != nil {
			return nil, err
		}
	}
	item.Span = item.Context.Pos()
	if item.OptionalVars != nil {
		item.Span = gast.Merge(item.Span, item.OptionalVars.Pos())
	}
	return item, nil
}

func (p *parser) parseDecorated() (Stmt, error) {
	var decorators []Decorator
	for p.Check(At) {
		at := p.Advance().Span
		expr, err := p.parseNamedExpr()
		if err != nil {
			return nil, err
		}
		decorators = append(decorators, Decorator{Span: gast.Merge(at, expr.Pos()), Expr: expr})
		if err := p.endLine(); err != nil {
			return nil, err
		}
	}
	switch p.Current().Kind {
	case KindDef:
		return p.parseFunctionDef(decorators, false)
	case KindClass:
		return p.parseClassDef(decorators)
	case KindAsync:
		if p.Peek(1).Kind == KindDef {
			p.Advance()
			return p.parseFunctionDef(decorators, true)
		}
	}
	return nil, p.Fail("expected 'def', 'class', or 'async def' after decorator")
}

func (p *parser) parseFunctionDef(decorators []Decorator, async bool) (Stmt, error) {
	start := p.Current().Span
	p.Advance()
	node := &FunctionDef{Decorators: decorators, Async: async}
	var err error
	name, err := p.Expect(KindName, "function name")
	if err != nil {
		return nil, err
	}
	node.Name = name.Text
	if p.Check(LBracket) {
		if node.TypeParams, err = p.parseTypeParams(); err != nil {
			return nil, err
		}
	}
	if _, err := p.Expect(LParen, "'(' after function name"); err != nil {
		return nil, err
	}
	if node.Params, err = p.parseParameters(true, RParen); err != nil {
		return nil, err
	}
	if _, err := p.Expect(RParen, "')' to close parameters"); err != nil {
		return nil, err
	}
	if p.Match(Arrow) {
		if node.Returns, err = p.parseTest(); err != nil {
			return nil, err
		}
	}
	if _, err := p.Expect(Colon, "':' after function header"); err != nil {
		return nil, err
	}
	if node.Body, err = p.parseBlock(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

func (p *parser) parseClassDef(decorators []Decorator) (Stmt, error) {
	start := p.Current().Span
	p.Advance()
	node := &ClassDef{Decorators: decorators}
	var err error
	name, err := p.Expect(KindName, "class name")
	if err != nil {
		return nil, err
	}
	node.Name = name.Text
	if p.Check(LBracket) {
		if node.TypeParams, err = p.parseTypeParams(); err != nil {
			return nil, err
		}
	}
	if p.Match(LParen) {
		var args []*Arg
		if args, _, err = p.parseCallArgs(); err != nil {
			return nil, err
		}
		for _, arg := range args {
			if arg.Name != "" {
				node.Keywords = append(node.Keywords, Keyword{Span: arg.Span, Name: arg.Name, Value: arg.Expr})
				continue
			}
			node.Bases = append(node.Bases, arg.Expr)
		}
	}
	if _, err := p.Expect(Colon, "':' after class header"); err != nil {
		return nil, err
	}
	if node.Body, err = p.parseBlock(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// parseTypeParams parses a PEP 695 `[T, U: Bound, V = Default]` list.
func (p *parser) parseTypeParams() ([]*TypeParam, error) {
	p.Advance()
	var params []*TypeParam
	for !p.Check(RBracket) && !p.AtEnd() {
		name, err := p.Expect(KindName, "type parameter name")
		if err != nil {
			return nil, err
		}
		param := &TypeParam{Span: name.Span, Name: name.Text}
		if p.Match(Colon) {
			if param.Bound, err = p.parseTest(); err != nil {
				return nil, err
			}
		}
		if p.Match(Assign) {
			if param.Default, err = p.parseTest(); err != nil {
				return nil, err
			}
		}
		param.Span = p.SpanFrom(name.Span)
		params = append(params, param)
		if !p.Match(Comma) {
			break
		}
	}
	if _, err := p.Expect(RBracket, "']' to close type parameters"); err != nil {
		return nil, err
	}
	return params, nil
}

func (p *parser) isTypeAliasStmt() bool {
	return p.Check(KindName) && p.Current().Text == wordType &&
		p.Peek(1).Kind == KindName &&
		(p.Peek(2).Kind == Assign || p.Peek(2).Kind == LBracket)
}

func (p *parser) parseTypeAliasStmt() (Stmt, error) {
	start := p.Current().Span
	p.Advance()
	node := &TypeAliasStmt{}
	var err error
	name, err := p.Expect(KindName, "type alias name")
	if err != nil {
		return nil, err
	}
	node.Name = &NameExpr{Span: name.Span, ID: name.Text}
	if p.Check(LBracket) {
		if node.TypeParams, err = p.parseTypeParams(); err != nil {
			return nil, err
		}
	}
	if _, err := p.Expect(Assign, "'=' in type alias"); err != nil {
		return nil, err
	}
	if node.Value, err = p.parseTest(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// isMatchStmt reports whether the current `match` name starts a match
// statement: a ':' ends the header line before any newline does.
func (p *parser) isMatchStmt() bool {
	if !p.Check(KindName) || p.Current().Text != wordMatch {
		return false
	}
	depth := 0
	for i := 1; ; i++ {
		switch p.Peek(i).Kind {
		case LParen, LBracket, LBrace:
			depth++
		case RParen, RBracket, RBrace:
			depth--
		case Colon:
			if depth == 0 && i > 1 {
				return true
			}
		case KindNewline, KindEOF:
			return false
		}
	}
}

func (p *parser) parseMatchStmt() (Stmt, error) {
	start := p.Current().Span
	p.Advance()
	node := &MatchStmt{}
	var err error
	if node.Subject, err = p.parseExprList(); err != nil {
		return nil, err
	}
	if _, err := p.Expect(Colon, "':' after match subject"); err != nil {
		return nil, err
	}
	if _, err := p.Expect(KindNewline, "newline after match subject"); err != nil {
		return nil, err
	}
	if _, err := p.Expect(KindIndent, "an indented block of case clauses"); err != nil {
		return nil, err
	}
	for p.isCaseStmt() {
		caseStmt, err := p.parseCaseStmt()
		if err != nil {
			return nil, err
		}
		node.Cases = append(node.Cases, caseStmt)
	}
	p.Match(KindDedent)
	if len(node.Cases) == 0 {
		return nil, p.FailAt(p.SpanFrom(start), "expected at least one 'case' clause")
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

func (p *parser) isCaseStmt() bool {
	return p.Check(KindName) && p.Current().Text == wordCase
}

func (p *parser) parseCaseStmt() (*MatchCase, error) {
	start := p.Current().Span
	p.Advance()
	node := &MatchCase{}
	var err error
	if node.Pattern, err = p.parsePattern(); err != nil {
		return nil, err
	}
	if p.Match(KindIf) {
		if node.Guard, err = p.parseNamedExpr(); err != nil {
			return nil, err
		}
	}
	if _, err := p.Expect(Colon, "':' after case pattern"); err != nil {
		return nil, err
	}
	if node.Body, err = p.parseBlock(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}
