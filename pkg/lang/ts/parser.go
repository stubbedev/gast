package ts

import (
	"slices"

	"github.com/stubbedev/gast/pkg/gast"
)

// cursor walks the token stream. It mirrors gast.TokenCursor but keeps
// the token slice mutable so '>'-run tokens (">>", ">>>=", ...) can be
// split while closing nested type-argument lists, and supports
// snapshot/restore for the speculative parses TypeScript's ambiguous
// "<" requires.
type cursor struct {
	tokens []gast.Token
	pos    int
	eof    gast.Token
}

func newCursor(tokens []gast.Token) *cursor {
	if len(tokens) > 0 && tokens[len(tokens)-1].Kind == KindEOF {
		tokens = tokens[:len(tokens)-1]
	}
	end := 0
	if len(tokens) > 0 {
		end = tokens[len(tokens)-1].Span.End
	}
	return &cursor{tokens: tokens, eof: gast.Token{Kind: KindEOF, Span: gast.Span{Start: end, End: end}}}
}

// Current returns the token at the cursor.
func (c *cursor) Current() gast.Token { return c.Peek(0) }

// Peek returns the token ahead positions from the cursor, or the
// end-of-input token past the end.
func (c *cursor) Peek(ahead int) gast.Token {
	if index := c.pos + ahead; index >= 0 && index < len(c.tokens) {
		return c.tokens[index]
	}
	return c.eof
}

// AtEnd reports whether every token has been consumed.
func (c *cursor) AtEnd() bool { return c.pos >= len(c.tokens) }

// Check reports whether the current token has kind.
func (c *cursor) Check(kind string) bool { return c.Current().Kind == kind }

// CheckAhead reports whether the token ahead positions from the cursor
// has kind.
func (c *cursor) CheckAhead(ahead int, kind string) bool {
	return c.Peek(ahead).Kind == kind
}

// Advance consumes and returns the current token. At the end of input it
// keeps returning the end-of-input token without consuming anything.
func (c *cursor) Advance() gast.Token {
	token := c.Current()
	if !c.AtEnd() {
		c.pos++
	}
	return token
}

// Match consumes the current token if it has kind.
func (c *cursor) Match(kind string) bool {
	if !c.Check(kind) {
		return false
	}
	c.pos++
	return true
}

// Expect consumes the current token if it has kind. Description replaces
// the default quoted kind in the error message.
func (c *cursor) Expect(kind, description string) (gast.Token, error) {
	if c.Check(kind) {
		return c.Advance(), nil
	}
	if description == "" {
		description = "'" + kind + "'"
	}
	return gast.Token{}, c.Fail("expected " + description + ", found " + gast.Describe(c.Current()))
}

// Fail returns a ParseError at the current token.
func (c *cursor) Fail(message string) error {
	return &gast.ParseError{Message: message, Pos: c.Current().Span}
}

// FailAt returns a ParseError at an explicit span.
func (c *cursor) FailAt(span gast.Span, message string) error {
	return &gast.ParseError{Message: message, Pos: span}
}

// SpanFrom merges start with the span of the last consumed token.
func (c *cursor) SpanFrom(start gast.Span) gast.Span {
	if c.pos == 0 {
		return start
	}
	return gast.Merge(start, c.tokens[c.pos-1].Span)
}

// snapshot returns the current position for a speculative parse.
func (c *cursor) snapshot() int { return c.pos }

// restore rewinds to a snapshot after a failed speculative parse.
func (c *cursor) restore(pos int) { c.pos = pos }

// matchGt consumes one '>' of a type-argument closer, splitting a
// merged '>'-run token when the lexer produced ">>" or worse.
func (c *cursor) matchGt() bool {
	if c.Match(Gt) {
		return true
	}
	if c.pos < len(c.tokens) {
		if tokens, ok := splitGreater(c.tokens, c.pos); ok {
			c.tokens = tokens
			return c.Match(Gt)
		}
	}
	return false
}

// expectGt consumes one '>' of a type-argument closer or fails.
func (c *cursor) expectGt(description string) error {
	if c.matchGt() {
		return nil
	}
	return c.Fail("expected '>' to close " + description + ", found " + gast.Describe(c.Current()))
}

// parser walks the token stream. It tracks whether it sits inside an
// async function or generator so await and yield are parsed as
// operators exactly where they can appear.
type parser struct {
	*cursor
	source      string
	inAsync     bool
	inGenerator bool
	inFunction  bool
}

func newParser(tokens []gast.Token, source string) *parser {
	return &parser{cursor: newCursor(tokens), source: source}
}

func (p *parser) parseFile() (*File, error) {
	p.Match(KindHashbang)
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
	for !p.AtEnd() {
		if slices.Contains(terminators, p.Current().Kind) {
			return stmts, nil
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, stmt)
	}
	return stmts, nil
}

// isWord reports whether the current token is an identifier spelling
// word. Used for contextual keywords: the TypeScript-only words (type,
// interface, enum, namespace, declare, abstract, readonly, public,
// private, protected, implements, satisfies, as, is, keyof, infer,
// asserts, unique) plus JavaScript's let, static, async, await, yield,
// of, get, set, from, as.
func (p *parser) isWord(word string) bool {
	return p.Check(KindIdent) && p.Current().Text == word
}

// wordAt reports whether the token ahead positions from the cursor is
// the given contextual word.
func (p *parser) wordAt(ahead int, word string) bool {
	tok := p.Peek(ahead)
	return tok.Kind == KindIdent && tok.Text == word
}

// hadLineBreak reports whether a line terminator separates the
// previous token from the current one; the parser's ASI rules read it.
func (p *parser) hadLineBreak() bool {
	from := p.Peek(-1).Span.End
	if from > p.Current().Span.Start {
		from = 0
	}
	return containsLineBreak(p.source[from:p.Current().Span.Start])
}

// lineBreakBetween reports whether a line terminator separates the
// spans of two tokens.
func (p *parser) lineBreakBetween(a, b gast.Token) bool {
	return containsLineBreak(p.source[a.Span.End:b.Span.Start])
}

func containsLineBreak(text string) bool {
	for i := range len(text) {
		switch text[i] {
		case '\n', '\r':
			return true
		case 0xE2:
			if i+2 < len(text) && text[i+1] == 0x80 && (text[i+2] == 0xA8 || text[i+2] == 0xA9) {
				return true
			}
		}
	}
	return false
}

// consumeSemicolon applies automatic semicolon insertion: a statement
// may end at a semicolon, a closing brace, the end of input, or a
// newline before the next token.
func (p *parser) consumeSemicolon() error {
	if p.Match(Semicolon) {
		return nil
	}
	if p.Check(RBrace) || p.AtEnd() || p.hadLineBreak() {
		return nil
	}
	return p.Fail("expected ';' after statement, found " + gast.Describe(p.Current()))
}

// isLetDecl reports whether a contextual "let" starts a declaration:
// only when followed by a binding start. Elsewhere let is a plain
// identifier (sloppy mode).
func (p *parser) isLetDecl() bool {
	if !p.isWord("let") {
		return false
	}
	switch p.Peek(1).Kind {
	case KindIdent, LBracket, LBrace:
		return true
	}
	return false
}

// isAsyncFunction reports whether the current token starts an `async
// function` declaration or expression on one line.
func (p *parser) isAsyncFunction() bool {
	return p.isWord("async") && p.CheckAhead(1, KindFunction) &&
		!p.lineBreakBetween(p.Current(), p.Peek(1))
}

// isTypeAliasStart reports whether a contextual "type" starts a type
// alias: followed by a name and then '=' or '<'.
func (p *parser) isTypeAliasStart() bool {
	return p.isWord("type") && p.CheckAhead(1, KindIdent) &&
		(p.CheckAhead(2, Assign) || p.CheckAhead(2, Lt))
}

// isEnumStart reports whether the current token starts an enum: the
// word "enum" (or `const enum`) followed by a name.
func (p *parser) isEnumStart() bool {
	if p.Check(KindConst) && p.wordAt(1, "enum") {
		return p.CheckAhead(2, KindIdent)
	}
	return p.isWord("enum") && p.CheckAhead(1, KindIdent)
}

// isNamespaceStart reports whether a contextual "namespace" or
// "module" starts a namespace: followed by a name or dotted name and
// then a body.
func (p *parser) isNamespaceStart() bool {
	if !p.isWord("namespace") && !p.isWord("module") {
		return false
	}
	switch p.Peek(1).Kind {
	case KindIdent, KindString:
		return p.CheckAhead(2, LBrace) || p.CheckAhead(2, Dot)
	}
	return false
}

// isDeclareStart reports whether a contextual "declare" precedes an
// ambient declaration.
func (p *parser) isDeclareStart() bool {
	if !p.isWord("declare") {
		return false
	}
	if p.CheckAhead(1, KindVar) || p.CheckAhead(1, KindConst) || p.CheckAhead(1, KindFunction) ||
		p.CheckAhead(1, KindClass) || p.wordAt(1, "enum") {
		return true
	}
	return p.wordAt(1, "let") || p.wordAt(1, "abstract") || p.wordAt(1, "namespace") ||
		p.wordAt(1, "module") || p.wordAt(1, "global") || p.wordAt(1, "interface") ||
		p.wordAt(1, "type") || p.wordAt(1, "async")
}

func (p *parser) parseStatement() (Stmt, error) {
	switch p.Current().Kind {
	case LBrace:
		return p.parseBlockStmt()
	case Semicolon:
		return &EmptyStmt{Span: p.Advance().Span}, nil
	case KindVar, KindConst:
		if p.isEnumStart() {
			return p.parseEnumDecl()
		}
		return p.parseVarStmt(false)
	case KindIf:
		return p.parseIfStmt()
	case KindWhile:
		return p.parseWhileStmt()
	case KindDo:
		return p.parseDoWhileStmt()
	case KindFor:
		return p.parseForStmt(false)
	case KindSwitch:
		return p.parseSwitchStmt()
	case KindTry:
		return p.parseTryStmt()
	case KindThrow:
		return p.parseThrowStmt()
	case KindReturn:
		return p.parseReturnStmt()
	case KindBreak:
		return p.parseBreakContinueStmt(false)
	case KindContinue:
		return p.parseBreakContinueStmt(true)
	case KindDebugger:
		tok := p.Advance()
		if err := p.consumeSemicolon(); err != nil {
			return nil, err
		}
		return &DebuggerStmt{Span: p.SpanFrom(tok.Span)}, nil
	case KindWith:
		return p.parseWithStmt()
	case KindFunction:
		return p.parseFunctionDecl(false, false, true)
	case KindClass:
		return p.parseClassDecl(nil, false, false, true)
	case KindImport:
		switch p.Peek(1).Kind {
		case LParen, Dot:
			// dynamic import or import.meta expression
		default:
			return p.parseImportDecl()
		}
	case KindExport:
		return p.parseExportStmt()
	case At:
		return p.parseDecoratedStmt()
	}
	if p.isAsyncFunction() {
		return p.parseFunctionDecl(true, false, true)
	}
	if p.isWord("abstract") && p.CheckAhead(1, KindClass) {
		return p.parseClassDecl(nil, true, false, true)
	}
	if p.isDeclareStart() {
		return p.parseDeclareStmt()
	}
	if p.isTypeAliasStart() {
		return p.parseTypeAliasDecl(false)
	}
	if p.isWord("interface") && p.CheckAhead(1, KindIdent) {
		return p.parseInterfaceDecl(false)
	}
	if p.isWord("enum") && p.CheckAhead(1, KindIdent) {
		return p.parseEnumDecl()
	}
	if p.isNamespaceStart() {
		return p.parseNamespaceDecl(false)
	}
	if p.isLetDecl() {
		return p.parseVarStmt(false)
	}
	// Label: ident ':' statement.
	if p.Check(KindIdent) && p.CheckAhead(1, Colon) && !p.lineBreakBetween(p.Current(), p.Peek(1)) {
		label := p.Advance()
		p.Advance()
		body, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		return &LabeledStmt{Span: p.SpanFrom(label.Span), Label: label.Text, Body: body}, nil
	}
	return p.parseExpressionStmt()
}

// parseDecoratedStmt parses decorators followed by a class declaration
// or a decorated export of one.
func (p *parser) parseDecoratedStmt() (Stmt, error) {
	decorators, err := p.parseDecorators()
	if err != nil {
		return nil, err
	}
	switch {
	case p.Check(KindClass):
		return p.parseClassDecl(decorators, false, false, true)
	case p.isWord("abstract") && p.CheckAhead(1, KindClass):
		return p.parseClassDecl(decorators, true, false, true)
	case p.Check(KindExport) && p.decoratedExportClassAhead(1):
		return p.parseExportDecorated(decorators)
	}
	return nil, p.Fail("expected class declaration after decorator, found " + gast.Describe(p.Current()))
}

// decoratedExportClassAhead reports whether the tokens after 'export'
// form a class declaration that may carry decorators.
func (p *parser) decoratedExportClassAhead(from int) bool {
	if p.CheckAhead(from, KindClass) {
		return true
	}
	if p.CheckAhead(from, KindDefault) {
		from++
	}
	if p.wordAt(from, "abstract") {
		from++
	}
	return p.CheckAhead(from, KindClass)
}

func (p *parser) parseExpressionStmt() (Stmt, error) {
	start := p.Current().Span
	expr, err := p.parseExpr(false)
	if err != nil {
		return nil, err
	}
	if err := p.checkExprStatement(expr); err != nil {
		return nil, err
	}
	if err := p.consumeSemicolon(); err != nil {
		return nil, err
	}
	return &ExpressionStmt{Span: p.SpanFrom(start), Expr: expr}, nil
}

// checkExprStatement rejects a destructuring cover that never became
// an assignment target: `({a = 1})` is valid only in pattern position.
func (p *parser) checkExprStatement(expr Expr) error {
	for {
		paren, ok := expr.(*ParenExpr)
		if !ok {
			break
		}
		expr = paren.Expr
	}
	if object, ok := expr.(*ObjectExpr); ok && objectHasDefault(object) {
		return p.FailAt(object.Span, "invalid property initializer outside destructuring")
	}
	return nil
}

func (p *parser) parseBlockStmt() (*BlockStmt, error) {
	open := p.Advance()
	stmts, err := p.parseStmtList(RBrace)
	if err != nil {
		return nil, err
	}
	close, err := p.Expect(RBrace, "'}'")
	if err != nil {
		return nil, err
	}
	return &BlockStmt{Span: gast.Merge(open.Span, close.Span), Stmts: stmts}, nil
}

// parenExpr parses ( expr ).
func (p *parser) parenExpr() (Expr, error) {
	if _, err := p.Expect(LParen, "'('"); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr(false)
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(RParen, "')'"); err != nil {
		return nil, err
	}
	return cond, nil
}

func (p *parser) parseIfStmt() (Stmt, error) {
	ifTok := p.Advance()
	cond, err := p.parenExpr()
	if err != nil {
		return nil, err
	}
	node := &IfStmt{Cond: cond}
	if node.Then, err = p.parseStatement(); err != nil {
		return nil, err
	}
	if p.Match(KindElse) {
		if node.Else, err = p.parseStatement(); err != nil {
			return nil, err
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
	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	return &WhileStmt{Span: p.SpanFrom(whileTok.Span), Cond: cond, Body: body}, nil
}

func (p *parser) parseDoWhileStmt() (Stmt, error) {
	doTok := p.Advance()
	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(KindWhile, "'while'"); err != nil {
		return nil, err
	}
	cond, err := p.parenExpr()
	if err != nil {
		return nil, err
	}
	node := &DoWhileStmt{Cond: cond, Body: body}
	if err := p.consumeSemicolon(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(doTok.Span)
	return node, nil
}

func (p *parser) parseWithStmt() (Stmt, error) {
	withTok := p.Advance()
	object, err := p.parenExpr()
	if err != nil {
		return nil, err
	}
	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	return &WithStmt{Span: p.SpanFrom(withTok.Span), Object: object, Body: body}, nil
}

// parseForStmt parses all four for variants. noIn hides the in
// operator inside the init clause so `for (x in y)` is a for-in, not a
// comparison.
func (p *parser) parseForStmt(await bool) (Stmt, error) {
	forTok := p.Advance()
	if p.isWord("await") && p.CheckAhead(1, LParen) && !await {
		p.Advance()
		await = true
	}
	if _, err := p.Expect(LParen, "'(' after 'for'"); err != nil {
		return nil, err
	}
	if p.Check(Semicolon) {
		return p.parseClassicFor(forTok, nil)
	}
	if p.Check(KindVar) || p.Check(KindConst) || p.isLetDecl() {
		var left Stmt
		var err error
		if left, err = p.parseForHeadDecl(); err != nil {
			return nil, err
		}
		if p.Check(KindIn) || p.isWord("of") {
			return p.parseForInOf(forTok, await, left)
		}
		if !p.Check(Semicolon) {
			return nil, p.Fail("expected ';' after for-loop declaration, found " + gast.Describe(p.Current()))
		}
		return p.parseClassicFor(forTok, left)
	}
	start := p.Current().Span
	init, err := p.parseExpr(true)
	if err != nil {
		return nil, err
	}
	wrapped := &ExpressionStmt{Span: p.SpanFrom(start), Expr: init}
	if p.Check(KindIn) || p.isWord("of") {
		return p.parseForInOf(forTok, await, wrapped)
	}
	return p.parseClassicFor(forTok, wrapped)
}

// parseForHeadDecl parses the declaration clause of a for-in/of head:
// exactly one declarator, whose initializer is consumed only by the
// classic-for path.
func (p *parser) parseForHeadDecl() (Stmt, error) {
	kindTok := p.Advance()
	decl, err := p.parseDeclarator()
	if err != nil {
		return nil, err
	}
	node := &VarStmt{Kind: kindTok.Kind, Decls: []*VarDeclarator{decl}}
	if p.Check(KindIn) || p.isWord("of") {
		if decl.Init != nil {
			return nil, p.FailAt(decl.Span, "for-in/of loop declaration cannot have an initializer")
		}
		node.Span = p.SpanFrom(kindTok.Span)
		return node, nil
	}
	if p.Match(Assign) {
		if decl.Init, err = p.parseAssignExpr(false); err != nil {
			return nil, err
		}
		decl.Span = p.SpanFrom(decl.Binding.Pos())
	}
	for p.Match(Comma) {
		next, err := p.parseDeclarator()
		if err != nil {
			return nil, err
		}
		node.Decls = append(node.Decls, next)
	}
	node.Span = p.SpanFrom(kindTok.Span)
	return node, nil
}

// parseForInOf parses the `in expr` / `of expr` head and body shared
// by for-in, for-of, and for-await-of.
func (p *parser) parseForInOf(forTok gast.Token, await bool, left Stmt) (Stmt, error) {
	isOf := p.isWord("of")
	p.Advance()
	if err := p.validateForTarget(left); err != nil {
		return nil, err
	}
	var right Expr
	var err error
	if isOf {
		right, err = p.parseAssignExpr(false)
	} else {
		right, err = p.parseExpr(false)
	}
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(RParen, "')' after for-loop head"); err != nil {
		return nil, err
	}
	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	span := p.SpanFrom(forTok.Span)
	if isOf {
		return &ForOfStmt{Span: span, Await: await, Left: left, Right: right, Body: body}, nil
	}
	if await {
		return nil, p.FailAt(span, "'for await' requires 'of'")
	}
	return &ForInStmt{Span: span, Left: left, Right: right, Body: body}, nil
}

// validateForTarget checks that the left side of a for-in/of is a
// declaration or a valid assignment target.
func (p *parser) validateForTarget(left Stmt) error {
	if _, ok := left.(*VarStmt); ok {
		return nil
	}
	stmt, ok := left.(*ExpressionStmt)
	if !ok {
		return p.FailAt(left.Pos(), "invalid for-in/of target")
	}
	expr := stmt.Expr
	if err := p.validateAssignTarget(expr, false); err != nil {
		return err
	}
	return p.checkExprStatement(expr)
}

func (p *parser) parseClassicFor(forTok gast.Token, init Stmt) (Stmt, error) {
	p.Advance() // ';'
	node := &ForStmt{Init: init}
	var err error
	if !p.Check(Semicolon) {
		if node.Test, err = p.parseExpr(false); err != nil {
			return nil, err
		}
	}
	if _, err := p.Expect(Semicolon, "';' in for-loop head"); err != nil {
		return nil, err
	}
	if !p.Check(RParen) {
		if node.Update, err = p.parseExpr(false); err != nil {
			return nil, err
		}
	}
	if _, err := p.Expect(RParen, "')' after for-loop head"); err != nil {
		return nil, err
	}
	if node.Body, err = p.parseStatement(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(forTok.Span)
	return node, nil
}

func (p *parser) parseSwitchStmt() (Stmt, error) {
	switchTok := p.Advance()
	disc, err := p.parenExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(LBrace, "'{' to open switch body"); err != nil {
		return nil, err
	}
	node := &SwitchStmt{Disc: disc}
	for !p.Check(RBrace) {
		if p.AtEnd() {
			return nil, p.Fail("expected 'case' or 'default' in switch body")
		}
		var head gast.Token
		var testCase Expr
		switch p.Current().Kind {
		case KindCase:
			head = p.Advance()
			if testCase, err = p.parseExpr(false); err != nil {
				return nil, err
			}
		case KindDefault:
			head = p.Advance()
		default:
			return nil, p.Fail("expected 'case' or 'default' in switch body, found " + gast.Describe(p.Current()))
		}
		if _, err := p.Expect(Colon, "':' after switch case"); err != nil {
			return nil, err
		}
		body, err := p.parseStmtList(KindCase, KindDefault, RBrace)
		if err != nil {
			return nil, err
		}
		node.Cases = append(node.Cases, &SwitchCase{Span: p.SpanFrom(head.Span), Test: testCase, Body: body})
	}
	close := p.Advance()
	node.Span = gast.Merge(switchTok.Span, close.Span)
	return node, nil
}

func (p *parser) parseTryStmt() (Stmt, error) {
	tryTok := p.Advance()
	body, err := p.parseBlockStmt()
	if err != nil {
		return nil, err
	}
	node := &TryStmt{Block: body}
	if p.Check(KindCatch) {
		catchTok := p.Advance()
		clause := &CatchClause{}
		if p.Match(LParen) {
			if clause.Param, err = p.parseBinding(); err != nil {
				return nil, err
			}
			if p.Match(Colon) {
				if clause.Type, err = p.parseType(); err != nil {
					return nil, err
				}
			}
			if _, err := p.Expect(RParen, "')' after catch binding"); err != nil {
				return nil, err
			}
		}
		if clause.Block, err = p.parseBlockStmt(); err != nil {
			return nil, err
		}
		clause.Span = p.SpanFrom(catchTok.Span)
		node.Catch = clause
	}
	if p.Match(KindFinally) {
		if node.Finally, err = p.parseBlockStmt(); err != nil {
			return nil, err
		}
	}
	if node.Catch == nil && node.Finally == nil {
		return nil, p.Fail("expected 'catch' or 'finally' after try block")
	}
	node.Span = p.SpanFrom(tryTok.Span)
	return node, nil
}

func (p *parser) parseThrowStmt() (Stmt, error) {
	throwTok := p.Advance()
	if p.hadLineBreak() {
		return nil, p.Fail("no line break allowed after 'throw'")
	}
	expr, err := p.parseExpr(false)
	if err != nil {
		return nil, err
	}
	if err := p.consumeSemicolon(); err != nil {
		return nil, err
	}
	return &ThrowStmt{Span: p.SpanFrom(throwTok.Span), Expr: expr}, nil
}

func (p *parser) parseReturnStmt() (Stmt, error) {
	returnTok := p.Advance()
	if !p.inFunction {
		return nil, p.FailAt(returnTok.Span, "'return' outside of function")
	}
	node := &ReturnStmt{}
	if !p.hadLineBreak() && !p.stmtExprEnd() {
		expr, err := p.parseExpr(false)
		if err != nil {
			return nil, err
		}
		node.Expr = expr
	}
	if err := p.consumeSemicolon(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(returnTok.Span)
	return node, nil
}

// stmtExprEnd reports whether the current token cannot continue the
// optional expression of return or yield.
func (p *parser) stmtExprEnd() bool {
	switch p.Current().Kind {
	case Semicolon, RBrace, RParen, RBracket, Comma, Colon, KindEOF:
		return true
	}
	return false
}

func (p *parser) parseBreakContinueStmt(isContinue bool) (Stmt, error) {
	kw := p.Advance()
	label := ""
	if !p.hadLineBreak() && p.Check(KindIdent) {
		label = p.Advance().Text
	}
	if err := p.consumeSemicolon(); err != nil {
		return nil, err
	}
	if isContinue {
		return &ContinueStmt{Span: p.SpanFrom(kw.Span), Label: label}, nil
	}
	return &BreakStmt{Span: p.SpanFrom(kw.Span), Label: label}, nil
}

func (p *parser) parseVarStmt(declare bool) (Stmt, error) {
	kindTok := p.Advance()
	node := &VarStmt{Kind: kindTok.Kind, Declare: declare}
	for {
		decl, err := p.parseDeclarator()
		if err != nil {
			return nil, err
		}
		node.Decls = append(node.Decls, decl)
		if !p.Match(Comma) {
			break
		}
	}
	if err := p.consumeSemicolon(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(kindTok.Span)
	return node, nil
}

func (p *parser) parseDeclarator() (*VarDeclarator, error) {
	binding, err := p.parseBinding()
	if err != nil {
		return nil, err
	}
	node := &VarDeclarator{Binding: binding}
	node.Span = binding.Pos()
	if p.Match(Bang) {
		node.Definite = true
	}
	if p.Match(Colon) {
		if node.Type, err = p.parseType(); err != nil {
			return nil, err
		}
	}
	if p.Match(Assign) {
		if node.Init, err = p.parseAssignExpr(false); err != nil {
			return nil, err
		}
		node.Span = p.SpanFrom(binding.Pos())
	}
	return node, nil
}

// parseBinding parses an identifier or destructuring pattern in a
// binding position.
func (p *parser) parseBinding() (Pattern, error) {
	switch p.Current().Kind {
	case KindIdent:
		tok := p.Advance()
		return &IdentExpr{Span: tok.Span, Name: tok.Text}, nil
	case LBrace:
		return p.parseObjectPattern()
	case LBracket:
		return p.parseArrayPattern()
	}
	return nil, p.Fail("expected binding, found " + gast.Describe(p.Current()))
}

// bindingWithDefault parses a binding with an optional initializer.
func (p *parser) bindingWithDefault() (*Param, error) {
	start := p.Current().Span
	if p.Match(Ellipsis) {
		binding, err := p.parseBinding()
		if err != nil {
			return nil, err
		}
		return &Param{Span: p.SpanFrom(start), Binding: binding, Rest: true}, nil
	}
	binding, err := p.parseBinding()
	if err != nil {
		return nil, err
	}
	param := &Param{Binding: binding}
	param.Span = binding.Pos()
	if p.Match(Assign) {
		if param.Default, err = p.parseAssignExpr(false); err != nil {
			return nil, err
		}
		param.Span = p.SpanFrom(start)
	}
	return param, nil
}

func (p *parser) parseObjectPattern() (Pattern, error) {
	open := p.Advance()
	node := &ObjectPattern{}
	for !p.Check(RBrace) {
		if p.AtEnd() {
			return nil, p.Fail("expected '}' to close object pattern")
		}
		prop := &BindingProp{}
		propStart := p.Current().Span
		if p.Match(Ellipsis) {
			binding, err := p.parseBinding()
			if err != nil {
				return nil, err
			}
			prop.Rest = true
			prop.Value = binding
			prop.Span = p.SpanFrom(propStart)
			node.Props = append(node.Props, prop)
			if !p.Match(Comma) {
				break
			}
			continue
		}
		key, computed, err := p.parsePropertyKey()
		if err != nil {
			return nil, err
		}
		prop.Key = key
		prop.Computed = computed
		if p.Match(Colon) {
			if prop.Value, err = p.parseBinding(); err != nil {
				return nil, err
			}
			if p.Match(Assign) {
				if prop.Default, err = p.parseAssignExpr(false); err != nil {
					return nil, err
				}
			}
		} else {
			if ident, ok := key.(*IdentExpr); ok && !computed {
				prop.Value = &IdentExpr{Span: ident.Span, Name: ident.Name}
			} else {
				return nil, p.Fail("expected ':' in object pattern")
			}
			if p.Match(Assign) {
				if prop.Default, err = p.parseAssignExpr(false); err != nil {
					return nil, err
				}
			}
		}
		prop.Span = p.SpanFrom(propStart)
		node.Props = append(node.Props, prop)
		if !p.Match(Comma) {
			break
		}
	}
	close, err := p.Expect(RBrace, "'}' to close object pattern")
	if err != nil {
		return nil, err
	}
	node.Span = gast.Merge(open.Span, close.Span)
	return node, nil
}

func (p *parser) parseArrayPattern() (Pattern, error) {
	open := p.Advance()
	var elements []Pattern
	for !p.Check(RBracket) {
		if p.AtEnd() {
			return nil, p.Fail("expected ']' to close array pattern")
		}
		if p.Match(Comma) {
			elements = append(elements, nil)
			continue
		}
		param, err := p.bindingWithDefault()
		if err != nil {
			return nil, err
		}
		var elem Pattern
		switch {
		case param.Rest:
			elem = &RestElement{Span: param.Span, Arg: param.Binding}
		case param.Default != nil:
			elem = &DefaultPattern{Span: param.Span, Binding: param.Binding, Default: param.Default}
		default:
			elem = param.Binding
		}
		elements = append(elements, elem)
		if !p.Match(Comma) {
			break
		}
	}
	close, err := p.Expect(RBracket, "']' to close array pattern")
	if err != nil {
		return nil, err
	}
	return &ArrayPattern{Span: gast.Merge(open.Span, close.Span), Elements: elements}, nil
}
