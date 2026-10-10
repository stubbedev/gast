package python

import (
	"github.com/stubbedev/gast/pkg/gast"
)

// Statement walk. Every method stops once an error has been recorded,
// so a single walk reports a single violation.

// stmts walks a statement list in scope s.
func (c *checker) stmts(list []Stmt, s *scope) {
	for _, stmt := range list {
		if c.err != nil {
			return
		}
		c.stmt(stmt, s)
	}
}

func (c *checker) stmt(node Stmt, s *scope) {
	switch st := node.(type) {
	case *ExpressionStmt:
		c.value(st.Expr, s)
	case *SeqStmt:
		c.stmts(st.Stmts, s)
	case *AssignStmt:
		for _, target := range st.Targets {
			c.starTarget(target, true)
			c.bindTarget(target, s)
		}
		c.value(st.Value, s)
	case *AugAssignStmt:
		c.bindTarget(st.Target, s)
		c.value(st.Value, s)
	case *AnnAssignStmt:
		c.value(st.Annotation, s)
		if name, ok := st.Target.(*NameExpr); ok {
			c.checkAnnotatedName(name, s)
			if st.Value != nil {
				s.bound[name.ID] = true
			}
		}
		c.value(st.Value, s)
	case *PassStmt:
	case *BreakStmt:
		c.checkLoopStmt(st.Span, s, "'break' outside loop")
	case *ContinueStmt:
		c.checkLoopStmt(st.Span, s, "'continue' not properly in loop")
	case *ReturnStmt:
		c.requireFunction(st.Span, s, "'return' outside function")
		c.value(st.Value, s)
	case *RaiseStmt:
		c.value(st.Exc, s)
		c.value(st.From, s)
	case *DelStmt:
		for _, target := range st.Targets {
			c.delTarget(target, s)
		}
	case *AssertStmt:
		c.value(st.Test, s)
		c.value(st.Msg, s)
	case *GlobalStmt:
		c.declareNames(st.Span, st.Names, s, true)
	case *NonlocalStmt:
		c.declareNames(st.Span, st.Names, s, false)
	case *ImportStmt:
		for _, alias := range st.Names {
			s.bound[importName(alias.Name, alias.AsName)] = true
		}
	case *ImportFromStmt:
		c.checkImportFrom(st, s)
		for _, alias := range st.Names {
			s.bound[importName(alias.Name, alias.AsName)] = true
		}
	case *IfStmt:
		c.value(st.Cond, s)
		c.stmts(st.Body, s)
		c.stmts(st.Orelse, s)
	case *WhileStmt:
		c.value(st.Cond, s)
		c.loopBody(st.Body, s)
		c.stmts(st.Orelse, s)
	case *ForStmt:
		c.checkAsyncStmt(st.Span, st.Async, s, "'async for' outside async function")
		c.starTarget(st.Target, true)
		c.bindTarget(st.Target, s)
		c.value(st.Iter, s)
		c.loopBody(st.Body, s)
		c.stmts(st.Orelse, s)
	case *TryStmt:
		c.stmts(st.Body, s)
		for _, handler := range st.Handlers {
			c.value(handler.Type, s)
			if handler.Name != "" {
				s.bound[handler.Name] = true
			}
			c.stmts(handler.Body, s)
		}
		c.stmts(st.Orelse, s)
		c.stmts(st.Finalbody, s)
	case *WithStmt:
		c.checkAsyncStmt(st.Span, st.Async, s, "'async with' outside async function")
		for _, item := range st.Items {
			c.value(item.Context, s)
			c.bindTarget(item.OptionalVars, s)
		}
		c.stmts(st.Body, s)
	case *FunctionDef:
		c.checkFunctionDef(st, s)
	case *ClassDef:
		for _, decorator := range st.Decorators {
			c.value(decorator.Expr, s)
		}
		c.typeParams(st.TypeParams, s)
		for _, base := range st.Bases {
			c.value(base, s)
		}
		for _, keyword := range st.Keywords {
			c.value(keyword.Value, s)
		}
		s.bound[st.Name] = true
		c.stmts(st.Body, newScope(s, scopeClass, false))
	case *TypeAliasStmt:
		c.typeParams(st.TypeParams, s)
		if name, ok := st.Name.(*NameExpr); ok {
			s.bound[name.ID] = true
		}
		c.value(st.Value, s)
	case *MatchStmt:
		c.value(st.Subject, s)
		for _, matchCase := range st.Cases {
			c.pattern(matchCase.Pattern, s)
			c.value(matchCase.Guard, s)
			c.stmts(matchCase.Body, s)
		}
	}
}

// loopBody walks a loop body with an incremented loop depth; the
// else clause keeps the outer depth because break and continue may not
// appear there.
func (c *checker) loopBody(body []Stmt, s *scope) {
	inner := *s
	inner.loopDepth++
	c.stmts(body, &inner)
}

// checkLoopStmt rejects break or continue outside a loop.
func (c *checker) checkLoopStmt(span gast.Span, s *scope, msg string) {
	if s.loopDepth == 0 {
		c.failMsg(span, msg)
	}
}

// requireFunction reports span unless s is inside a function; class
// and module scopes do not count.
func (c *checker) requireFunction(span gast.Span, s *scope, msg string) {
	switch c.enclosingFunc(s).kind {
	case scopeFunction:
	default:
		c.failMsg(span, msg)
	}
}

// checkAsyncStmt rejects async for/with statements outside an async
// function.
func (c *checker) checkAsyncStmt(span gast.Span, async bool, s *scope, msg string) {
	if async && !inAsyncFunc(s) {
		c.failMsg(span, msg)
	}
}

// checkImportFrom validates star imports and __future__ placement.
func (c *checker) checkImportFrom(st *ImportFromStmt, s *scope) {
	if st.Star && s.kind != scopeModule {
		c.fail(st.Span, "import * only allowed at module level")
	}
	if st.Dots == 0 && st.Module == modFuture && !c.futures[st] {
		c.fail(st.Span, "from __future__ imports must occur at the beginning of the file")
	}
}

// checkFunctionDef validates a def: its parameter list, then the
// pieces evaluated in the enclosing scope (decorators, type
// parameters, defaults, and annotations), then the body in a new
// function scope.
func (c *checker) checkFunctionDef(st *FunctionDef, s *scope) {
	c.checkParams(st.Params)
	for _, decorator := range st.Decorators {
		c.value(decorator.Expr, s)
	}
	c.typeParams(st.TypeParams, s)
	c.paramExtras(st.Params, s)
	s.bound[st.Name] = true
	fn := newScope(s, scopeFunction, st.Async)
	c.addParams(st.Params, fn)
	c.stmts(st.Body, fn)
}

// typeParams walks the bounds and defaults of PEP 695 type parameters,
// which are evaluated in the enclosing scope.
func (c *checker) typeParams(params []*TypeParam, s *scope) {
	for _, param := range params {
		c.value(param.Bound, s)
		c.value(param.Default, s)
	}
}

// paramExtras walks parameter defaults and annotations, evaluated
// where the function is defined rather than inside it: await in a
// default follows the enclosing scope's rules.
func (c *checker) paramExtras(params *Parameters, s *scope) {
	for _, param := range allParams(params) {
		c.value(param.Annotation, s)
		c.value(param.Default, s)
	}
}

// allParams returns every parameter of a list in declaration order.
func allParams(params *Parameters) []*Param {
	all := make([]*Param, 0, len(params.PosOnly)+len(params.PosOrKeyword)+len(params.KwOnly)+2)
	all = append(all, params.PosOnly...)
	all = append(all, params.PosOrKeyword...)
	if params.Vararg != nil {
		all = append(all, params.Vararg)
	}
	all = append(all, params.KwOnly...)
	if params.Kwarg != nil {
		all = append(all, params.Kwarg)
	}
	return all
}

// checkParams rejects duplicate parameter names across every section
// of a def or lambda parameter list, including *args, **kwargs,
// positional-only, and keyword-only sections.
func (c *checker) checkParams(params *Parameters) {
	seen := make(map[string]bool, len(allParams(params)))
	for _, param := range allParams(params) {
		if seen[param.Name] {
			c.fail(param.Span, "duplicate argument %q in function definition", param.Name)
			return
		}
		seen[param.Name] = true
	}
}

// addParams binds a parameter list into the function scope it
// introduces.
func (c *checker) addParams(params *Parameters, fn *scope) {
	for _, param := range allParams(params) {
		fn.params[param.Name] = true
		fn.bound[param.Name] = true
	}
}

// importName returns the name an import alias binds: the alias when
// given, else the first component of a dotted module path.
func importName(name, asName string) string {
	if asName != "" {
		return asName
	}
	for i := range len(name) {
		if name[i] == '.' {
			return name[:i]
		}
	}
	return name
}

// declareNames validates a global or nonlocal declaration and records
// it in the scope.
func (c *checker) declareNames(span gast.Span, names []string, s *scope, global bool) {
	for _, name := range names {
		if global {
			c.declareGlobal(span, name, s)
			continue
		}
		c.declareNonlocal(span, name, s)
		if c.err != nil {
			return
		}
	}
}

func (c *checker) declareGlobal(span gast.Span, name string, s *scope) {
	switch {
	case s.kind == scopeModule:
	case s.params[name]:
		c.fail(span, "name %q is parameter and global", name)
	case s.annotated[name]:
		c.fail(span, "annotated name %q can't be global", name)
	case s.bound[name]:
		c.fail(span, "name %q is assigned to before global declaration", name)
	case s.nonlocals[name]:
		c.fail(span, "name %q is nonlocal and global", name)
	default:
		s.globals[name] = true
	}
}

func (c *checker) declareNonlocal(span gast.Span, name string, s *scope) {
	switch {
	case s.kind == scopeModule:
		c.fail(span, "nonlocal declaration not allowed at module level")
	case s.params[name]:
		c.fail(span, "name %q is parameter and nonlocal", name)
	case s.annotated[name]:
		c.fail(span, "annotated name %q can't be nonlocal", name)
	case s.bound[name]:
		c.fail(span, "name %q is assigned to before nonlocal declaration", name)
	case s.globals[name]:
		c.fail(span, "name %q is nonlocal and global", name)
	default:
		s.nonlocals[name] = true
		c.nonlocals = append(c.nonlocals, pendingNonlocal{scope: s, name: name, span: span})
	}
}

// resolveNonlocals verifies each deferred nonlocal declaration against
// the fully collected bindings of the enclosing function scopes;
// global declarations hide outer bindings and class scopes never bind.
func (c *checker) resolveNonlocals() {
	for _, decl := range c.nonlocals {
		if c.err != nil {
			return
		}
		found := false
		for cur := decl.scope.parent; cur != nil; cur = cur.parent {
			if cur.kind != scopeFunction {
				continue
			}
			if cur.globals[decl.name] {
				break
			}
			if cur.bound[decl.name] {
				found = true
				break
			}
		}
		if !found {
			c.fail(decl.span, "no binding for nonlocal %q found", decl.name)
		}
	}
}

// checkAnnotatedName validates annotation of a plain name in a scope:
// __debug__ and names already declared global or nonlocal are illegal.
func (c *checker) checkAnnotatedName(name *NameExpr, s *scope) {
	switch {
	case name.ID == nameDebug:
		c.fail(name.Span, "cannot assign to "+nameDebug)
	case s.globals[name.ID]:
		c.fail(name.Span, "annotated name %q can't be global", name.ID)
	case s.nonlocals[name.ID]:
		c.fail(name.Span, "annotated name %q can't be nonlocal", name.ID)
	default:
		s.annotated[name.ID] = true
	}
}

// pattern walks a match pattern, binding its capture names.
func (c *checker) pattern(node Pattern, s *scope) {
	switch pat := node.(type) {
	case *MatchValue:
		c.value(pat.Value, s)
	case *MatchSingleton:
		c.value(pat.Value, s)
	case *MatchSequence:
		for _, sub := range pat.Patterns {
			c.pattern(sub, s)
		}
	case *MatchStar:
		if pat.Name != "" && pat.Name != wildcard {
			s.bound[pat.Name] = true
		}
	case *MatchMapping:
		for _, key := range pat.Keys {
			c.value(key, s)
		}
		for _, sub := range pat.Patterns {
			c.pattern(sub, s)
		}
		if pat.Rest != "" && pat.Rest != wildcard {
			s.bound[pat.Rest] = true
		}
	case *MatchClass:
		c.value(pat.Cls, s)
		for _, sub := range pat.Patterns {
			c.pattern(sub, s)
		}
		for _, sub := range pat.KwdPatterns {
			c.pattern(sub, s)
		}
	case *MatchAs:
		c.pattern(pat.Pattern, s)
		if pat.Name != "" && pat.Name != wildcard {
			s.bound[pat.Name] = true
		}
	case *MatchOr:
		for _, sub := range pat.Patterns {
			c.pattern(sub, s)
		}
	}
}
