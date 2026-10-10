package php

import (
	"slices"
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

type parser struct {
	*gast.TokenCursor
	source string
	scope  int // nesting depth of statement lists; 0 is the file's top level
}

func newParser(tokens []gast.Token, source string) *parser {
	if len(tokens) > 0 && tokens[len(tokens)-1].Kind == KindEOF {
		tokens = tokens[:len(tokens)-1]
	}
	return &parser{TokenCursor: gast.NewTokenCursor(tokens, KindEOF), source: source}
}

func (p *parser) parseFile() (*File, error) {
	stmts, err := p.parseStmtList()
	if err != nil {
		return nil, err
	}
	file := &File{Stmts: stmts}
	file.Span = gast.Span{Start: 0, End: len(p.source)}
	return file, nil
}

// parseStmtList parses statements until EOF or one of terminators is
// current (the terminator is left for the caller). Tag switches
// (?> html <?php) become inline HTML statements inside the list. A list
// with terminators is a nested scope: __halt_compiler() is only legal
// at the file's top level.
func (p *parser) parseStmtList(terminators ...string) ([]Stmt, error) {
	var stmts []Stmt
	if len(terminators) > 0 {
		p.scope++
		defer func() { p.scope-- }()
	}
	for !p.AtEnd() {
		if p.Match(KindOpenTag) {
			continue
		}
		if slices.Contains(terminators, p.Current().Kind) {
			return stmts, nil
		}
		if p.Check(KindCloseTag) {
			more, html, err := p.parseTagSwitch()
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, html...)
			if !more {
				return stmts, nil
			}
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

// parseTagSwitch handles a close tag: any following inline HTML becomes
// a statement, and an open tag resumes the statement list. A short echo
// tag emits its echo statement here. The bool reports whether the list
// continues.
func (p *parser) parseTagSwitch() (bool, []Stmt, error) {
	p.Advance()
	var stmts []Stmt
	if p.Check(KindInlineHTML) {
		html := p.Advance()
		stmts = append(stmts, &InlineHTMLStmt{Span: html.Span, Text: html.Text})
	}
	if p.AtEnd() || p.Match(KindOpenTag) {
		return !p.AtEnd(), stmts, nil
	}
	if p.Check(KindOpenTagEcho) {
		echo := p.Advance()
		args, err := p.parseExprList()
		if err != nil {
			return false, nil, err
		}
		if err := p.endStatement(); err != nil {
			return false, nil, err
		}
		stmts = append(stmts, &EchoStmt{Span: p.SpanFrom(echo.Span), Args: args})
		return true, stmts, nil
	}
	return false, nil, p.Fail("unexpected " + gast.Describe(p.Current()))
}

func (p *parser) parseStatement() (Stmt, error) {
	switch p.Current().Kind {
	case KindInlineHTML:
		tok := p.Advance()
		return &InlineHTMLStmt{Span: tok.Span, Text: tok.Text}, nil
	case KindOpenTagEcho:
		echo := p.Advance()
		args, err := p.parseExprList()
		if err != nil {
			return nil, err
		}
		if err := p.endStatement(); err != nil {
			return nil, err
		}
		return &EchoStmt{Span: p.SpanFrom(echo.Span), Args: args}, nil
	case LBrace:
		return p.parseBlockStmt()
	case Semicolon:
		tok := p.Advance()
		return &NopStmt{Span: tok.Span}, nil
	case KindIf:
		return p.parseIfStmt()
	case KindWhile:
		return p.parseWhileStmt()
	case KindDo:
		return p.parseDoWhileStmt()
	case KindFor:
		return p.parseForStmt()
	case KindForeach:
		return p.parseForeachStmt()
	case KindSwitch:
		return p.parseSwitchStmt()
	case KindBreak, KindContinue:
		return p.parseBreakContinueStmt()
	case KindReturn:
		return p.parseReturnStmt()
	case KindGoto:
		gotoTok := p.Advance()
		label, err := p.Expect(KindIdent, "label")
		if err != nil {
			return nil, err
		}
		if err := p.endStatement(); err != nil {
			return nil, err
		}
		return &GotoStmt{Span: p.SpanFrom(gotoTok.Span), Label: label.Text}, nil
	case KindTry:
		return p.parseTryStmt()
	case KindDeclare:
		return p.parseDeclareStmt()
	case KindFunction:
		if p.isFunctionDecl() {
			return p.parseFunctionStmt(nil)
		}
	case KindAbstract, KindFinal:
		return p.parseClassStmt(nil)
	case KindClass:
		return p.parseClassStmt(nil)
	case KindInterface:
		return p.parseInterfaceStmt()
	case KindTrait:
		return p.parseTraitStmt()
	case KindNamespace:
		return p.parseNamespaceStmt()
	case KindUse:
		return p.parseUseStmt()
	case KindConst:
		return p.parseConstStmt()
	case KindEcho:
		return p.parseEchoStmt()
	case KindGlobal:
		return p.parseGlobalStmt()
	case KindUnset:
		return p.parseUnsetStmt()
	case KindHaltCompiler:
		return p.parseHaltCompilerStmt()
	case KindStatic:
		switch p.Peek(1).Kind {
		case KindVariable, "$":
			return p.parseStaticStmt()
		case KindFunction, KindFn:
			break
		default:
			return nil, p.Fail("unexpected 'static'")
		}
	case KindAttributeOpen:
		attrs, err := p.parseAttributeGroups()
		if err != nil {
			return nil, err
		}
		switch p.Current().Kind {
		case KindAbstract, KindFinal, KindClass:
			return p.parseClassStmt(attrs)
		case KindFunction:
			if p.isFunctionDecl() {
				return p.parseFunctionStmt(attrs)
			}
		}
		if p.isContextualIdent(wordEnum) && p.Peek(1).Kind == KindIdent {
			return p.parseEnumStmt(attrs)
		}
		return nil, p.Fail("expected declaration after attributes")
	}
	if p.Check(KindIdent) && p.CheckAhead(1, Colon) {
		tok := p.Advance()
		p.Advance()
		return &LabelStmt{Span: tok.Span, Name: tok.Text}, nil
	}
	if p.isContextualIdent(wordReadonly) && p.CheckAhead(1, KindClass) {
		return p.parseClassStmt(nil)
	}
	if p.isContextualIdent(wordEnum) && p.Peek(1).Kind == KindIdent {
		return p.parseEnumStmt(nil)
	}
	start := p.Current().Span
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if err := p.endStatement(); err != nil {
		return nil, err
	}
	return &ExpressionStmt{Span: p.SpanFrom(start), Expr: expr}, nil
}

// isContextualIdent reports whether the current token is an identifier
// spelling word case-insensitively. Used for soft keywords: enum,
// readonly.
func (p *parser) isContextualIdent(word string) bool {
	return p.Check(KindIdent) && strings.EqualFold(p.Current().Text, word)
}

func (p *parser) isFunctionDecl() bool {
	next := p.Peek(1).Kind
	return next == KindIdent || (next == "&" && p.Peek(2).Kind == KindIdent)
}

func (p *parser) endStatement() error {
	if p.Match(Semicolon) {
		return nil
	}
	if p.Match(KindCloseTag) {
		return nil
	}
	return p.Fail("expected ';' after statement, found " + gast.Describe(p.Current()))
}

// endAltSyntax consumes an alternative-syntax terminator keyword and
// the statement end that follows it.
func (p *parser) endAltSyntax(endKind string) error {
	if _, err := p.Expect(endKind, "'"+endKind+"'"); err != nil {
		return err
	}
	return p.endStatement()
}

// parseOptionalExpr parses an expression that may be omitted before a
// statement terminator, as in a bare return or break.
func (p *parser) parseOptionalExpr() (Expr, error) {
	switch p.Current().Kind {
	case Semicolon, KindCloseTag, RBrace:
		return nil, nil
	}
	if p.AtEnd() {
		return nil, nil
	}
	return p.parseExpr()
}

func (p *parser) parseBlockStmt() (*BlockStmt, error) {
	open := p.Advance()
	stmts, err := p.parseStmtList("}")
	if err != nil {
		return nil, err
	}
	close, err := p.Expect(RBrace, "'}'")
	if err != nil {
		return nil, err
	}
	return &BlockStmt{Span: gast.Merge(open.Span, close.Span), Stmts: stmts}, nil
}

func (p *parser) parenExpr() (Expr, error) {
	if _, err := p.Expect(LParen, "'('"); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(RParen, "')'"); err != nil {
		return nil, err
	}
	return cond, nil
}

func (p *parser) parseExprList() ([]Expr, error) {
	var exprs []Expr
	for {
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)
		if !p.Match(Comma) {
			return exprs, nil
		}
	}
}

// altBlock wraps an alternative-syntax statement list as a block whose
// span runs from the end of the opening ':' to the terminator that
// stops the list.
func (p *parser) altBlock(colon gast.Span, stmts []Stmt) *BlockStmt {
	block := &BlockStmt{Stmts: stmts}
	block.Span = gast.Span{Start: colon.End, End: p.Current().Span.Start}
	return block
}

// parseBodyOrAlt parses the body of a control structure: either a
// single statement, or ':' statements ... <endKind>. The bool reports
// whether the alternative syntax was used.
func (p *parser) parseBodyOrAlt(endKind string) (Stmt, bool, error) {
	if colon, matched := p.Current(), p.Match(Colon); matched {
		stmts, err := p.parseStmtList(endKind)
		if err != nil {
			return nil, true, err
		}
		return p.altBlock(colon.Span, stmts), true, nil
	}
	stmt, err := p.parseScopedStatement()
	return stmt, false, err
}

// parseScopedStatement parses a single-statement control-structure
// body as a nested scope, like a braced block.
func (p *parser) parseScopedStatement() (Stmt, error) {
	p.scope++
	defer func() { p.scope-- }()
	return p.parseStatement()
}

func (p *parser) parseIfStmt() (Stmt, error) {
	ifTok := p.Advance()
	cond, err := p.parenExpr()
	if err != nil {
		return nil, err
	}
	node := &IfStmt{Cond: cond}
	if colon, matched := p.Current(), p.Match(Colon); matched {
		then, err := p.parseStmtList(KindElseIf, KindElse, KindEndIf)
		if err != nil {
			return nil, err
		}
		node.Then = p.altBlock(colon.Span, then)
		for p.Check(KindElseIf) {
			elseIfTok := p.Advance()
			elseIfCond, err := p.parenExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.Expect(Colon, "':' after alternative-syntax condition"); err != nil {
				return nil, err
			}
			body, err := p.parseStmtList(KindElseIf, KindElse, KindEndIf)
			if err != nil {
				return nil, err
			}
			node.ElseIfs = append(node.ElseIfs, ElseIf{
				Span: p.SpanFrom(elseIfTok.Span),
				Cond: elseIfCond,
				Then: p.altBlock(colon.Span, body),
			})
		}
		if p.Match(KindElse) {
			elseColon, err := p.Expect(Colon, "':' after 'else'")
			if err != nil {
				return nil, err
			}
			body, err := p.parseStmtList(KindEndIf)
			if err != nil {
				return nil, err
			}
			node.Else = p.altBlock(elseColon.Span, body)
		}
		if err := p.endAltSyntax(KindEndIf); err != nil {
			return nil, err
		}
	} else {
		if node.Then, err = p.parseScopedStatement(); err != nil {
			return nil, err
		}
		for p.Check(KindElseIf) {
			elseIfTok := p.Advance()
			elseIfCond, err := p.parenExpr()
			if err != nil {
				return nil, err
			}
			then, err := p.parseScopedStatement()
			if err != nil {
				return nil, err
			}
			node.ElseIfs = append(node.ElseIfs, ElseIf{
				Span: p.SpanFrom(elseIfTok.Span),
				Cond: elseIfCond,
				Then: then,
			})
		}
		if p.Match(KindElse) {
			if node.Else, err = p.parseScopedStatement(); err != nil {
				return nil, err
			}
		}
	}
	node.Span = p.SpanFrom(ifTok.Span)
	return node, nil
}

func (p *parser) parseWhileStmt() (Stmt, error) {
	whileTok := p.Advance()
	cond, err := p.parenExpr()
	if err != nil {
		return nil, err
	}
	body, alt, err := p.parseBodyOrAlt(KindEndWhile)
	if err != nil {
		return nil, err
	}
	if alt {
		if err := p.endAltSyntax(KindEndWhile); err != nil {
			return nil, err
		}
	}
	return &WhileStmt{Span: p.SpanFrom(whileTok.Span), Cond: cond, Body: body}, nil
}

func (p *parser) parseDoWhileStmt() (Stmt, error) {
	doTok := p.Advance()
	body, err := p.parseScopedStatement()
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(KindWhile, "'while' after 'do'"); err != nil {
		return nil, err
	}
	cond, err := p.parenExpr()
	if err != nil {
		return nil, err
	}
	if err := p.endStatement(); err != nil {
		return nil, err
	}
	return &DoWhileStmt{Span: p.SpanFrom(doTok.Span), Cond: cond, Body: body}, nil
}

func (p *parser) parseForStmt() (Stmt, error) {
	forTok := p.Advance()
	if _, err := p.Expect(LParen, "'('"); err != nil {
		return nil, err
	}
	init, err := p.parseForSection()
	if err != nil {
		return nil, err
	}
	cond, err := p.parseForSection()
	if err != nil {
		return nil, err
	}
	step, err := p.parseOptionalExprList(RParen)
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(RParen, "')' to close for header"); err != nil {
		return nil, err
	}
	body, alt, err := p.parseBodyOrAlt(KindEndFor)
	if err != nil {
		return nil, err
	}
	if alt {
		if err := p.endAltSyntax(KindEndFor); err != nil {
			return nil, err
		}
	}
	return &ForStmt{Span: p.SpanFrom(forTok.Span), Init: init, Cond: cond, Step: step, Body: body}, nil
}

// parseForSection parses one semicolon-terminated section of a for
// header.
func (p *parser) parseForSection() ([]Expr, error) {
	exprs, err := p.parseOptionalExprList(Semicolon)
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(Semicolon, "';' in for header"); err != nil {
		return nil, err
	}
	return exprs, nil
}

func (p *parser) parseOptionalExprList(terminator string) ([]Expr, error) {
	var exprs []Expr
	if p.Check(terminator) {
		return exprs, nil
	}
	exprs, err := p.parseExprList()
	if err != nil {
		return nil, err
	}
	return exprs, nil
}

func (p *parser) parseForeachStmt() (Stmt, error) {
	foreachTok := p.Advance()
	if _, err := p.Expect(LParen, "'('"); err != nil {
		return nil, err
	}
	iter, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(KindAs, "'as'"); err != nil {
		return nil, err
	}
	byRef := p.Match(Ampersand)
	first, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	var key, value Expr
	if p.Match(DoubleArrow) {
		key = first
		byRef = p.Match(Ampersand)
		if value, err = p.parseExpr(); err != nil {
			return nil, err
		}
	} else {
		value = first
	}
	if _, err := p.Expect(RParen, "')'"); err != nil {
		return nil, err
	}
	body, alt, err := p.parseBodyOrAlt(KindEndForeach)
	if err != nil {
		return nil, err
	}
	if alt {
		if err := p.endAltSyntax(KindEndForeach); err != nil {
			return nil, err
		}
	}
	return &ForeachStmt{
		Span:  p.SpanFrom(foreachTok.Span),
		Iter:  iter,
		Key:   key,
		Value: value,
		ByRef: byRef,
		Body:  body,
	}, nil
}

func (p *parser) parseSwitchStmt() (Stmt, error) {
	switchTok := p.Advance()
	cond, err := p.parenExpr()
	if err != nil {
		return nil, err
	}
	alt := p.Match(Colon)
	if !alt {
		if _, err := p.Expect(LBrace, "'{' or ':' after switch"); err != nil {
			return nil, err
		}
	}
	endKind := "}"
	if alt {
		endKind = KindEndSwitch
	}
	var cases []SwitchCase
	for !p.Check(endKind) {
		switch p.Current().Kind {
		case KindCase:
			caseTok := p.Advance()
			caseCond, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if err := p.endCaseLabel(); err != nil {
				return nil, err
			}
			body, err := p.parseStmtList(KindCase, KindDefault, endKind)
			if err != nil {
				return nil, err
			}
			cases = append(cases, SwitchCase{Span: p.SpanFrom(caseTok.Span), Cond: caseCond, Body: body})
		case KindDefault:
			caseTok := p.Advance()
			if err := p.endCaseLabel(); err != nil {
				return nil, err
			}
			body, err := p.parseStmtList(KindCase, KindDefault, endKind)
			if err != nil {
				return nil, err
			}
			cases = append(cases, SwitchCase{Span: p.SpanFrom(caseTok.Span), Body: body})
		default:
			return nil, p.Fail("expected 'case' or 'default', found " + gast.Describe(p.Current()))
		}
	}
	if alt {
		if err := p.endAltSyntax(KindEndSwitch); err != nil {
			return nil, err
		}
	} else {
		if _, err := p.Expect(RBrace, "'}' to close switch"); err != nil {
			return nil, err
		}
	}
	return &SwitchStmt{Span: p.SpanFrom(switchTok.Span), Cond: cond, Cases: cases}, nil
}

func (p *parser) endCaseLabel() error {
	if p.Match(Colon) || p.Match(Semicolon) {
		return nil
	}
	return p.Fail("expected ':' after case label, found " + gast.Describe(p.Current()))
}

func (p *parser) parseBreakContinueStmt() (Stmt, error) {
	tok := p.Advance()
	level, err := p.parseOptionalExpr()
	if err != nil {
		return nil, err
	}
	if err := p.endStatement(); err != nil {
		return nil, err
	}
	if tok.Kind == KindBreak {
		return &BreakStmt{Span: p.SpanFrom(tok.Span), Level: level}, nil
	}
	return &ContinueStmt{Span: p.SpanFrom(tok.Span), Level: level}, nil
}

func (p *parser) parseReturnStmt() (Stmt, error) {
	tok := p.Advance()
	value, err := p.parseOptionalExpr()
	if err != nil {
		return nil, err
	}
	if err := p.endStatement(); err != nil {
		return nil, err
	}
	return &ReturnStmt{Span: p.SpanFrom(tok.Span), Expr: value}, nil
}

func (p *parser) parseTryStmt() (Stmt, error) {
	tryTok := p.Advance()
	body, err := p.parseBlockStmt()
	if err != nil {
		return nil, err
	}
	node := &TryStmt{Body: body}
	for p.Check(KindCatch) {
		catchTok := p.Advance()
		if _, err := p.Expect(LParen, "'(' after 'catch'"); err != nil {
			return nil, err
		}
		types, err := p.parseNameListWith(Pipe)
		if err != nil {
			return nil, err
		}
		var caught *Variable
		if p.Check(KindVariable) {
			caught = variable(p.Advance())
		}
		if _, err := p.Expect(RParen, "')'"); err != nil {
			return nil, err
		}
		catchBody, err := p.parseBlockStmt()
		if err != nil {
			return nil, err
		}
		node.Catches = append(node.Catches, &CatchClause{
			Span:  p.SpanFrom(catchTok.Span),
			Types: types,
			Var:   caught,
			Body:  catchBody,
		})
	}
	if p.Match(KindFinally) {
		if node.Finally, err = p.parseBlockStmt(); err != nil {
			return nil, err
		}
	}
	if len(node.Catches) == 0 && node.Finally == nil {
		return nil, p.Fail("expected 'catch' or 'finally' after try block")
	}
	node.Span = p.SpanFrom(tryTok.Span)
	return node, nil
}

func (p *parser) parseDeclareStmt() (Stmt, error) {
	declareTok := p.Advance()
	if _, err := p.Expect(LParen, "'(' after 'declare'"); err != nil {
		return nil, err
	}
	var directives []DeclareDirective
	for !p.Check(RParen) {
		name, err := p.Expect(KindIdent, "directive name")
		if err != nil {
			return nil, err
		}
		if _, err := p.Expect(Assign, "'=' in declare directive"); err != nil {
			return nil, err
		}
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		directives = append(directives, DeclareDirective{Span: gast.Merge(name.Span, value.Pos()), Name: name.Text, Value: value})
		if !p.Match(Comma) {
			break
		}
	}
	if _, err := p.Expect(RParen, "')' to close declare"); err != nil {
		return nil, err
	}
	node := &DeclareStmt{Directives: directives}
	switch {
	case p.Match(Semicolon):
		node.Body = &NopStmt{Span: p.Current().Span}
		node.Span = p.SpanFrom(declareTok.Span)
		return node, nil
	case p.Check(LBrace):
		body, err := p.parseBlockStmt()
		if err != nil {
			return nil, err
		}
		node.Body = body
	case p.Check(Colon):
		colon := p.Advance()
		stmts, err := p.parseStmtList(KindEndDeclare)
		if err != nil {
			return nil, err
		}
		node.Body = p.altBlock(colon.Span, stmts)
		if _, err := p.Expect(KindEndDeclare, "'enddeclare'"); err != nil {
			return nil, err
		}
		if err := p.endStatement(); err != nil {
			return nil, err
		}
	default:
		return nil, p.Fail("expected '{', ':' or ';' after declare")
	}
	node.Span = p.SpanFrom(declareTok.Span)
	return node, nil
}

// parseHaltCompilerStmt parses the reserved __halt_compiler statement.
// When the lexer recognized the empty call and semicolon, it already
// swallowed the file remainder into a raw data token; otherwise the bare
// reserved word is parsed shape-first so errors point at the offending
// token, and any use outside the file's top level is rejected as php
// rejects it.
func (p *parser) parseHaltCompilerStmt() (Stmt, error) {
	tok := p.Current()
	if p.scope > 0 {
		return nil, p.FailAt(tok.Span, "__halt_compiler() can only be used from the outermost scope")
	}
	p.Advance()
	if !strings.HasSuffix(tok.Text, Semicolon) {
		if _, err := p.Expect(LParen, "'(' after '__halt_compiler'"); err != nil {
			return nil, err
		}
		if _, err := p.Expect(RParen, "')' in __halt_compiler()"); err != nil {
			return nil, err
		}
		if err := p.endStatement(); err != nil {
			return nil, err
		}
	}
	node := &HaltCompilerStmt{Span: tok.Span}
	if p.Check(KindRawData) {
		data := p.Advance()
		node.Data = &RawDataStmt{Span: data.Span, Text: data.Text}
	}
	return node, nil
}
