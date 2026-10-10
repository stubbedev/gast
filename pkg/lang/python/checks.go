package python

import (
	"fmt"

	"github.com/stubbedev/gast/pkg/gast"
)

// Compile-time checks that CPython's symbol-table pass enforces after
// parsing: scope-sensitive statement placement (return, yield, await,
// break/continue), duplicate parameters, __debug__ targets, star
// placement, global/nonlocal misuse, __future__ placement, and star
// imports outside module level. The checker is a single recursive AST
// walk with a scope stack; O(n) in the module size.

// Reserved names that cannot be rebound or imported via placement
// rules checked here.
const (
	nameDebug = "__debug__"
	modFuture = "__future__"
)

// scopeKind classifies a lexical scope.
type scopeKind int

const (
	scopeModule scopeKind = iota
	scopeFunction
	scopeClass
	scopeComprehension
)

// scope tracks one lexical scope of the module: its kind, whether a
// function scope is async, the loop depth for break/continue, and the
// names bound so far for global/nonlocal validation.
type scope struct {
	parent    *scope
	kind      scopeKind
	async     bool
	loopDepth int

	bound     map[string]bool // names bound in this scope, in order
	params    map[string]bool // parameter names (function scopes)
	globals   map[string]bool // names declared global
	nonlocals map[string]bool // names declared nonlocal
	annotated map[string]bool // names given a bare annotation
}

// newScope allocates a child scope of parent; the module scope has a
// nil parent.
func newScope(parent *scope, kind scopeKind, async bool) *scope {
	return &scope{
		parent:    parent,
		kind:      kind,
		async:     async,
		bound:     map[string]bool{},
		params:    map[string]bool{},
		globals:   map[string]bool{},
		nonlocals: map[string]bool{},
		annotated: map[string]bool{},
	}
}

// pendingNonlocal defers a nonlocal declaration until the whole module
// has been walked, because the binding it refers to may be textually
// later in an enclosing function.
type pendingNonlocal struct {
	scope *scope
	name  string
	span  gast.Span
}

// checker walks a parsed module and reports the first violation as a
// *gast.ParseError.
type checker struct {
	nonlocals []pendingNonlocal
	futures   map[*ImportFromStmt]bool
	err       error
}

// checkFile validates file with CPython's post-parse symbol-table
// checks; a nil result means the module compiles.
func checkFile(file *File) error {
	c := &checker{futures: futureImports(file)}
	module := newScope(nil, scopeModule, false)
	c.stmts(file.Stmts, module)
	if c.err != nil {
		return c.err
	}
	c.resolveNonlocals()
	return c.err
}

// futureImports collects the `from __future__ import` statements that
// sit in the module's legal prefix: an optional docstring, then other
// future imports, before any other statement. Future imports anywhere
// else (later in the file, nested in a block, or inside a function or
// class) are rejected by the walk.
func futureImports(file *File) map[*ImportFromStmt]bool {
	allowed := map[*ImportFromStmt]bool{}
	sawOther := false
	for i, stmt := range file.Stmts {
		if imp, ok := stmt.(*ImportFromStmt); ok && imp.Dots == 0 && imp.Module == modFuture {
			if !sawOther {
				allowed[imp] = true
			}
			continue
		}
		if i == 0 && isDocstring(stmt) {
			continue
		}
		sawOther = true
	}
	return allowed
}

// isDocstring reports whether stmt is the module docstring: a plain
// string literal expression.
func isDocstring(stmt Stmt) bool {
	expr, ok := stmt.(*ExpressionStmt)
	if !ok {
		return false
	}
	_, ok = expr.Expr.(*StringLit)
	return ok
}

// fail records the first error encountered.
func (c *checker) fail(span gast.Span, format string, args ...any) {
	if c.err == nil {
		c.err = &gast.ParseError{Message: fmt.Sprintf(format, args...), Pos: span}
	}
}

// failMsg records a first error whose message needs no formatting.
func (c *checker) failMsg(span gast.Span, msg string) {
	c.fail(span, "%s", msg)
}

// inAsyncFunc reports whether s lies, through any comprehension
// scopes, inside an async function scope. Lambdas and class scopes
// break the chain: await and async comprehensions are illegal there.
func inAsyncFunc(s *scope) bool {
	for cur := s; cur != nil; cur = cur.parent {
		if cur.kind == scopeComprehension {
			continue
		}
		return cur.kind == scopeFunction && cur.async
	}
	return false
}

// enclosingFunc returns the innermost non-comprehension scope of s.
func (c *checker) enclosingFunc(s *scope) *scope {
	for cur := s; cur != nil; cur = cur.parent {
		if cur.kind == scopeComprehension {
			continue
		}
		return cur
	}
	return nil
}
