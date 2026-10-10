package php

import (
	"slices"
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

// parseNameListAfter consumes keyword and parses a comma-separated name
// list if it was present.
func (p *parser) parseNameListAfter(keyword string) ([]Name, error) {
	if !p.Match(keyword) {
		return nil, nil
	}
	return p.parseNameList()
}

func (p *parser) parseFunctionStmt(attrs []*AttributeGroup) (Stmt, error) {
	fnTok := p.Advance()
	ref := p.Match(Ampersand)
	name, err := p.Expect(KindIdent, "function name")
	if err != nil {
		return nil, err
	}
	node := &FunctionStmt{Attrs: attrs, Ref: ref, Name: name.Text}
	if node.Params, err = p.parseParams(); err != nil {
		return nil, err
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

func (p *parser) parseParams() ([]*Param, error) {
	if _, err := p.Expect(LParen, "'('"); err != nil {
		return nil, err
	}
	var params []*Param
	for !p.Check(RParen) {
		param := &Param{}
		var err error
		if param.Attrs, err = p.parseAttributeGroups(); err != nil {
			return nil, err
		}
		param.Promoted = p.parseModifiers(KindPublic, KindPrivate, KindProtected)
		if p.isTypeStart() {
			if param.Type, err = p.parseType(); err != nil {
				return nil, err
			}
		}
		param.ByRef = p.Match(Ampersand)
		param.Variadic = p.Match(Ellipsis)
		varTok, err := p.Expect(KindVariable, "parameter variable")
		if err != nil {
			return nil, err
		}
		param.Var = variable(varTok)
		if p.Match(Assign) {
			if param.Default, err = p.parseExpr(); err != nil {
				return nil, err
			}
		}
		param.Span = p.SpanFrom(varTok.Span)
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

// parseModifiers collects modifier keywords, including the contextual
// readonly when a declaration continues after it. kinds are the
// reserved modifiers accepted in this position.
func (p *parser) parseModifiers(kinds ...string) []string {
	var modifiers []string
	for {
		if slices.Contains(kinds, p.Current().Kind) {
			modifiers = append(modifiers, p.Advance().Kind)
			continue
		}
		if p.isContextualIdent(wordReadonly) && p.readonlyPrecedesDeclaration() {
			modifiers = append(modifiers, wordReadonly)
			p.Advance()
			continue
		}
		return modifiers
	}
}

// readonlyPrecedesDeclaration guards the soft keyword readonly: it is a
// modifier only when something readonly can annotate follows.
func (p *parser) readonlyPrecedesDeclaration() bool {
	switch p.Peek(1).Kind {
	case KindVariable, Dollar, Ampersand, Ellipsis, KindFunction, KindConst, KindClass:
		return true
	}
	return p.isTypeStartAt(1)
}

func (p *parser) parseClassStmt(attrs []*AttributeGroup) (Stmt, error) {
	modifiers := p.parseModifiers(KindAbstract, KindFinal)
	classTok, err := p.Expect(KindClass, "'class'")
	if err != nil {
		return nil, err
	}
	return p.parseClassTail(attrs, modifiers, classTok)
}

func (p *parser) parseClassTail(attrs []*AttributeGroup, modifiers []string, classTok gast.Token) (Stmt, error) {
	name, err := p.Expect(KindIdent, "class name")
	if err != nil {
		return nil, err
	}
	node := &ClassStmt{Attrs: attrs, Modifiers: modifiers, Name: name.Text}
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
	node.Span = p.SpanFrom(classTok.Span)
	return node, nil
}

func (p *parser) parseInterfaceStmt() (Stmt, error) {
	ifaceTok := p.Advance()
	name, err := p.Expect(KindIdent, "interface name")
	if err != nil {
		return nil, err
	}
	node := &InterfaceStmt{Name: name.Text}
	if node.Extends, err = p.parseNameListAfter(KindExtends); err != nil {
		return nil, err
	}
	if node.Members, err = p.parseMemberBlock(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(ifaceTok.Span)
	return node, nil
}

func (p *parser) parseTraitStmt() (Stmt, error) {
	traitTok := p.Advance()
	name, err := p.Expect(KindIdent, "trait name")
	if err != nil {
		return nil, err
	}
	node := &TraitStmt{Name: name.Text}
	if node.Members, err = p.parseMemberBlock(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(traitTok.Span)
	return node, nil
}

func (p *parser) parseEnumStmt(attrs []*AttributeGroup) (Stmt, error) {
	enumTok := p.Advance()
	name, err := p.Expect(KindIdent, "enum name")
	if err != nil {
		return nil, err
	}
	node := &EnumStmt{Attrs: attrs, Name: name.Text}
	if p.Match(Colon) {
		if node.ScalarType, err = p.parseType(); err != nil {
			return nil, err
		}
	}
	if node.Implements, err = p.parseNameListAfter(KindImplements); err != nil {
		return nil, err
	}
	if node.Members, err = p.parseMemberBlock(); err != nil {
		return nil, err
	}
	if node.ScalarType != nil {
		for _, member := range node.Members {
			enumCase, ok := member.(*EnumCase)
			if ok && enumCase.Value == nil {
				return nil, p.FailAt(enumCase.Span, "case of backed enum must have a value")
			}
		}
	}
	node.Span = p.SpanFrom(enumTok.Span)
	return node, nil
}

func (p *parser) parseMemberBlock() ([]ClassMember, error) {
	if _, err := p.Expect(LBrace, "'{'"); err != nil {
		return nil, err
	}
	var members []ClassMember
	for !p.Check(RBrace) {
		if p.Match(Semicolon) {
			continue
		}
		member, err := p.parseClassMember()
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	if _, err := p.Expect(RBrace, "'}' to close class body"); err != nil {
		return nil, err
	}
	return members, nil
}

func (p *parser) parseClassMember() (ClassMember, error) {
	attrs, err := p.parseAttributeGroups()
	if err != nil {
		return nil, err
	}
	memberStart := p.Current().Span
	modifiers := p.parseModifiers(
		KindPublic, KindPrivate, KindProtected, KindStatic, KindAbstract, KindFinal, KindVar,
	)
	switch p.Current().Kind {
	case KindFunction:
		return p.parseMethod(attrs, modifiers)
	case KindConst:
		return p.parseClassConst(attrs, modifiers)
	case KindUse:
		return p.parseTraitUse()
	case KindCase:
		return p.parseEnumCase(attrs)
	case KindVariable:
		return p.parseProperty(attrs, modifiers, nil, memberStart)
	}
	if !p.isTypeStart() {
		return nil, p.Fail("expected class member, found " + gast.Describe(p.Current()))
	}
	memberType, err := p.parseType()
	if err != nil {
		return nil, err
	}
	return p.parseProperty(attrs, modifiers, memberType, memberStart)
}

func (p *parser) parseMethod(attrs []*AttributeGroup, modifiers []string) (ClassMember, error) {
	fnTok := p.Advance()
	ref := p.Match(Ampersand)
	name, err := p.memberName("method name")
	if err != nil {
		return nil, err
	}
	node := &Method{Attrs: attrs, Modifiers: modifiers, Ref: ref, Name: name.Text}
	if node.Params, err = p.parseParams(); err != nil {
		return nil, err
	}
	if p.Match(Colon) {
		if node.ReturnType, err = p.parseType(); err != nil {
			return nil, err
		}
	}
	if p.Match(Semicolon) || p.Match(KindCloseTag) {
		node.Span = p.SpanFrom(fnTok.Span)
		return node, nil
	}
	if node.Body, err = p.parseBlockStmt(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(fnTok.Span)
	return node, nil
}

// memberName accepts identifiers and any keyword, since PHP allows
// reserved words as member names.
func (p *parser) memberName(description string) (gast.Token, error) {
	tok := p.Current()
	if tok.Kind == KindIdent {
		return p.Advance(), nil
	}
	if tok.Text != "" {
		if kind, ok := keywords[strings.ToLower(tok.Text)]; ok && kind == tok.Kind {
			return p.Advance(), nil
		}
	}
	return gast.Token{}, p.Fail("expected " + description + ", found " + gast.Describe(tok))
}

func (p *parser) parseClassConst(attrs []*AttributeGroup, modifiers []string) (ClassMember, error) {
	constTok := p.Advance()
	node := &ClassConst{Attrs: attrs, Modifiers: modifiers}
	var err error
	if p.isConstType() {
		if node.Type, err = p.parseType(); err != nil {
			return nil, err
		}
	}
	if node.Consts, err = p.parseConstEntries(); err != nil {
		return nil, err
	}
	if err := p.endStatement(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(constTok.Span)
	return node, nil
}

// isConstType reports whether a type precedes the constant names: a
// type start that is not immediately followed by =, ',' or ';', which
// would make it the constant name itself.
func (p *parser) isConstType() bool {
	if !p.isTypeStart() {
		return false
	}
	switch p.Peek(1).Kind {
	case Assign, Comma, Semicolon:
		return false
	}
	return true
}

func (p *parser) parseConstEntries() ([]ConstEntry, error) {
	var entries []ConstEntry
	for {
		name, err := p.Expect(KindIdent, "constant name")
		if err != nil {
			return nil, err
		}
		if _, err := p.Expect(Assign, "'=' in constant declaration"); err != nil {
			return nil, err
		}
		value, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		entries = append(entries, ConstEntry{Span: gast.Merge(name.Span, value.Pos()), Name: name.Text, Value: value})
		if !p.Match(Comma) {
			return entries, nil
		}
	}
}

func (p *parser) parseEnumCase(attrs []*AttributeGroup) (ClassMember, error) {
	caseTok := p.Advance()
	name, err := p.Expect(KindIdent, "enum case name")
	if err != nil {
		return nil, err
	}
	node := &EnumCase{Attrs: attrs, Name: name.Text}
	if p.Match(Assign) {
		if node.Value, err = p.parseExpr(); err != nil {
			return nil, err
		}
	}
	if err := p.endStatement(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(caseTok.Span)
	return node, nil
}

func (p *parser) parseTraitUse() (ClassMember, error) {
	useTok := p.Advance()
	node := &TraitUse{}
	var err error
	if node.Traits, err = p.parseNameList(); err != nil {
		return nil, err
	}
	if p.Match(LBrace) {
		for !p.Check(RBrace) {
			method, err := p.parseMethodRef()
			if err != nil {
				return nil, err
			}
			var adaptation TraitAdaptation
			switch p.Current().Kind {
			case KindInsteadOf:
				p.Advance()
				traits, err := p.parseNameList()
				if err != nil {
					return nil, err
				}
				adaptation = &TraitInsteadOf{Span: p.SpanFrom(method.Span), Method: method, InsteadOf: traits}
			case KindAs:
				asTok := p.Advance()
				entry := &TraitAs{Method: method}
				if p.isVisibility() {
					entry.Visibility = p.Advance().Kind
				}
				if p.Check(KindIdent) {
					entry.Alias = p.Advance().Text
				}
				if entry.Visibility == "" && entry.Alias == "" {
					return nil, p.FailAt(asTok.Span, "expected alias or visibility after 'as'")
				}
				adaptation = entry
			default:
				return nil, p.Fail("expected 'insteadof' or 'as' in trait adaptation")
			}
			if err := p.endStatement(); err != nil {
				return nil, err
			}
			node.Adaptations = append(node.Adaptations, adaptation)
		}
		if _, err := p.Expect(RBrace, "'}' to close trait adaptations"); err != nil {
			return nil, err
		}
	} else if err := p.endStatement(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(useTok.Span)
	return node, nil
}

// parseMethodRef parses `method` or `Trait::method` in a trait
// adaptation.
func (p *parser) parseMethodRef() (MethodRef, error) {
	start := p.Current().Span
	first, err := p.memberName("method name")
	if err != nil {
		return MethodRef{}, err
	}
	ref := MethodRef{Method: first.Text}
	if p.Match(DoubleColon) {
		method, err := p.memberName("method name after '::'")
		if err != nil {
			return MethodRef{}, err
		}
		trait := Name{Span: first.Span, Parts: []string{first.Text}}
		ref = MethodRef{Trait: &trait, Method: method.Text}
	}
	ref.Span = p.SpanFrom(start)
	return ref, nil
}

func (p *parser) isVisibility() bool {
	switch p.Current().Kind {
	case KindPublic, KindPrivate, KindProtected:
		return true
	}
	return false
}

func (p *parser) parseProperty(attrs []*AttributeGroup, modifiers []string, memberType *Type, memberStart gast.Span) (ClassMember, error) {
	node := &Property{Attrs: attrs, Modifiers: modifiers, Type: memberType}
	for {
		varTok, err := p.Expect(KindVariable, "property variable")
		if err != nil {
			return nil, err
		}
		entry := PropertyVar{Var: variable(varTok)}
		if p.Match(Assign) {
			if entry.Default, err = p.parseExpr(); err != nil {
				return nil, err
			}
		}
		entry.Span = p.SpanFrom(varTok.Span)
		node.Vars = append(node.Vars, entry)
		if !p.Match(Comma) {
			break
		}
	}
	if err := p.endStatement(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(memberStart)
	return node, nil
}

func (p *parser) parseNamespaceStmt() (Stmt, error) {
	nsTok := p.Advance()
	node := &NamespaceStmt{}
	if p.Check(KindIdent) || p.Check(Backslash) {
		name, err := p.parseName()
		if err != nil {
			return nil, err
		}
		node.Name = name.String()
	}
	if p.Match(Semicolon) {
		node.Span = p.SpanFrom(nsTok.Span)
		return node, nil
	}
	if _, err := p.Expect(LBrace, "'{' or ';' after namespace"); err != nil {
		return nil, err
	}
	stmts, err := p.parseStmtList("}")
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(RBrace, "'}' to close namespace"); err != nil {
		return nil, err
	}
	node.Body = &BlockStmt{Stmts: stmts}
	node.Span = p.SpanFrom(nsTok.Span)
	return node, nil
}

func (p *parser) parseUseStmt() (Stmt, error) {
	useTok := p.Advance()
	kind := "type"
	switch {
	case p.Match(KindFunction):
		kind = "function"
	case p.Match(KindConst):
		kind = "const"
	}
	node := &UseStmt{Kind: kind}
	for {
		imports, err := p.parseUseImport()
		if err != nil {
			return nil, err
		}
		node.Imports = append(node.Imports, imports...)
		if !p.Match(Comma) {
			break
		}
	}
	if err := p.endStatement(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(useTok.Span)
	return node, nil
}

func (p *parser) parseUseImport() ([]UseImport, error) {
	start := p.Current().Span
	qualified := p.Match(Backslash)
	var parts []string
	for {
		name, err := p.Expect(KindIdent, "name in use statement")
		if err != nil {
			return nil, err
		}
		parts = append(parts, name.Text)
		if !p.Match(Backslash) {
			break
		}
		if p.Match(LBrace) {
			var imports []UseImport
			for !p.Check(RBrace) {
				entry, err := p.parseUseEntry()
				if err != nil {
					return nil, err
				}
				full := append([]string{}, parts...)
				full = append(full, entry.Parts...)
				imports = append(imports, UseImport{
					Span:  p.SpanFrom(start),
					Name:  Name{Span: entry.Span, Qualified: qualified, Parts: full},
					Alias: entry.Alias,
				})
				if !p.Match(Comma) {
					break
				}
			}
			if _, err := p.Expect(RBrace, "'}' to close use group"); err != nil {
				return nil, err
			}
			return imports, nil
		}
	}
	entry := UseImport{Span: p.SpanFrom(start), Name: Name{Span: p.SpanFrom(start), Qualified: qualified, Parts: parts}}
	if p.Match(KindAs) {
		alias, err := p.Expect(KindIdent, "alias after 'as'")
		if err != nil {
			return nil, err
		}
		entry.Alias = alias.Text
		entry.Span = p.SpanFrom(start)
	}
	return []UseImport{entry}, nil
}

type useEntry struct {
	gast.Span
	Parts []string
	Alias string
}

func (p *parser) parseUseEntry() (useEntry, error) {
	start := p.Current().Span
	switch p.Current().Kind {
	case KindFunction, KindConst:
		p.Advance()
	}
	var parts []string
	for {
		name, err := p.Expect(KindIdent, "name in use group")
		if err != nil {
			return useEntry{}, err
		}
		parts = append(parts, name.Text)
		if !p.Match(Backslash) {
			break
		}
	}
	entry := useEntry{Span: p.SpanFrom(start), Parts: parts}
	if p.Match(KindAs) {
		alias, err := p.Expect(KindIdent, "alias after 'as'")
		if err != nil {
			return useEntry{}, err
		}
		entry.Alias = alias.Text
		entry.Span = p.SpanFrom(start)
	}
	return entry, nil
}

func (p *parser) parseConstStmt() (Stmt, error) {
	constTok := p.Advance()
	node := &ConstStmt{}
	var err error
	if node.Consts, err = p.parseConstEntries(); err != nil {
		return nil, err
	}
	if err := p.endStatement(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(constTok.Span)
	return node, nil
}

func (p *parser) parseEchoStmt() (Stmt, error) {
	echoTok := p.Advance()
	args, err := p.parseExprList()
	if err != nil {
		return nil, err
	}
	if err := p.endStatement(); err != nil {
		return nil, err
	}
	return &EchoStmt{Span: p.SpanFrom(echoTok.Span), Args: args}, nil
}

func (p *parser) parseGlobalStmt() (Stmt, error) {
	globalTok := p.Advance()
	var vars []Expr
	for {
		v, err := p.parseVariableTarget()
		if err != nil {
			return nil, err
		}
		vars = append(vars, v)
		if !p.Match(Comma) {
			break
		}
	}
	if err := p.endStatement(); err != nil {
		return nil, err
	}
	return &GlobalStmt{Span: p.SpanFrom(globalTok.Span), Vars: vars}, nil
}

func (p *parser) parseStaticStmt() (Stmt, error) {
	staticTok := p.Advance()
	var vars []StaticVarEntry
	for {
		varTok, err := p.Expect(KindVariable, "static variable")
		if err != nil {
			return nil, err
		}
		entry := StaticVarEntry{Var: variable(varTok)}
		if p.Match(Assign) {
			if entry.Init, err = p.parseExpr(); err != nil {
				return nil, err
			}
		}
		entry.Span = p.SpanFrom(varTok.Span)
		vars = append(vars, entry)
		if !p.Match(Comma) {
			break
		}
	}
	if err := p.endStatement(); err != nil {
		return nil, err
	}
	return &StaticStmt{Span: p.SpanFrom(staticTok.Span), Vars: vars}, nil
}

func (p *parser) parseUnsetStmt() (Stmt, error) {
	unsetTok := p.Advance()
	if _, err := p.Expect(LParen, "'(' after 'unset'"); err != nil {
		return nil, err
	}
	vars, err := p.parseExprList()
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(RParen, "')' to close unset"); err != nil {
		return nil, err
	}
	if err := p.endStatement(); err != nil {
		return nil, err
	}
	return &UnsetStmt{Span: p.SpanFrom(unsetTok.Span), Vars: vars}, nil
}
