package ts

import (
	"github.com/stubbedev/gast/pkg/gast"
)

// parseType parses a full type: constructor types, function types,
// conditional types, and the union hierarchy below them.
func (p *parser) parseType() (TypeNode, error) {
	switch {
	case p.Check(KindNew):
		return p.parseFunctionType(false, true)
	case p.isWord("abstract") && p.CheckAhead(1, KindNew):
		p.Advance()
		return p.parseFunctionType(true, true)
	case p.Check(LParen) && p.fnTypeAhead():
		return p.parseFunctionType(false, false)
	}
	t, err := p.parseUnionType()
	if err != nil {
		return nil, err
	}
	if !p.Check(KindExtends) {
		return t, nil
	}
	p.Advance()
	node := &ConditionalType{Check: t}
	if node.Extends, err = p.parseUnionType(); err != nil {
		return nil, err
	}
	if _, err := p.Expect(Question, "'?' in conditional type"); err != nil {
		return nil, err
	}
	if node.True, err = p.parseType(); err != nil {
		return nil, err
	}
	if _, err := p.Expect(Colon, "':' in conditional type"); err != nil {
		return nil, err
	}
	if node.False, err = p.parseType(); err != nil {
		return nil, err
	}
	node.Span = gast.Merge(t.Pos(), node.False.Pos())
	return node, nil
}

// fnTypeAhead reports whether the "(" at the cursor opens a function
// type: its matching ")" is followed by "=>". Semicolons inside the
// brackets (object-type members) do not end the scan.
func (p *parser) fnTypeAhead() bool {
	depth := 0
	for i := 0; ; i++ {
		tok := p.Peek(i)
		switch tok.Kind {
		case LParen, LBracket, LBrace:
			depth++
		case RParen:
			depth--
			if depth <= 0 {
				return p.Peek(i+1).Kind == Arrow
			}
		case RBracket, RBrace:
			depth--
			if depth < 0 {
				return false
			}
		case KindEOF:
			return false
		}
	}
}

// parseFunctionType parses a function type `(params) => ret`, a
// constructor type `new (params) => ret`, or `abstract new ...`.
func (p *parser) parseFunctionType(abstract, ctor bool) (TypeNode, error) {
	start := p.Current().Span
	if abstract || ctor {
		p.Advance() // 'new' (the abstract word is already consumed)
	}
	node := &FunctionType{Abstract: abstract, New: ctor}
	var err error
	if node.TypeParams, err = p.parseTypeParams(); err != nil {
		return nil, err
	}
	if node.Params, err = p.parseParams(); err != nil {
		return nil, err
	}
	if _, err := p.Expect(Arrow, "'=>' in function type"); err != nil {
		return nil, err
	}
	if node.ReturnType, err = p.parseReturnType(); err != nil {
		return nil, err
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// parseUnionType parses unions with optional leading and separating
// '|' operators.
func (p *parser) parseUnionType() (TypeNode, error) {
	p.Match(Pipe)
	first, err := p.parseIntersectionType()
	if err != nil {
		return nil, err
	}
	if !p.Check(Pipe) {
		return first, nil
	}
	types := []TypeNode{first}
	for p.Match(Pipe) {
		next, err := p.parseIntersectionType()
		if err != nil {
			return nil, err
		}
		types = append(types, next)
	}
	return &UnionType{Span: gast.Merge(first.Pos(), types[len(types)-1].Pos()), Types: types}, nil
}

// parseIntersectionType parses '&' separated types.
func (p *parser) parseIntersectionType() (TypeNode, error) {
	p.Match(Ampersand)
	first, err := p.parseTypeOperator()
	if err != nil {
		return nil, err
	}
	if !p.Check(Ampersand) {
		return first, nil
	}
	types := []TypeNode{first}
	for p.Match(Ampersand) {
		next, err := p.parseTypeOperator()
		if err != nil {
			return nil, err
		}
		types = append(types, next)
	}
	return &IntersectionType{Span: gast.Merge(first.Pos(), types[len(types)-1].Pos()), Types: types}, nil
}

// parseTypeOperator parses the prefix type operators keyof, unique,
// and readonly, which apply to postfix types: `keyof T[]` is
// keyof (T[]).
func (p *parser) parseTypeOperator() (TypeNode, error) {
	word := ""
	if p.isWord("keyof") || p.isWord("unique") || p.isWord("readonly") {
		word = p.Current().Text
		if !p.typeAhead(1) {
			word = ""
		}
	}
	if word == "" {
		return p.parsePostfixType()
	}
	tok := p.Advance()
	operand, err := p.parseTypeOperator()
	if err != nil {
		return nil, err
	}
	return &TypeOperator{Span: gast.Merge(tok.Span, operand.Pos()), Op: word, Type: operand}, nil
}

// parsePostfixType parses a primary type with array and indexed-access
// suffixes.
func (p *parser) parsePostfixType() (TypeNode, error) {
	t, err := p.parsePrimaryType()
	if err != nil {
		return nil, err
	}
	for p.Check(LBracket) {
		p.Advance()
		if p.Check(RBracket) {
			close := p.Advance()
			t = &ArrayType{Span: gast.Merge(t.Pos(), close.Span), Elem: t}
			continue
		}
		index, err := p.parseType()
		if err != nil {
			return nil, err
		}
		close, err := p.Expect(RBracket, "']' after indexed access")
		if err != nil {
			return nil, err
		}
		t = &IndexedAccessType{Span: gast.Merge(t.Pos(), close.Span), Obj: t, Index: index}
	}
	return t, nil
}

// parsePrimaryType parses every non-composed type form.
func (p *parser) parsePrimaryType() (TypeNode, error) {
	tok := p.Current()
	switch tok.Kind {
	case KindVoid:
		p.Advance()
		return &TypeRef{Span: tok.Span, Parts: []string{"void"}}, nil
	case KindThis:
		p.Advance()
		return &TypeRef{Span: tok.Span, Parts: []string{"this"}}, nil
	case KindTrue, KindFalse, KindNull:
		p.Advance()
		return &LiteralType{Span: tok.Span, Text: tok.Text}, nil
	case KindNum, KindBigInt:
		p.Advance()
		return &LiteralType{Span: tok.Span, Text: tok.Text}, nil
	case KindString:
		p.Advance()
		return &LiteralType{Span: tok.Span, Text: tok.Text}, nil
	case Minus:
		if p.CheckAhead(1, KindNum) || p.CheckAhead(1, KindBigInt) {
			p.Advance()
			lit := p.Advance()
			return &LiteralType{
				Span: gast.Merge(tok.Span, lit.Span),
				Text: lit.Text, Neg: true,
			}, nil
		}
	case KindTemplateHead:
		return p.parseTemplateLiteralType()
	case KindTypeof:
		p.Advance()
		var operand TypeNode
		var err error
		if p.Check(KindImport) {
			operand, err = p.parseImportType()
		} else {
			operand, err = p.parseNamedTypeRef()
		}
		if err != nil {
			return nil, err
		}
		return &TypeOperator{Span: gast.Merge(tok.Span, operand.Pos()), Op: "typeof", Type: operand}, nil
	case KindImport:
		return p.parseImportType()
	case LBrace:
		return p.parseObjectOrMappedType()
	case LBracket:
		return p.parseTupleType()
	case LParen:
		p.Advance()
		inner, err := p.parseType()
		if err != nil {
			return nil, err
		}
		close, err := p.Expect(RParen, "')' in parenthesized type")
		if err != nil {
			return nil, err
		}
		return &ParenType{Span: gast.Merge(tok.Span, close.Span), Type: inner}, nil
	case KindIdent:
		if p.isWord("infer") {
			return p.parseInferType()
		}
		return p.parseNamedTypeRef()
	}
	return nil, p.Fail("expected type, found " + gast.Describe(tok))
}

// parseNamedTypeRef parses a qualified type name with optional type
// arguments after any segment, as in `A.B<C>.D`.
func (p *parser) parseNamedTypeRef() (TypeNode, error) {
	start := p.Current().Span
	name, err := p.Expect(KindIdent, "type name")
	if err != nil {
		return nil, err
	}
	node := &TypeRef{Parts: []string{name.Text}}
	node.Span = name.Span
	for {
		if p.Check(Lt) {
			args, err := p.parseTypeArgs()
			if err != nil {
				return nil, err
			}
			node.TypeArgs = args
			node.Span = p.SpanFrom(start)
		}
		if p.Match(Dot) {
			part, err := p.propertyName("type name after '.'")
			if err != nil {
				return nil, err
			}
			ident, ok := part.(*IdentExpr)
			if !ok {
				return nil, p.FailAt(part.Pos(), "expected type name after '.'")
			}
			node.Parts = append(node.Parts, ident.Name)
			node.Span = gast.Merge(start, ident.Span)
			continue
		}
		return node, nil
	}
}

// parseInferType parses `infer name` with an optional `extends`
// constraint.
func (p *parser) parseInferType() (TypeNode, error) {
	tok := p.Advance() // 'infer'
	name, err := p.Expect(KindIdent, "name after 'infer'")
	if err != nil {
		return nil, err
	}
	node := &InferType{Name: name.Text}
	node.Span = name.Span
	if p.Match(KindExtends) {
		if node.Constraint, err = p.parseUnionType(); err != nil {
			return nil, err
		}
		node.Span = gast.Merge(tok.Span, node.Constraint.Pos())
	}
	return node, nil
}

// parseImportType parses `import("./m").A<T>`.
func (p *parser) parseImportType() (TypeNode, error) {
	importTok := p.Advance()
	if _, err := p.Expect(LParen, "'(' in import type"); err != nil {
		return nil, err
	}
	source, err := p.Expect(KindString, "module specifier in import type")
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect(RParen, "')' in import type"); err != nil {
		return nil, err
	}
	node := &ImportType{Source: source.Text}
	node.Span = gast.Merge(importTok.Span, p.Peek(-1).Span)
	for p.Match(Dot) {
		part, err := p.propertyName("type name after '.'")
		if err != nil {
			return nil, err
		}
		ident, ok := part.(*IdentExpr)
		if !ok {
			return nil, p.FailAt(part.Pos(), "expected type name after '.'")
		}
		node.Parts = append(node.Parts, ident.Name)
	}
	if p.Check(Lt) {
		var err error
		if node.TypeArgs, err = p.parseTypeArgs(); err != nil {
			return nil, err
		}
	}
	if len(node.Parts) > 0 {
		node.Span = gast.Merge(importTok.Span, p.Peek(-1).Span)
	}
	return node, nil
}

// parseTupleType parses `[A, B?, ...C]` and the named-element forms
// `[a: A, b?: B]`.
func (p *parser) parseTupleType() (TypeNode, error) {
	open := p.Advance()
	node := &TupleType{}
	for !p.Check(RBracket) {
		if p.AtEnd() {
			return nil, p.Fail("expected ']' to close tuple type")
		}
		elem := &TupleElement{}
		elemStart := p.Current().Span
		if p.Match(Ellipsis) {
			elem.Rest = true
		}
		// A name introduces its type with ':' and cannot itself be a
		// type-start keyword.
		if p.Check(KindIdent) && p.typeAheadColon(0) {
			elem.Name = p.Advance().Text
			if p.Match(Question) {
				elem.Optional = true
			}
			if _, err := p.Expect(Colon, "':' after tuple element name"); err != nil {
				return nil, err
			}
		}
		typ, err := p.parseType()
		if err != nil {
			return nil, err
		}
		elem.Type = typ
		elem.Span = p.SpanFrom(elemStart)
		node.Elems = append(node.Elems, elem)
		if !p.Match(Comma) {
			break
		}
	}
	close, err := p.Expect(RBracket, "']' to close tuple type")
	if err != nil {
		return nil, err
	}
	node.Span = gast.Merge(open.Span, close.Span)
	return node, nil
}

// typeAheadColon reports whether the identifier at the cursor is a
// tuple-element name: followed by ':' or '?:'.
func (p *parser) typeAheadColon(ahead int) bool {
	if p.CheckAhead(ahead+1, Colon) {
		return true
	}
	return p.CheckAhead(ahead+1, Question) && p.CheckAhead(ahead+2, Colon)
}

// parseTemplateLiteralType parses a template literal type whose
// substitutions are types.
func (p *parser) parseTemplateLiteralType() (TypeNode, error) {
	first := p.Advance()
	node := &TemplateLiteralType{Quasis: []string{templateQuasiText(first)}}
	if first.Kind == KindNoSubTemplate {
		node.Span = first.Span
		return node, nil
	}
	for {
		typ, err := p.parseType()
		if err != nil {
			return nil, err
		}
		node.Types = append(node.Types, typ)
		cur := p.Current()
		if cur.Kind != KindTemplateMiddle && cur.Kind != KindTemplateTail {
			return nil, p.Fail("expected '}' to continue template literal type, found " + gast.Describe(cur))
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

// parseObjectOrMappedType parses an object type literal or a mapped
// type; a '[' followed by a name and 'in' after an optional readonly
// modifier opens the mapped form.
func (p *parser) parseObjectOrMappedType() (TypeNode, error) {
	open := p.Current()
	bracketAt := 1
	if p.isWord("readonly") {
		bracketAt = 2
	} else if (p.CheckAhead(1, Plus) || p.CheckAhead(1, Minus)) && p.wordAt(2, "readonly") {
		bracketAt = 3
	}
	if p.CheckAhead(bracketAt, LBracket) && p.CheckAhead(bracketAt+1, KindIdent) &&
		p.CheckAhead(bracketAt+2, KindIn) {
		return p.parseMappedType()
	}
	p.Advance()
	node := &ObjectType{}
	var err error
	if node.Members, err = p.parseTypeMembers(); err != nil {
		return nil, err
	}
	close := p.Peek(-1)
	node.Span = gast.Merge(open.Span, close.Span)
	return node, nil
}

// expectModifierWord consumes the mapped-type modifier word that
// follows a '+' or '-' sign.
func (p *parser) expectModifierWord(word string) {
	if p.isWord(word) {
		p.Advance()
	}
}

// parseMappedType parses `{ [K in T as U]?: V }` with its '+'/'-'
// variance forms. The '{' is current.
func (p *parser) parseMappedType() (TypeNode, error) {
	open := p.Advance()
	node := &MappedType{}
	switch {
	case p.Match(Plus):
		node.ReadonlyMod = "+readonly"
		p.expectModifierWord("readonly")
	case p.Match(Minus):
		node.ReadonlyMod = "-readonly"
		p.expectModifierWord("readonly")
	default:
		if p.isWord("readonly") {
			p.Advance()
			node.ReadonlyMod = "readonly"
		}
	}
	if _, err := p.Expect(LBracket, "'[' in mapped type"); err != nil {
		return nil, err
	}
	name, err := p.Expect(KindIdent, "type parameter name in mapped type")
	if err != nil {
		return nil, err
	}
	node.KeyName = name.Text
	if _, err := p.Expect(KindIn, "'in' in mapped type"); err != nil {
		return nil, err
	}
	if node.In, err = p.parseType(); err != nil {
		return nil, err
	}
	if p.isWord("as") {
		p.Advance()
		if node.As, err = p.parseType(); err != nil {
			return nil, err
		}
	}
	if _, err := p.Expect(RBracket, "']' in mapped type"); err != nil {
		return nil, err
	}
	switch {
	case p.Match(Plus):
		node.OptionalMod = "+?"
	case p.Match(Minus):
		node.OptionalMod = "-?"
	default:
		if p.Match(Question) {
			node.OptionalMod = "?"
		}
	}
	if node.OptionalMod != "" && !p.Check(Colon) && p.Match(Question) {
		node.OptionalMod += "?"
	}
	if _, err := p.Expect(Colon, "':' in mapped type"); err != nil {
		return nil, err
	}
	if node.Value, err = p.parseType(); err != nil {
		return nil, err
	}
	if err := p.consumeTypeMemberSeparator(); err != nil {
		return nil, err
	}
	close, err := p.Expect(RBrace, "'}' to close mapped type")
	if err != nil {
		return nil, err
	}
	node.Span = gast.Merge(open.Span, close.Span)
	return node, nil
}

// parseTypeMembers parses object-type members until '}'. The '{' has
// been consumed; members end at ';', ',', a line break, or the brace.
func (p *parser) parseTypeMembers() ([]TypeMember, error) {
	var members []TypeMember
	for !p.Check(RBrace) {
		if p.AtEnd() {
			return nil, p.Fail("expected '}' to close type body")
		}
		member, err := p.parseTypeMember()
		if err != nil {
			return nil, err
		}
		members = append(members, member)
		if err := p.consumeTypeMemberSeparator(); err != nil {
			return nil, err
		}
	}
	p.Advance()
	return members, nil
}

// consumeTypeMemberSeparator consumes the ';' or ',' between type
// members; a line break or the closing brace also separates.
func (p *parser) consumeTypeMemberSeparator() error {
	if p.Match(Semicolon) || p.Match(Comma) || p.Check(RBrace) || p.hadLineBreak() {
		return nil
	}
	return p.Fail("expected ';' or ',' between type members, found " + gast.Describe(p.Current()))
}

// parseTypeMember parses one member of an object type or interface
// body.
func (p *parser) parseTypeMember() (TypeMember, error) {
	start := p.Current().Span
	switch {
	case p.Check(LParen), p.Check(Lt):
		return p.parseCallSig(start)
	case p.Check(KindNew):
		return p.parseConstructSig(start, false)
	case p.isWord("abstract") && p.CheckAhead(1, KindNew):
		p.Advance()
		return p.parseConstructSig(start, true)
	}
	readonly := false
	if p.isWord("readonly") && p.typeMemberKeyAhead(1) {
		readonly = true
		p.Advance()
	}
	if (p.isWord("get") || p.isWord("set")) && p.typeMemberKeyAhead(1) {
		return p.parseAccessorSig(start)
	}
	// Index signature: [name: KeyType]: Type.
	if p.Check(LBracket) && p.CheckAhead(1, KindIdent) && p.CheckAhead(2, Colon) {
		return p.parseIndexSig(start, readonly)
	}
	key, _, err := p.parsePropertyKey()
	if err != nil {
		return nil, err
	}
	optional := p.Match(Question)
	if p.Check(LParen) || p.Check(Lt) {
		return p.parseMethodSig(start, key, optional, readonly)
	}
	node := &PropertySig{Key: key, Optional: optional, Readonly: readonly}
	if p.Match(Colon) {
		if node.Type, err = p.parseType(); err != nil {
			return nil, err
		}
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// typeMemberKeyAhead reports whether the token ahead continues a type
// member after a modifier word, keeping members named "readonly".
func (p *parser) typeMemberKeyAhead(ahead int) bool {
	kind := p.Peek(ahead).Kind
	if isClassKeyStart(kind) {
		return kind != LBracket || p.CheckAhead(ahead+1, KindIdent)
	}
	return kind == Asterisk
}

// parseCallSig parses a call signature.
func (p *parser) parseCallSig(start gast.Span) (TypeMember, error) {
	node := &CallSig{}
	var err error
	if node.TypeParams, err = p.parseTypeParams(); err != nil {
		return nil, err
	}
	if node.Params, err = p.parseParams(); err != nil {
		return nil, err
	}
	if p.Match(Colon) {
		if node.ReturnType, err = p.parseReturnType(); err != nil {
			return nil, err
		}
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// parseConstructSig parses a construct signature.
func (p *parser) parseConstructSig(start gast.Span, abstract bool) (TypeMember, error) {
	p.Advance() // 'new'
	node := &ConstructSig{Abstract: abstract}
	var err error
	if node.TypeParams, err = p.parseTypeParams(); err != nil {
		return nil, err
	}
	if node.Params, err = p.parseParams(); err != nil {
		return nil, err
	}
	if p.Match(Colon) {
		if node.ReturnType, err = p.parseReturnType(); err != nil {
			return nil, err
		}
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// parseAccessorSig parses a getter or setter signature.
func (p *parser) parseAccessorSig(start gast.Span) (TypeMember, error) {
	accessor := p.Advance().Text
	key, _, err := p.parsePropertyKey()
	if err != nil {
		return nil, err
	}
	params, err := p.parseParams()
	if err != nil {
		return nil, err
	}
	if err := p.checkParameterProperties(params, false); err != nil {
		return nil, err
	}
	if accessor == "get" {
		if len(params) != 0 {
			return nil, p.FailAt(p.SpanFrom(start), "getter signature cannot take parameters")
		}
		node := &GetSig{Key: key}
		if p.Match(Colon) {
			if node.Type, err = p.parseType(); err != nil {
				return nil, err
			}
		}
		node.Span = p.SpanFrom(start)
		return node, nil
	}
	if len(params) != 1 {
		return nil, p.FailAt(p.SpanFrom(start), "setter signature takes exactly one parameter")
	}
	node := &SetSig{Key: key, Param: params[0]}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// parseIndexSig parses an index signature.
func (p *parser) parseIndexSig(start gast.Span, readonly bool) (TypeMember, error) {
	p.Advance() // '['
	node := &IndexSig{Readonly: readonly}
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
	node.Span = p.SpanFrom(start)
	return node, nil
}

// parseMethodSig parses a method signature after its key.
func (p *parser) parseMethodSig(start gast.Span, key Expr, optional, readonly bool) (TypeMember, error) {
	node := &MethodSig{Key: key, Optional: optional, Readonly: readonly}
	var err error
	if node.TypeParams, err = p.parseTypeParams(); err != nil {
		return nil, err
	}
	if node.Params, err = p.parseParams(); err != nil {
		return nil, err
	}
	if p.Match(Colon) {
		if node.ReturnType, err = p.parseReturnType(); err != nil {
			return nil, err
		}
	}
	node.Span = p.SpanFrom(start)
	return node, nil
}

// parseTypeParams parses a type-parameter list
// `<const T in out extends X = D, ...>` when a '<' follows; every call
// site is a position where '<' cannot be a comparison operator, so no
// lookahead is needed. typeParamsTrailingComma reports a trailing
// comma, which TSX's arrow-versus-JSX disambiguation needs.
func (p *parser) parseTypeParams() ([]*TypeParam, error) {
	p.typeParamsTrailingComma = false
	if !p.Check(Lt) {
		return nil, nil
	}
	p.Advance() // '<'
	var params []*TypeParam
	for {
		start := p.Current().Span
		param := &TypeParam{}
		if p.Check(KindConst) {
			param.Const = true
			p.Advance()
		}
		if p.Check(KindIn) {
			param.In = true
			p.Advance()
		}
		if p.isWord("out") {
			param.Out = true
			p.Advance()
		}
		name, err := p.Expect(KindIdent, "type parameter name")
		if err != nil {
			return nil, err
		}
		param.Name = name.Text
		if p.Match(KindExtends) {
			if param.Constraint, err = p.parseUnionType(); err != nil {
				return nil, err
			}
		}
		if p.Match(Assign) {
			if param.Default, err = p.parseType(); err != nil {
				return nil, err
			}
		}
		param.Span = p.SpanFrom(start)
		params = append(params, param)
		if !p.Match(Comma) {
			break
		}
		if gtCloses(p.Current().Kind) > 0 {
			p.typeParamsTrailingComma = true
			break // trailing comma
		}
	}
	if err := p.expectGt("type parameter list"); err != nil {
		return nil, err
	}
	return params, nil
}

// parseTypeArgs parses a type-argument list `<T, U>`; the '<' is
// current.
func (p *parser) parseTypeArgs() ([]TypeNode, error) {
	p.Advance() // '<'
	var args []TypeNode
	for {
		arg, err := p.parseType()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if !p.Match(Comma) {
			break
		}
		if gtCloses(p.Current().Kind) > 0 {
			break // trailing comma
		}
	}
	if err := p.expectGt("type argument list"); err != nil {
		return nil, err
	}
	if len(args) == 0 {
		args = []TypeNode{}
	}
	return args, nil
}

// parseReturnType parses a return type: a type or a type predicate
// (`x is T`, `asserts x is T`, `asserts this is T`).
func (p *parser) parseReturnType() (TypeNode, error) {
	if p.isWord("asserts") {
		switch p.Peek(1).Kind {
		case KindIdent, KindThis:
			if !p.lineBreakBetween(p.Current(), p.Peek(1)) {
				tok := p.Advance()
				name := p.Advance()
				node := &TypePredicate{Name: name.Text, Asserts: true}
				if p.isWord("is") {
					p.Advance()
					var err error
					if node.Type, err = p.parseType(); err != nil {
						return nil, err
					}
				}
				node.Span = gast.Merge(tok.Span, p.Peek(-1).Span)
				return node, nil
			}
		}
	}
	if (p.Check(KindIdent) || p.Check(KindThis)) && p.wordAt(1, "is") {
		name := p.Advance()
		p.Advance() // 'is'
		node := &TypePredicate{Name: name.Text}
		var err error
		if node.Type, err = p.parseType(); err != nil {
			return nil, err
		}
		node.Span = gast.Merge(name.Span, node.Type.Pos())
		return node, nil
	}
	return p.parseType()
}
