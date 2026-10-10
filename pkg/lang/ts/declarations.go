package ts

import (
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

// parseDecorators parses a run of @expression decorators.
func (p *parser) parseDecorators() ([]Decorator, error) {
	var decorators []Decorator
	for p.Check(At) {
		p.Advance()
		decorator := Decorator{}
		var err error
		if decorator.Expr, err = p.parseLeftHandSide(); err != nil {
			return nil, err
		}
		decorator.Span = decorator.Expr.Pos()
		decorators = append(decorators, decorator)
	}
	return decorators, nil
}

// checkParameterProperties rejects parameter-property modifiers outside
// constructor parameters.
func (p *parser) checkParameterProperties(params []*Param, ctor bool) error {
	if ctor {
		return nil
	}
	for _, param := range params {
		if param.Accessibility != "" || param.Readonly {
			return p.FailAt(param.Span, "parameter properties are only allowed in constructors")
		}
	}
	return nil
}

// parseParams parses a parenthesized parameter list with types,
// defaults, rest, optional markers, this-parameters, parameter-property
// modifiers, and decorators.
func (p *parser) parseParams() ([]*Param, error) {
	if _, err := p.Expect(LParen, "'('"); err != nil {
		return nil, err
	}
	var params []*Param
	for !p.Check(RParen) {
		if p.AtEnd() {
			return nil, p.Fail("expected ')' to close parameter list")
		}
		param, err := p.parseParam()
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

// parseParam parses one parameter, including the TypeScript-only
// modifier forms of constructor parameter properties.
func (p *parser) parseParam() (*Param, error) {
	start := p.Current().Span
	param := &Param{}
	var err error
	if param.Decorators, err = p.parseDecorators(); err != nil {
		return nil, err
	}
	for _, word := range []string{"public", "private", "protected"} {
		if p.isWord(word) && p.paramBindingAhead(1) {
			param.Accessibility = p.Advance().Text
			break
		}
	}
	if p.isWord("readonly") && p.paramBindingAhead(1) {
		param.Readonly = true
		p.Advance()
	}
	if p.Match(Ellipsis) {
		param.Rest = true
	}
	if p.Check(KindThis) && p.CheckAhead(1, Colon) {
		thisTok := p.Advance()
		param.This = true
		param.Binding = &ThisExpr{Span: thisTok.Span}
	} else if param.Binding, err = p.parseBinding(); err != nil {
		return nil, err
	}
	if p.Match(Question) {
		param.Optional = true
	}
	if p.Match(Colon) {
		if param.Type, err = p.parseType(); err != nil {
			return nil, err
		}
	}
	if p.Match(Assign) {
		if param.Default, err = p.parseAssignExpr(false); err != nil {
			return nil, err
		}
	}
	param.Span = p.SpanFrom(start)
	return param, nil
}

// paramBindingAhead reports whether the token ahead positions from the
// cursor continues a parameter binding, distinguishing a
// parameter-property modifier from a parameter named like one.
func (p *parser) paramBindingAhead(ahead int) bool {
	switch p.Peek(ahead).Kind {
	case KindIdent, LBracket, LBrace, Ellipsis, KindThis:
		return true
	case At:
		return true
	}
	return p.isWordAt(p.Peek(ahead), "readonly")
}

// parseFunctionDecl parses a function declaration; async marks an
// `async function` declaration and declare an ambient one. requireName
// is false only for export default, where the name is optional.
func (p *parser) parseFunctionDecl(async, declare, requireName bool) (Stmt, error) {
	fn, err := p.parseFunction(async, declare, requireName)
	if err != nil {
		return nil, err
	}
	decl := &FunctionDecl{
		Span: fn.Span, Async: fn.Async, Generator: fn.Generator, Declare: fn.Declare,
		Name: fn.Name, TypeParams: fn.TypeParams, Params: fn.Params,
		ReturnType: fn.ReturnType, Body: fn.Body,
	}
	return decl, nil
}

// parseFunctionExpr parses a function expression; async is set by the
// caller when the expression follows an async keyword.
func (p *parser) parseFunctionExpr(async bool) (Expr, error) {
	fn, err := p.parseFunction(async, false, false)
	if err != nil {
		return nil, err
	}
	return &FunctionExpr{
		Span: fn.Span, Async: fn.Async, Generator: fn.Generator, Name: fn.Name,
		TypeParams: fn.TypeParams, Params: fn.Params, ReturnType: fn.ReturnType,
		Body: fn.Body,
	}, nil
}

// fnParts is the shared shape of function declarations and
// expressions.
type fnParts struct {
	gast.Span
	Declare    bool
	Async      bool
	Generator  bool
	Name       string
	TypeParams []*TypeParam
	Params     []*Param
	ReturnType TypeNode
	Body       *BlockStmt
}

// parseFunction consumes `function [*] [name] [<T>] (params) [: ret]`
// followed by a block body or, for overloads and ambient declarations,
// just a semicolon.
func (p *parser) parseFunction(async, declare, requireName bool) (*fnParts, error) {
	start := p.Current().Span
	if async {
		start = p.Advance().Span // async
	}
	if _, err := p.Expect(KindFunction, "'function'"); err != nil {
		return nil, err
	}
	fn := &fnParts{Async: async, Declare: declare}
	if p.Match(Asterisk) {
		fn.Generator = true
	}
	if p.Check(KindIdent) {
		fn.Name = p.Advance().Text
	} else if requireName {
		return nil, p.Fail("expected function name, found " + gast.Describe(p.Current()))
	}
	var err error
	if fn.TypeParams, err = p.parseTypeParams(); err != nil {
		return nil, err
	}
	if fn.Params, err = p.parseParams(); err != nil {
		return nil, err
	}
	if err := p.checkParameterProperties(fn.Params, false); err != nil {
		return nil, err
	}
	if p.Match(Colon) {
		if fn.ReturnType, err = p.parseReturnType(); err != nil {
			return nil, err
		}
	}
	if p.Check(LBrace) {
		savedAsync, savedGen, savedFn := p.inAsync, p.inGenerator, p.inFunction
		p.inAsync, p.inGenerator, p.inFunction = async, fn.Generator, true
		fn.Body, err = p.parseBlockStmt()
		p.inAsync, p.inGenerator, p.inFunction = savedAsync, savedGen, savedFn
		if err != nil {
			return nil, err
		}
	} else if err := p.consumeSemicolon(); err != nil {
		return nil, err
	}
	fn.Span = p.SpanFrom(start)
	return fn, nil
}

// parseClassDecl parses a class declaration; requireName is false only
// for export default.
func (p *parser) parseClassDecl(decorators []Decorator, abstract, declare, requireName bool) (Stmt, error) {
	node, err := p.parseClass(decorators, abstract, requireName)
	if err != nil {
		return nil, err
	}
	decl := &ClassDecl{
		Span: node.Span, Decorators: node.Decorators, Abstract: node.Abstract,
		Declare: declare, Name: node.Name, TypeParams: node.TypeParams,
		Extends: node.Extends, Implements: node.Implements, Body: node.Body,
	}
	return decl, nil
}

// parseClassExpr parses a class expression.
func (p *parser) parseClassExpr() (Expr, error) {
	node, err := p.parseClass(nil, false, false)
	if err != nil {
		return nil, err
	}
	return &ClassExpr{
		Span: node.Span, Decorators: node.Decorators, Abstract: node.Abstract,
		Name: node.Name, TypeParams: node.TypeParams, Extends: node.Extends,
		Implements: node.Implements, Body: node.Body,
	}, nil
}

// classParts is the shared shape of class declarations and
// expressions.
type classParts struct {
	gast.Span
	Decorators []Decorator
	Abstract   bool
	Name       string
	TypeParams []*TypeParam
	Extends    Expr
	Implements []TypeNode
	Body       []ClassMember
}

// parseClass consumes `[decorators] [abstract] class [name] [<T>]\n
// [extends expr] [implements types] { members }`.
func (p *parser) parseClass(decorators []Decorator, abstract, requireName bool) (*classParts, error) {
	if abstract {
		p.Advance() // the abstract keyword
	}
	classTok := p.Advance()
	node := &classParts{Decorators: decorators, Abstract: abstract}
	if p.Check(KindIdent) {
		node.Name = p.Advance().Text
	} else if requireName {
		return nil, p.Fail("expected class name, found " + gast.Describe(p.Current()))
	}
	var err error
	if node.TypeParams, err = p.parseTypeParams(); err != nil {
		return nil, err
	}
	if p.Match(KindExtends) {
		if node.Extends, err = p.parseLeftHandSide(); err != nil {
			return nil, err
		}
	}
	if p.isWord("implements") {
		p.Advance()
		for {
			var iface TypeNode
			if iface, err = p.parseType(); err != nil {
				return nil, err
			}
			node.Implements = append(node.Implements, iface)
			if !p.Match(Comma) {
				break
			}
		}
	}
	if _, err := p.Expect(LBrace, "'{' to open class body"); err != nil {
		return nil, err
	}
	if node.Body, err = p.parseClassBody(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(classTok.Span)
	return node, nil
}

// parseClassBody parses members until the closing brace. Fields end by
// semicolon, newline (ASI), or brace; methods carry their own bodies,
// which overload, abstract, and ambient signatures omit.
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

// isModifier reports whether the token kind after a modifier word can
// begin a member: modifiers are plain names when a "(", "=", ":", "?",
// ";", "}", or "!" follows, as in a member named "static".
func isModifier(kind string) bool {
	if kind == Asterisk {
		return true
	}
	return isClassKeyStart(kind)
}

// parseClassMember parses one class member with TypeScript's modifier,
// decorator, and signature forms.
func (p *parser) parseClassMember() (ClassMember, error) {
	start := p.Current().Span
	decorators, err := p.parseDecorators()
	if err != nil {
		return nil, err
	}
	static, abstract, declare := false, false, false
	accessibility, readonly := "", false
	if p.isWord("declare") && isModifier(p.Peek(1).Kind) {
		declare = true
		p.Advance()
	}
	for p.isWord("static") && isModifier(p.Peek(1).Kind) {
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
	if p.isWord("abstract") && isModifier(p.Peek(1).Kind) {
		abstract = true
		p.Advance()
	}
	for _, word := range []string{"public", "private", "protected"} {
		if p.isWord(word) && isModifier(p.Peek(1).Kind) {
			accessibility = p.Advance().Text
			break
		}
	}
	if p.isWord("readonly") && isModifier(p.Peek(1).Kind) {
		readonly = true
		p.Advance()
	}
	async := false
	if p.isWord("async") && isModifier(p.Peek(1).Kind) &&
		!p.lineBreakBetween(p.Current(), p.Peek(1)) {
		async = true
		p.Advance()
	}
	generator := p.Match(Asterisk)
	if (p.isWord("get") || p.isWord("set")) && isModifier(p.Peek(1).Kind) {
		accessor := p.Advance().Text
		key, computed, err := p.parsePropertyKey()
		if err != nil {
			return nil, err
		}
		if computed || generator || async {
			return nil, p.FailAt(p.SpanFrom(start), "accessor cannot be computed, async, or a generator")
		}
		return p.parseAccessorMember(start, decorators, key, accessor, static, accessibility)
	}
	// Index signature member: [name: KeyType]: Type.
	if p.Check(LBracket) && p.CheckAhead(1, KindIdent) && p.CheckAhead(2, Colon) {
		return p.parseClassIndexSig(start, readonly)
	}
	key, computed, err := p.parsePropertyKey()
	if err != nil {
		return nil, err
	}
	optional, definite := false, false
	if p.Match(Question) {
		optional = true
	} else if p.Match(Bang) {
		definite = true
	}
	if p.Check(Colon) || (definite && p.Check(Assign)) || p.Check(Assign) {
		node := &PropertyDef{
			Decorators: decorators, Key: key, Static: static, Computed: computed,
			Declare: declare, Abstract: abstract, Definite: definite,
			Optional: optional, Readonly: readonly, Accessibility: accessibility,
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
		}
		if err := p.consumeSemicolon(); err != nil {
			return nil, err
		}
		node.Span = p.SpanFrom(start)
		return node, nil
	}
	savedAsync2, savedGen2, savedFn2 := p.inAsync, p.inGenerator, p.inFunction
	p.inAsync, p.inGenerator, p.inFunction = async, generator, true
	defer func() { p.inAsync, p.inGenerator, p.inFunction = savedAsync2, savedGen2, savedFn2 }()
	typeParams, err := p.parseTypeParams()
	if err != nil {
		return nil, err
	}
	if !p.Check(LParen) {
		if accessor, ok := key.(*IdentExpr); ok && accessor.Name != "" {
			return nil, p.Fail("expected '(' or ':' after member '" + accessor.Name + "'")
		}
		return nil, p.Fail("expected '(' or ':' after class member")
	}
	var params []*Param
	if params, err = p.parseParams(); err != nil {
		return nil, err
	}
	var returnType TypeNode
	if p.Match(Colon) {
		if returnType, err = p.parseReturnType(); err != nil {
			return nil, err
		}
	}
	var body *BlockStmt
	if p.Check(LBrace) {
		if body, err = p.parseBlockStmt(); err != nil {
			return nil, err
		}
	} else if err := p.consumeSemicolon(); err != nil {
		return nil, err
	}
	if abstract && body != nil {
		return nil, p.FailAt(p.SpanFrom(start), "abstract member cannot have a body")
	}
	kind := "method"
	if !static && !computed {
		if ident, ok := key.(*IdentExpr); ok && ident.Name == "constructor" {
			kind = "constructor"
		} else if lit, ok := key.(*StringLit); ok && lit.Text == `"constructor"` {
			kind = "constructor"
		}
	}
	if err := p.checkParameterProperties(params, kind == "constructor"); err != nil {
		return nil, err
	}
	if async && kind == "constructor" {
		return nil, p.FailAt(p.SpanFrom(start), "constructor cannot be async")
	}
	if generator && kind == "constructor" {
		return nil, p.FailAt(p.SpanFrom(start), "constructor cannot be a generator")
	}
	return &MethodDef{
		Span: p.SpanFrom(start), Decorators: decorators, Key: key, Static: static,
		Async: async, Generator: generator, Abstract: abstract, Optional: optional,
		Accessibility: accessibility, Kind: kind, TypeParams: typeParams,
		Params: params, ReturnType: returnType, Body: body,
	}, nil
}

// parseAccessorMember parses a getter or setter after its key.
func (p *parser) parseAccessorMember(start gast.Span, decorators []Decorator, key Expr, accessor string, static bool, accessibility string) (ClassMember, error) {
	savedAsync, savedGen, savedFn := p.inAsync, p.inGenerator, p.inFunction
	p.inAsync, p.inGenerator, p.inFunction = false, false, true
	params, err := p.parseParams()
	var returnType TypeNode
	if err == nil && p.Match(Colon) {
		returnType, err = p.parseReturnType()
	}
	var body *BlockStmt
	if err == nil && p.Check(LBrace) {
		body, err = p.parseBlockStmt()
	}
	defer func() { p.inAsync, p.inGenerator, p.inFunction = savedAsync, savedGen, savedFn }()
	if err != nil {
		return nil, err
	}
	if accessor == "get" && len(params) != 0 {
		return nil, p.FailAt(p.SpanFrom(start), "getter cannot take parameters")
	}
	if accessor == "set" && len(params) != 1 {
		return nil, p.FailAt(p.SpanFrom(start), "setter takes exactly one parameter")
	}
	return &MethodDef{
		Span: p.SpanFrom(start), Decorators: decorators, Key: key, Static: static,
		Kind: accessor, Accessibility: accessibility, Params: params,
		ReturnType: returnType, Body: body,
	}, nil
}

// parseClassIndexSig parses a class index-signature member.
func (p *parser) parseClassIndexSig(start gast.Span, readonly bool) (ClassMember, error) {
	p.Advance() // '['
	node := &ClassIndexSig{Readonly: readonly}
	name, err := p.Expect(KindIdent, "index signature name")
	if err != nil {
		return nil, err
	}
	node.KeyName = name.Text
	if _, err := p.Expect(Colon, "':' in index signature"); err != nil {
		return nil, err
	}
	if node.KeyType, err = p.parseType(); err != nil {
		return nil, err
	}
	if _, err := p.Expect(RBracket, "']' to close index signature"); err != nil {
		return nil, err
	}
	if _, err := p.Expect(Colon, "':' after index signature"); err != nil {
		return nil, err
	}
	if node.Type, err = p.parseType(); err != nil {
		return nil, err
	}
	if err := p.consumeSemicolon(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// parsePropertyKey parses an identifier, private name, string, number,
// or computed key; the bool reports the computed form.
func (p *parser) parsePropertyKey() (Expr, bool, error) {
	tok := p.Current()
	if isReservedWord(tok) && tok.Kind != KindTrue && tok.Kind != KindFalse && tok.Kind != KindNull {
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
		return &StringLit{Span: tok.Span, Text: tok.Text}, false, nil
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

// parseInterfaceDecl parses an interface declaration.
func (p *parser) parseInterfaceDecl(declare bool) (Stmt, error) {
	start := p.Current().Span
	p.Advance() // 'interface'
	node := &InterfaceDecl{Declare: declare}
	node.Name = p.Advance().Text
	var err error
	if node.TypeParams, err = p.parseTypeParams(); err != nil {
		return nil, err
	}
	if p.Match(KindExtends) {
		for {
			var iface TypeNode
			if iface, err = p.parseType(); err != nil {
				return nil, err
			}
			node.Extends = append(node.Extends, iface)
			if !p.Match(Comma) {
				break
			}
		}
	}
	if _, err := p.Expect(LBrace, "'{' to open interface body"); err != nil {
		return nil, err
	}
	if node.Body, err = p.parseTypeMembers(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// parseTypeAliasDecl parses a type alias declaration.
func (p *parser) parseTypeAliasDecl(declare bool) (Stmt, error) {
	start := p.Current().Span
	p.Advance() // 'type'
	node := &TypeAliasDecl{Declare: declare}
	node.Name = p.Advance().Text
	var err error
	if node.TypeParams, err = p.parseTypeParams(); err != nil {
		return nil, err
	}
	if _, err := p.Expect(Assign, "'=' in type alias"); err != nil {
		return nil, err
	}
	if node.Type, err = p.parseType(); err != nil {
		return nil, err
	}
	if err := p.consumeSemicolon(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// parseEnumDecl parses a (possibly const) enum declaration; the
// current token is the const or enum keyword.
func (p *parser) parseEnumDecl() (Stmt, error) {
	start := p.Current().Span
	node := &EnumDecl{}
	if p.Check(KindConst) {
		node.Const = true
		p.Advance()
	}
	p.Advance() // 'enum'
	node.Name = p.Advance().Text
	if _, err := p.Expect(LBrace, "'{' to open enum body"); err != nil {
		return nil, err
	}
	for !p.Check(RBrace) {
		if p.AtEnd() {
			return nil, p.Fail("expected '}' to close enum body")
		}
		member := &EnumMember{}
		name, computed, err := p.parsePropertyKey()
		if err != nil {
			return nil, err
		}
		if computed {
			return nil, p.FailAt(name.Pos(), "computed enum member names must be simple")
		}
		member.Name = name
		if p.Match(Assign) {
			if member.Init, err = p.parseAssignExpr(false); err != nil {
				return nil, err
			}
		}
		member.Span = name.Pos()
		node.Members = append(node.Members, member)
		if !p.Match(Comma) && !p.Match(Semicolon) {
			break
		}
	}
	close, err := p.Expect(RBrace, "'}' to close enum body")
	if err != nil {
		return nil, err
	}
	node.Span = gast.Merge(start, close.Span)
	return node, nil
}

// parseDeclareEnum parses an ambient enum declaration.
func (p *parser) parseDeclareEnum() (Stmt, error) {
	stmt, err := p.parseEnumDecl()
	if err != nil {
		return nil, err
	}
	if enum, ok := stmt.(*EnumDecl); ok {
		enum.Declare = true
	}
	return stmt, nil
}

// parseNamespaceDecl parses a namespace or module declaration with a
// dotted name and a declaration body.
func (p *parser) parseNamespaceDecl(declare bool) (Stmt, error) {
	start := p.Current().Span
	p.Advance() // 'namespace' or 'module' or 'global'
	node := &NamespaceDecl{Declare: declare}
	if p.Check(KindString) {
		node.Name = p.Advance().Text
	} else {
		var builder strings.Builder
		builder.WriteString(p.Advance().Text)
		for p.Match(Dot) {
			part, err := p.Expect(KindIdent, "namespace name after '.'")
			if err != nil {
				return nil, err
			}
			builder.WriteByte('.')
			builder.WriteString(part.Text)
		}
		node.Name = builder.String()
	}
	var err error
	if _, err = p.Expect(LBrace, "'{' to open namespace body"); err != nil {
		return nil, err
	}
	if node.Body, err = p.parseStmtList(RBrace); err != nil {
		return nil, err
	}
	close, err := p.Expect(RBrace, "'}' to close namespace body")
	if err != nil {
		return nil, err
	}
	node.Span = gast.Merge(start, close.Span)
	return node, nil
}

// parseDeclareStmt parses an ambient declaration; the 'declare'
// contextual keyword is current.
func (p *parser) parseDeclareStmt() (Stmt, error) {
	p.Advance() // 'declare'
	switch {
	case p.Check(KindVar) || p.isWord("let"):
		return p.parseVarStmt(true)
	case p.Check(KindConst):
		if p.wordAt(1, "enum") {
			return p.parseDeclareEnum()
		}
		return p.parseVarStmt(true)
	case p.isWord("enum"):
		return p.parseDeclareEnum()
	case p.isAsyncFunction():
		return p.parseFunctionDecl(true, true, true)
	case p.Check(KindFunction):
		return p.parseFunctionDecl(false, true, true)
	case p.Check(KindClass):
		return p.parseClassDecl(nil, false, true, true)
	case p.isWord("abstract") && p.CheckAhead(1, KindClass):
		return p.parseClassDecl(nil, true, true, true)
	case p.isWord("enum"):
		return p.parseDeclareEnum()
	case p.isWord("namespace") || p.isWord("module") || p.isWord("global"):
		stmt, err := p.parseNamespaceDecl(true)
		if err != nil {
			return nil, err
		}
		return stmt, nil
	case p.isWord("interface"):
		return p.parseInterfaceDecl(true)
	case p.isTypeAliasStart():
		return p.parseTypeAliasDecl(true)
	}
	return nil, p.Fail("expected declaration after 'declare', found " + gast.Describe(p.Current()))
}

// parseImportDecl parses every import form: side-effect-only,
// default, namespace, named (with renames and string names), their
// combinations, TypeScript's `import type`, and the equals forms
// `import x = require("m")` / `import x = A.B.C`.
func (p *parser) parseImportDecl() (Stmt, error) {
	importTok := p.Advance()
	node := &ImportDecl{}
	if p.isWord("type") {
		switch p.Peek(1).Kind {
		case KindString:
			// `import type "m";` is not valid; fall through to error
		case KindIdent:
			if p.CheckAhead(2, Assign) {
				return p.parseImportEquals(importTok, true)
			}
			if !p.wordAt(2, "from") && !p.CheckAhead(2, Comma) {
				// `type` is the default binding name
				break
			}
			node.TypeOnly = true
			p.Advance()
		case Asterisk, LBrace:
			node.TypeOnly = true
			p.Advance()
		}
	}
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
		if p.CheckAhead(1, Assign) {
			return p.parseImportEquals(importTok, node.TypeOnly)
		}
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

// parseImportEquals parses `import [type] name = require(source)` or
// `import [type] name = A.B.C`.
func (p *parser) parseImportEquals(importTok gast.Token, typeOnly bool) (Stmt, error) {
	node := &ImportEqualsDecl{TypeOnly: typeOnly}
	node.Name = p.Advance().Text
	if _, err := p.Expect(Assign, "'=' in import equals declaration"); err != nil {
		return nil, err
	}
	if p.isWord("require") && p.CheckAhead(1, LParen) {
		p.Advance()
		p.Advance()
		source, err := p.Expect(KindString, "module specifier")
		if err != nil {
			return nil, err
		}
		if _, err := p.Expect(RParen, "')' after require"); err != nil {
			return nil, err
		}
		node.Require = true
		node.Source = source.Text
	} else {
		var err error
		if node.Target, err = p.parseLeftHandSide(); err != nil {
			return nil, err
		}
	}
	if err := p.consumeSemicolon(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(importTok.Span)
	return node, nil
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
		p.matchTypeOnlyMarker()
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

// matchTypeOnlyMarker consumes an inline `type` marker in an import
// or export specifier list when the following tokens prove it is a
// marker rather than the imported name itself.
func (p *parser) matchTypeOnlyMarker() {
	if !p.isWord("type") {
		return
	}
	switch p.Peek(1).Kind {
	case KindIdent, KindString:
	default:
		return
	}
	switch p.Peek(2).Kind {
	case Comma, RBrace:
		p.Advance()
	case KindIdent:
		if p.wordAt(2, "as") {
			p.Advance()
		}
	}
}

// moduleExportName reads a module export name: an identifierName
// (keywords allowed) or a string literal.
func (p *parser) moduleExportName(what string) (gast.Token, error) {
	tok := p.Current()
	if tok.Kind == KindString {
		p.Advance()
		return tok, nil
	}
	if isReservedWord(tok) || tok.Kind == KindIdent {
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
// exports, export lists, re-exports, TypeScript's `export type`, and
// the ambient `export = expression`.
func (p *parser) parseExportStmt() (Stmt, error) {
	return p.parseExportDecorated(nil)
}

// parseExportDecorated parses an export statement, attaching class
// decorators collected before the export keyword when given.
func (p *parser) parseExportDecorated(decorators []Decorator) (Stmt, error) {
	exportTok := p.Advance()
	if p.Match(Assign) {
		expr, err := p.parseAssignExpr(false)
		if err != nil {
			return nil, err
		}
		if err := p.consumeSemicolon(); err != nil {
			return nil, err
		}
		return &ExportAssignStmt{Span: p.SpanFrom(exportTok.Span), Expr: expr}, nil
	}
	if p.Match(KindDefault) {
		return p.parseExportDefault(exportTok)
	}
	typeOnly := false
	if p.isWord("type") {
		switch p.Peek(1).Kind {
		case LBrace:
			typeOnly = true
			p.Advance()
		case Asterisk:
			p.Advance() // `export type * from` re-exports types only
		}
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
		node := &ExportNamedStmt{TypeOnly: typeOnly}
		for !p.Check(RBrace) {
			if p.AtEnd() {
				return nil, p.Fail("expected '}' to close export list")
			}
			p.matchTypeOnlyMarker()
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
	case p.isDeclareStart():
		decl, err = p.parseDeclareStmt()
	case p.isWord("abstract") && p.CheckAhead(1, KindClass):
		decl, err = p.parseClassDecl(decorators, true, false, true)
	case p.isAsyncFunction():
		decl, err = p.parseFunctionDecl(true, false, true)
	case p.Check(KindVar) || p.isLetDecl():
		decl, err = p.parseVarStmt(false)
	case p.isEnumStart():
		decl, err = p.parseEnumDecl()
	case p.Check(KindConst):
		decl, err = p.parseVarStmt(false)
	case p.Check(KindFunction):
		decl, err = p.parseFunctionDecl(false, false, true)
	case p.Check(KindClass):
		decl, err = p.parseClassDecl(decorators, false, false, true)
	case p.isTypeAliasStart():
		decl, err = p.parseTypeAliasDecl(false)
	case p.isWord("interface") && p.CheckAhead(1, KindIdent):
		decl, err = p.parseInterfaceDecl(false)
	case p.isEnumStart():
		decl, err = p.parseEnumDecl()
	case p.isNamespaceStart():
		decl, err = p.parseNamespaceDecl(false)
	default:
		return nil, p.Fail("expected declaration after 'export', found " + gast.Describe(p.Current()))
	}
	if err != nil {
		return nil, err
	}
	node := &ExportNamedStmt{Span: p.SpanFrom(exportTok.Span), TypeOnly: typeOnly, Decl: decl}
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
		node.Decl, err = p.parseFunctionDecl(true, false, false)
	case p.Check(KindFunction):
		node.Decl, err = p.parseFunctionDecl(false, false, false)
	case p.isWord("abstract") && p.CheckAhead(1, KindClass):
		node.Decl, err = p.parseClassDecl(nil, true, false, false)
	case p.Check(KindClass):
		node.Decl, err = p.parseClassDecl(nil, false, false, false)
	case p.isWord("interface") && p.CheckAhead(1, KindIdent):
		return nil, p.FailAt(exportTok.Span, "interface cannot be a default export")
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
