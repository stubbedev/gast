package python

import (
	"github.com/stubbedev/gast/pkg/gast"
)

// Expression walk and target validation.

// value walks an expression in value position, where a starred
// expression is never allowed (displays and subscript indices admit
// them and use starElt instead).
func (c *checker) value(e Expr, s *scope) {
	if c.err == nil {
		if _, ok := e.(*StarredExpr); ok {
			c.fail(e.Pos(), "cannot use starred expression here")
			return
		}
		c.expr(e, s)
	}
}

// starElt walks one element of a display, tuple, or subscript index,
// where `*x` unpacking is legal.
func (c *checker) starElt(e Expr, s *scope) {
	if star, ok := e.(*StarredExpr); ok {
		c.expr(star.Value, s)
		return
	}
	c.expr(e, s)
}

func (c *checker) expr(e Expr, s *scope) {
	switch t := e.(type) {
	case nil, *NameExpr, *IntLit, *FloatLit, *ComplexLit, *StringLit,
		*BytesLit, *BoolLit, *NoneLit, *EllipsisLit:
	case *JoinedStr:
		for _, part := range t.Parts {
			c.value(part, s)
		}
	case *FormattedValue:
		c.value(t.Value, s)
		c.value(t.FormatSpec, s)
	case *TupleExpr:
		for _, elt := range t.Elts {
			c.starElt(elt, s)
		}
	case *ListExpr, *SetExpr:
		for _, elt := range displayElts(t) {
			c.starElt(elt, s)
		}
	case *DictExpr:
		for _, item := range t.Items {
			c.value(item.Key, s)
			c.value(item.Value, s)
		}
	case *StarredExpr:
		c.expr(t.Value, s)
	case *AttributeExpr:
		c.value(t.Value, s)
	case *SubscriptExpr:
		c.value(t.Value, s)
		c.starElt(t.Index, s)
	case *SliceExpr:
		c.value(t.Lower, s)
		c.value(t.Upper, s)
		c.value(t.Step, s)
	case *CallExpr:
		c.value(t.Func, s)
		for _, arg := range t.Args {
			c.value(arg.Expr, s)
		}
	case *LambdaExpr:
		c.checkParams(t.Params)
		c.paramExtras(t.Params, s)
		c.expr(t.Body, newScope(s, scopeFunction, false))
	case *IfExpExpr:
		c.value(t.Test, s)
		c.value(t.Body, s)
		c.value(t.Orelse, s)
	case *BinOpExpr:
		c.value(t.Left, s)
		c.value(t.Right, s)
	case *BoolOpExpr:
		for _, v := range t.Values {
			c.value(v, s)
		}
	case *UnaryOpExpr:
		c.value(t.Operand, s)
	case *CompareExpr:
		c.value(t.Left, s)
		for _, comparator := range t.Comparators {
			c.value(comparator, s)
		}
	case *AwaitExpr:
		c.checkAwait(t.Span, s)
		c.value(t.Value, s)
	case *YieldExpr:
		c.checkYield(t.Span, s, "'yield' outside function")
		c.value(t.Value, s)
	case *YieldFromExpr:
		c.checkYield(t.Span, s, "'yield from' outside function")
		c.value(t.Value, s)
	case *NamedExpr:
		c.bindTarget(t.Target, s)
		c.value(t.Value, s)
	case *ComprehensionExpr:
		c.checkComprehension(t, s)
	}
}

// displayElts returns the elements of a list or set display.
func displayElts(e Expr) []Expr {
	switch d := e.(type) {
	case *ListExpr:
		return d.Elts
	case *SetExpr:
		return d.Elts
	}
	return nil
}

// checkAwait enforces await placement: inside an async function,
// looking through comprehension scopes but never through a lambda, a
// sync def, a class, or the module.
func (c *checker) checkAwait(span gast.Span, s *scope) {
	switch fn := c.enclosingFunc(s); fn.kind {
	case scopeFunction:
		if !fn.async {
			c.fail(span, "'await' outside async function")
		}
	default:
		c.fail(span, "'await' outside function")
	}
}

// checkYield enforces yield and yield-from placement: any function
// scope (including a lambda) admits them, but a comprehension scope
// rejects them and the module and class scopes are outside.
func (c *checker) checkYield(span gast.Span, s *scope, outside string) {
	for cur := s; cur != nil; cur = cur.parent {
		switch cur.kind {
		case scopeComprehension:
			c.fail(span, "'yield' inside comprehension")
			return
		case scopeFunction:
			return
		default:
			c.failMsg(span, outside)
			return
		}
	}
}

// checkComprehension validates a comprehension. The first iterable is
// evaluated in the enclosing scope; the element, key, filters, and
// later iterables belong to the comprehension's own scope. An async
// comprehension (an `async for` clause or an await) must appear inside
// an async function.
func (c *checker) checkComprehension(t *ComprehensionExpr, s *scope) {
	first := t.Generators[0]
	c.value(first.Iter, s)
	comp := newScope(s, scopeComprehension, false)
	if first.Async && !inAsyncFunc(comp) {
		c.fail(first.Span, "asynchronous comprehension outside of an asynchronous function")
		return
	}
	c.starTarget(first.Target, true)
	c.bindTarget(first.Target, comp)
	for _, cond := range first.Ifs {
		c.value(cond, comp)
	}
	for _, clause := range t.Generators[1:] {
		c.comprehensionClause(clause, comp)
		if c.err != nil {
			return
		}
	}
	c.value(t.Elt, comp)
	c.value(t.Key, comp)
}

// comprehensionClause validates one later `for target in iter` clause,
// with its filters, inside the comprehension scope comp.
func (c *checker) comprehensionClause(clause *ComprehensionClause, comp *scope) {
	if clause.Async && !inAsyncFunc(comp) {
		c.fail(clause.Span, "asynchronous comprehension outside of an asynchronous function")
		return
	}
	c.starTarget(clause.Target, true)
	c.bindTarget(clause.Target, comp)
	c.value(clause.Iter, comp)
	for _, cond := range clause.Ifs {
		c.value(cond, comp)
	}
}

// starTarget validates star placement in an assignment, for, or
// comprehension target: at most one starred element per tuple or list,
// a starred element must not be parenthesized, and a bare star is only
// legal inside a tuple or list.
func (c *checker) starTarget(target Expr, top bool) {
	switch t := target.(type) {
	case *StarredExpr:
		if t.Parenthesized {
			c.fail(t.Span, "cannot use starred expression here")
		} else if top {
			c.fail(t.Span, "starred assignment target must be in a list or tuple")
		}
	case *TupleExpr:
		c.starElements(t.Elts)
	case *ListExpr:
		c.starElements(t.Elts)
	}
}

// starElements validates the element list of a tuple or list target:
// at most one starred element per list.
func (c *checker) starElements(elts []Expr) {
	starred := false
	for _, elt := range elts {
		if _, ok := elt.(*StarredExpr); ok {
			if starred {
				c.fail(elt.Pos(), "multiple starred expressions in assignment")
				return
			}
			starred = true
		}
	}
	for _, elt := range elts {
		c.starTarget(elt, false)
		if c.err != nil {
			return
		}
	}
}

// bindTarget records every name a target binds in scope s and rejects
// assignment to __debug__.
func (c *checker) bindTarget(target Expr, s *scope) {
	if target == nil {
		return
	}
	switch t := target.(type) {
	case *NameExpr:
		if t.ID == nameDebug {
			c.fail(t.Span, "cannot assign to "+nameDebug)
			return
		}
		s.bound[t.ID] = true
	case *StarredExpr:
		c.bindTarget(t.Value, s)
	case *TupleExpr:
		for _, elt := range t.Elts {
			c.bindTarget(elt, s)
		}
	case *ListExpr:
		for _, elt := range t.Elts {
			c.bindTarget(elt, s)
		}
	}
}

// delTarget records the names a del statement unbinds, which still
// count as bindings for nonlocal lookup, and rejects del of __debug__.
func (c *checker) delTarget(target Expr, s *scope) {
	if name, ok := target.(*NameExpr); ok {
		if name.ID == nameDebug {
			c.fail(name.Span, "cannot delete "+nameDebug)
			return
		}
		s.bound[name.ID] = true
		return
	}
	c.bindTarget(target, s)
}
