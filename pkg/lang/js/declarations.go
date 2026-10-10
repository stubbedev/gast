package js

import (
	"github.com/stubbedev/gast/pkg/gast"
)

// parseParams parses a parenthesized parameter list with defaults,
// rest, and a trailing comma.
func (p *parser) parseParams() ([]*Param, error) {
	if _, err := p.Expect(LParen, "'('"); err != nil {
		return nil, err
	}
	var params []*Param
	for !p.Check(RParen) {
		if p.AtEnd() {
			return nil, p.Fail("expected ')' to close parameter list")
		}
		param, err := p.bindingWithDefault()
		if err != nil {
			return nil, err
		}
		if param.Rest && param.Default != nil {
			return nil, p.FailAt(param.Span, "rest parameter cannot have a default")
		}
		params = append(params, param)
		if !p.Match(Comma) {
			break
		}
	}
	if _, err := p.Expect(RParen, "')' to close parameter list"); err != nil {
		return nil, err
	}
	return params, nil
}

// parseFunctionDecl parses a function declaration; async marks an
// `async function` declaration. requireName is false only for export
// default, where the name is optional.
func (p *parser) parseFunctionDecl(async, requireName bool) (Stmt, error) {
	fn, err := p.parseFunction(async, requireName)
	if err != nil {
		return nil, err
	}
	decl := &FunctionDecl{Span: fn.Span, Async: fn.Async, Generator: fn.Generator, Name: fn.Name, Params: fn.Params, Body: fn.Body}
	return decl, nil
}

// parseFunctionExpr parses a function expression; async is set by the
// caller when the expression follows an async keyword.
func (p *parser) parseFunctionExpr(async bool) (Expr, error) {
	fn, err := p.parseFunction(async, false)
	if err != nil {
		return nil, err
	}
	return &FunctionExpr{Span: fn.Span, Async: fn.Async, Generator: fn.Generator, Name: fn.Name, Params: fn.Params, Body: fn.Body}, nil
}

// fnParts is the shared shape of function declarations and
// expressions.
type fnParts struct {
	gast.Span
	Async     bool
	Generator bool
	Name      string
	Params    []*Param
	Body      *BlockStmt
}

// parseFunction consumes `function [*] [name] (params) { body }`. For
// declarations the name is required (unless default-exported); for
// expressions it is optional.
func (p *parser) parseFunction(async, requireName bool) (*fnParts, error) {
	start := p.Current().Span
	if async {
		start = p.Advance().Span // async
	}
	if _, err := p.Expect(KindFunction, "'function'"); err != nil {
		return nil, err
	}
	fn := &fnParts{Async: async}
	if p.Match(Asterisk) {
		fn.Generator = true
	}
	if p.Check(KindIdent) {
		fn.Name = p.Advance().Text
	} else if requireName {
		return nil, p.Fail("expected function name, found " + gast.Describe(p.Current()))
	}
	var err error
	if fn.Params, err = p.parseParams(); err != nil {
		return nil, err
	}
	savedAsync, savedGen, savedFn := p.inAsync, p.inGenerator, p.inFunction
	p.inAsync, p.inGenerator, p.inFunction = async, fn.Generator, true
	fn.Body, err = p.parseBlockStmt()
	p.inAsync, p.inGenerator, p.inFunction = savedAsync, savedGen, savedFn
	if err != nil {
		return nil, err
	}
	fn.Span = p.SpanFrom(start)
	return fn, nil
}

// parseClassDecl parses a class declaration; requireName is false only
// for export default.
func (p *parser) parseClassDecl(requireName bool) (Stmt, error) {
	node, err := p.parseClass(requireName)
	if err != nil {
		return nil, err
	}
	return &ClassDecl{Span: node.Span, Name: node.Name, Extends: node.Extends, Body: node.Body}, nil
}

// parseClassExpr parses a class expression.
func (p *parser) parseClassExpr() (Expr, error) {
	node, err := p.parseClass(false)
	if err != nil {
		return nil, err
	}
	return &ClassExpr{Span: node.Span, Name: node.Name, Extends: node.Extends, Body: node.Body}, nil
}

// parseClass consumes `class [name] [extends expr] { members }`.
func (p *parser) parseClass(requireName bool) (*ClassDecl, error) {
	classTok := p.Advance()
	node := &ClassDecl{}
	if p.Check(KindIdent) {
		node.Name = p.Advance().Text
	} else if requireName {
		return nil, p.Fail("expected class name, found " + gast.Describe(p.Current()))
	}
	if p.Match(KindExtends) {
		var err error
		if node.Extends, err = p.parseLeftHandSide(); err != nil {
			return nil, err
		}
	}
	if _, err := p.Expect(LBrace, "'{' to open class body"); err != nil {
		return nil, err
	}
	var err error
	if node.Body, err = p.parseClassBody(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(classTok.Span)
	return node, nil
}

// parseClassBody parses members until the closing brace. Fields end by
// semicolon, newline (ASI), or brace; methods carry their own bodies.
func (p *parser) parseClassBody() ([]ClassMember, error) {
	var members []ClassMember
	for !p.Check(RBrace) {
		if p.AtEnd() {
			return nil, p.Fail("expected '}' to close class body")
		}
		if p.Match(Semicolon) {
			continue
		}
		member, err := p.parseClassMember()
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	p.Advance()
	return members, nil
}

// isClassKeyStart reports whether a token can begin a member name.
func isClassKeyStart(kind string) bool {
	switch kind {
	case KindIdent, KindPrivateName, KindString, KindNum, KindBigInt, LBracket:
		return true
	}
	return false
}

// isModifier reports whether the identifier token followed by a token
// of the given kind acts as the modifier word rather than a member
// name: get/set/async/static are plain names when a "(" follows, as in
// a method named "get". The next-next kind parameter is unused but
// kept so call sites read uniformly.
func isModifier(_, kind, _ string) bool {
	if kind == Asterisk {
		return true
	}
	return isClassKeyStart(kind) && kind != LParen
}

func (p *parser) parseClassMember() (ClassMember, error) {
	start := p.Current().Span
	static := false
	if p.isWord("static") && (isModifier("static", p.Peek(1).Kind, p.Peek(2).Kind) || p.CheckAhead(1, LBrace)) {
		static = true
		p.Advance()
		if p.Check(LBrace) {
			block, err := p.parseBlockStmt()
			if err != nil {
				return nil, err
			}
			return &StaticBlock{Span: p.SpanFrom(start), Stmts: block.Stmts}, nil
		}
	}
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
	switch {
	case p.Check(LParen):
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
		if accessor == "get" {
			kind = "get"
			if len(params) != 0 {
				return nil, p.FailAt(p.SpanFrom(start), "getter cannot take parameters")
			}
		} else if accessor == "set" {
			kind = "set"
			if len(params) != 1 {
				return nil, p.FailAt(p.SpanFrom(start), "setter takes exactly one parameter")
			}
		} else if !static && !computed {
			if ident, ok := key.(*IdentExpr); ok && ident.Name == "constructor" {
				kind = "constructor"
			} else if lit, ok := key.(*StringLit); ok && lit.Value == "constructor" {
				kind = "constructor"
			}
		}
		return &MethodDef{
			Span: p.SpanFrom(start), Key: key, Static: static, Async: async,
			Generator: generator, Kind: kind, Params: params, Body: body,
		}, nil
	case accessor != "":
		return nil, p.Fail("expected '(' after accessor name")
	default:
		node := &PropertyDef{Key: key, Static: static, Computed: computed}
		if p.Match(Assign) {
			if node.Init, err = p.parseAssignExpr(false); err != nil {
				return nil, err
			}
		}
		if err := p.consumeSemicolon(); err != nil {
			return nil, err
		}
		node.Span = p.SpanFrom(start)
		return node, nil
	}
}

// parsePropertyKey parses an identifier, private name, string, number,
// or computed key; the bool reports the computed form.
func (p *parser) parsePropertyKey() (Expr, bool, error) {
	tok := p.Current()
	if _, ok := keywords[tok.Text]; ok && tok.Kind != KindTrue && tok.Kind != KindFalse && tok.Kind != KindNull {
		// reserved words are valid identifierName property keys
		p.Advance()
		return &IdentExpr{Span: tok.Span, Name: tok.Text}, false, nil
	}
	switch tok.Kind {
	case KindIdent, KindPrivateName:
		p.Advance()
		if tok.Kind == KindPrivateName {
			return &PrivateName{Span: tok.Span, Name: tok.Text[1:]}, false, nil
		}
		return &IdentExpr{Span: tok.Span, Name: tok.Text}, false, nil
	case KindString:
		p.Advance()
		return &StringLit{Span: tok.Span, Text: tok.Text, Value: decodeString(tok.Text)}, false, nil
	case KindNum, KindBigInt:
		p.Advance()
		return p.litToken(tok), false, nil
	case LBracket:
		p.Advance()
		expr, err := p.parseAssignExpr(false)
		if err != nil {
			return nil, false, err
		}
		if _, err := p.Expect(RBracket, "']' to close computed key"); err != nil {
			return nil, false, err
		}
		return expr, true, nil
	}
	return nil, false, p.Fail("expected property name, found " + gast.Describe(tok))
}

// parseImportDecl parses every import form: side-effect-only,
// default, namespace, named (with renames and string names), and their
// combinations.
func (p *parser) parseImportDecl() (Stmt, error) {
	importTok := p.Advance()
	node := &ImportDecl{}
	if p.Check(KindString) {
		node.Source = p.Advance().Text
		if err := p.consumeSemicolon(); err != nil {
			return nil, err
		}
		node.Span = p.SpanFrom(importTok.Span)
		return node, nil
	}
	switch p.Current().Kind {
	case KindIdent:
		node.Default = p.Advance().Text
	case Asterisk:
		if err := p.parseImportNamespace(node); err != nil {
			return nil, err
		}
		return p.finishImport(importTok, node)
	case LBrace:
		if err := p.parseImportSpecs(node); err != nil {
			return nil, err
		}
		return p.finishImport(importTok, node)
	default:
		return nil, p.Fail("expected import clause, found " + gast.Describe(p.Current()))
	}
	if p.Match(Comma) {
		switch p.Current().Kind {
		case Asterisk:
			if err := p.parseImportNamespace(node); err != nil {
				return nil, err
			}
		case LBrace:
			if err := p.parseImportSpecs(node); err != nil {
				return nil, err
			}
		default:
			return nil, p.Fail("expected '{' or '*' after ',' in import clause")
		}
	}
	return p.finishImport(importTok, node)
}

func (p *parser) parseImportNamespace(node *ImportDecl) error {
	p.Advance()
	if !p.isWord("as") {
		return p.Fail("expected 'as' after '*' in import clause")
	}
	p.Advance()
	if !p.Check(KindIdent) {
		return p.Fail("expected namespace name after 'as'")
	}
	node.Namespace = p.Advance().Text
	return nil
}

func (p *parser) parseImportSpecs(node *ImportDecl) error {
	p.Advance()
	for !p.Check(RBrace) {
		if p.AtEnd() {
			return p.Fail("expected '}' to close import list")
		}
		imported, err := p.moduleExportName("imported name")
		if err != nil {
			return err
		}
		spec := ImportSpec{Span: imported.Span, Imported: imported.Text, Local: imported.Text}
		if p.isWord("as") {
			p.Advance()
			local, err := p.moduleExportName("local name")
			if err != nil {
				return err
			}
			if local.Kind == KindString {
				return p.FailAt(local.Span, "local import name cannot be a string")
			}
			spec.Local = local.Text
			spec.Span = gast.Merge(imported.Span, local.Span)
		}
		node.Named = append(node.Named, spec)
		if !p.Match(Comma) {
			break
		}
	}
	if _, err := p.Expect(RBrace, "'}' to close import list"); err != nil {
		return err
	}
	return nil
}

// moduleExportName reads a module export name: an identifierName
// (keywords allowed) or a string literal.
func (p *parser) moduleExportName(what string) (gast.Token, error) {
	tok := p.Current()
	if tok.Kind == KindString {
		p.Advance()
		return tok, nil
	}
	if _, ok := keywords[tok.Text]; ok || tok.Kind == KindIdent {
		p.Advance()
		return tok, nil
	}
	return gast.Token{}, p.Fail("expected " + what + ", found " + gast.Describe(tok))
}

func (p *parser) finishImport(importTok gast.Token, node *ImportDecl) (Stmt, error) {
	if !p.isWord("from") {
		return nil, p.Fail("expected 'from' in import declaration")
	}
	p.Advance()
	source, err := p.Expect(KindString, "module specifier")
	if err != nil {
		return nil, err
	}
	node.Source = source.Text
	if err := p.consumeSemicolon(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(importTok.Span)
	return node, nil
}

// parseExportStmt parses every export form: declarations, default
// exports, export lists, and re-exports.
func (p *parser) parseExportStmt() (Stmt, error) {
	exportTok := p.Advance()
	if p.Match(KindDefault) {
		return p.parseExportDefault(exportTok)
	}
	if p.Match(Asterisk) {
		node := &ExportAllStmt{}
		if p.isWord("as") {
			p.Advance()
			name, err := p.moduleExportName("export name")
			if err != nil {
				return nil, err
			}
			node.Exported = name.Text
		}
		source, err := p.parseFromClause()
		if err != nil {
			return nil, err
		}
		node.Source = source
		node.Span = p.SpanFrom(exportTok.Span)
		return node, nil
	}
	if p.Check(LBrace) {
		p.Advance()
		node := &ExportNamedStmt{}
		for !p.Check(RBrace) {
			if p.AtEnd() {
				return nil, p.Fail("expected '}' to close export list")
			}
			local, err := p.moduleExportName("export name")
			if err != nil {
				return nil, err
			}
			spec := ExportSpec{Span: local.Span, Local: local.Text, Exported: local.Text}
			if p.isWord("as") {
				p.Advance()
				exported, err := p.moduleExportName("export name")
				if err != nil {
					return nil, err
				}
				spec.Exported = exported.Text
				spec.Span = gast.Merge(local.Span, exported.Span)
			}
			node.Specs = append(node.Specs, spec)
			if !p.Match(Comma) {
				break
			}
		}
		if _, err := p.Expect(RBrace, "'}' to close export list"); err != nil {
			return nil, err
		}
		if p.isWord("from") {
			source, err := p.parseFromClause()
			if err != nil {
				return nil, err
			}
			node.Source = source
		}
		if err := p.consumeSemicolon(); err != nil {
			return nil, err
		}
		node.Span = p.SpanFrom(exportTok.Span)
		return node, nil
	}
	var decl Stmt
	var err error
	switch {
	case p.isAsyncFunction():
		decl, err = p.parseFunctionDecl(true, true)
	case p.Check(KindVar) || p.Check(KindConst) || p.isLetDecl():
		decl, err = p.parseVarStmt()
	case p.Check(KindFunction):
		decl, err = p.parseFunctionDecl(false, true)
	case p.Check(KindClass):
		decl, err = p.parseClassDecl(true)
	default:
		return nil, p.Fail("expected declaration after 'export', found " + gast.Describe(p.Current()))
	}
	if err != nil {
		return nil, err
	}
	node := &ExportNamedStmt{Span: p.SpanFrom(exportTok.Span), Decl: decl}
	return node, nil
}

func (p *parser) parseFromClause() (string, error) {
	if !p.isWord("from") {
		return "", p.Fail("expected 'from' in export declaration")
	}
	p.Advance()
	source, err := p.Expect(KindString, "module specifier")
	if err != nil {
		return "", err
	}
	return source.Text, nil
}

func (p *parser) parseExportDefault(exportTok gast.Token) (Stmt, error) {
	node := &ExportDefaultStmt{}
	var err error
	switch {
	case p.isAsyncFunction():
		node.Decl, err = p.parseFunctionDecl(true, false)
	case p.Check(KindFunction):
		node.Decl, err = p.parseFunctionDecl(false, false)
	case p.Check(KindClass):
		node.Decl, err = p.parseClassDecl(false)
	default:
		node.Value, err = p.parseAssignExpr(false)
		if err == nil {
			if err = p.checkExprStatement(node.Value); err != nil {
				return nil, err
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if node.Value != nil {
		if err = p.consumeSemicolon(); err != nil {
			return nil, err
		}
	}
	node.Span = p.SpanFrom(exportTok.Span)
	return node, nil
}
