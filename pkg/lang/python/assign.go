package python

import (
	"slices"
)

// parseAssignLine or an assignment:
// plain, augmented, and annotated forms.
// parseAssignLine parses an expression statement or an assignment:
// plain, augmented, and annotated forms.
func (p *parser) parseAssignLine() (Stmt, error) {
	start := p.Current().Span
	first, err := p.parseExprList()
	if err != nil {
		return nil, err
	}
	if p.Check(Assign) {
		var targets []Expr
		value := first
		for p.Match(Assign) {
			targets = append(targets, value)
			if value, err = p.parseExprList(); err != nil {
				return nil, err
			}
		}
		for _, target := range targets {
			if err := p.checkAssignTarget(target, false); err != nil {
				return nil, err
			}
		}
		node := &AssignStmt{Targets: targets, Value: value}
		node.Span = p.SpanFrom(start)
		return node, nil
	}
	if slices.Contains(augmentedOps, p.Current().Kind) {
		op := p.Advance()
		if err := p.checkAssignTarget(first, true); err != nil {
			return nil, err
		}
		value, err := p.parseExprList()
		if err != nil {
			return nil, err
		}
		node := &AugAssignStmt{Target: first, Op: op.Kind, Value: value}
		node.Span = p.SpanFrom(start)
		return node, nil
	}
	if p.Match(Colon) {
		if err := p.checkAnnTarget(first); err != nil {
			return nil, err
		}
		annotation, err := p.parseTest()
		if err != nil {
			return nil, err
		}
		var value Expr
		if p.Match(Assign) {
			if value, err = p.parseExprList(); err != nil {
				return nil, err
			}
		}
		node := &AnnAssignStmt{Target: first, Annotation: annotation, Value: value, Simple: isPlainName(first)}
		node.Span = p.SpanFrom(start)
		return node, nil
	}
	if _, isNamed := first.(*NamedExpr); isNamed && p.source[start.Start] != '(' {
		return nil, p.FailAt(first.Pos(), "assignment expression is not allowed in this context")
	}
	node := &ExpressionStmt{Expr: first}
	node.Span = p.SpanFrom(start)
	return node, nil
}

func isPlainName(expr Expr) bool {
	_, ok := expr.(*NameExpr)
	return ok
}

// checkAssignTarget validates an assignment target; aug restricts it
// to the single forms an augmented assignment admits.
func (p *parser) checkAssignTarget(expr Expr, aug bool) error {
	switch t := expr.(type) {
	case *NameExpr, *AttributeExpr, *SubscriptExpr:
		return nil
	case *StarredExpr:
		if aug {
			return p.FailAt(t.Span, "starred assignment target is not allowed here")
		}
		return p.checkAssignTarget(t.Value, false)
	case *TupleExpr:
		if aug {
			return p.FailAt(t.Span, "augmented assignment target must be a single name, attribute, or subscript")
		}
		return p.checkAllTargets(t.Elts)
	case *ListExpr:
		if aug {
			return p.FailAt(t.Span, "augmented assignment target must be a single name, attribute, or subscript")
		}
		return p.checkAllTargets(t.Elts)
	case *CallExpr:
		return p.FailAt(t.Span, "cannot assign to function call")
	case *IntLit, *FloatLit, *ComplexLit, *StringLit, *BytesLit, *BoolLit, *NoneLit, *EllipsisLit, *JoinedStr:
		return p.FailAt(t.Pos(), "cannot assign to literal")
	}
	return p.FailAt(expr.Pos(), "invalid assignment target")
}

func (p *parser) checkAllTargets(targets []Expr) error {
	for _, target := range targets {
		if err := p.checkAssignTarget(target, false); err != nil {
			return err
		}
	}
	return nil
}

// checkAnnTarget validates an annotated assignment target; only a
// single name, attribute, or subscript may be annotated.
func (p *parser) checkAnnTarget(expr Expr) error {
	switch expr.(type) {
	case *NameExpr, *AttributeExpr, *SubscriptExpr:
		return nil
	}
	return p.FailAt(expr.Pos(), "only a single target can be annotated")
}

// checkDelTarget validates a del target; starred targets are
// rejected.
func (p *parser) checkDelTarget(expr Expr) error {
	switch t := expr.(type) {
	case *NameExpr, *AttributeExpr, *SubscriptExpr:
		return nil
	case *StarredExpr:
		return p.FailAt(t.Span, "cannot delete a starred target")
	case *TupleExpr:
		return p.checkAllDelTargets(t.Elts)
	case *ListExpr:
		return p.checkAllDelTargets(t.Elts)
	case *CallExpr:
		return p.FailAt(t.Span, "cannot delete function call")
	case *IntLit, *FloatLit, *ComplexLit, *StringLit, *BytesLit, *BoolLit, *NoneLit, *EllipsisLit, *JoinedStr:
		return p.FailAt(t.Pos(), "cannot delete literal")
	}
	return p.FailAt(expr.Pos(), "invalid delete target")
}

func (p *parser) checkAllDelTargets(targets []Expr) error {
	for _, target := range targets {
		if err := p.checkDelTarget(target); err != nil {
			return err
		}
	}
	return nil
}
