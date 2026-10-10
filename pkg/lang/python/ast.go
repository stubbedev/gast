package python

import "github.com/stubbedev/gast/pkg/gast"

// Node is implemented by every AST node; Pos returns the node's span.
type Node interface {
	Pos() gast.Span
}

// Stmt is a node that can appear in a statement list.
type Stmt interface {
	Node
	stmt()
}

// Expr is a node that can appear in an expression position.
type Expr interface {
	Node
	expr()
}

// Pattern is a node that can appear in a match case.
type Pattern interface {
	Node
	pattern()
}

// File is the root node of a parsed module.
type File struct {
	gast.Span
	Stmts []Stmt
}

// Statements.

// ExpressionStmt is an expression evaluated for its effect.
type ExpressionStmt struct {
	gast.Span
	Expr Expr
}

// SeqStmt is a semicolon-separated run of simple statements on one
// logical line.
type SeqStmt struct {
	gast.Span
	Stmts []Stmt
}

// AssignStmt is an assignment with one or more targets.
type AssignStmt struct {
	gast.Span
	Targets []Expr
	Value   Expr
}

// AugAssignStmt is an augmented assignment; Op is the operator text.
type AugAssignStmt struct {
	gast.Span
	Target Expr
	Op     string
	Value  Expr
}

// AnnAssignStmt is an annotated assignment; Value is nil for a bare
// annotation and Simple marks a plain-name target.
type AnnAssignStmt struct {
	gast.Span
	Target     Expr
	Annotation Expr
	Value      Expr
	Simple     bool
}

// PassStmt is a pass statement.
type PassStmt struct{ gast.Span }

// BreakStmt is a break statement.
type BreakStmt struct{ gast.Span }

// ContinueStmt is a continue statement.
type ContinueStmt struct{ gast.Span }

// ReturnStmt is a return with an optional value.
type ReturnStmt struct {
	gast.Span
	Value Expr
}

// RaiseStmt is a raise with optional exception and cause.
type RaiseStmt struct {
	gast.Span
	Exc  Expr
	From Expr
}

// DelStmt is a del statement.
type DelStmt struct {
	gast.Span
	Targets []Expr
}

// AssertStmt is an assert with an optional message.
type AssertStmt struct {
	gast.Span
	Test Expr
	Msg  Expr
}

// GlobalStmt is a global declaration.
type GlobalStmt struct {
	gast.Span
	Names []string
}

// NonlocalStmt is a nonlocal declaration.
type NonlocalStmt struct {
	gast.Span
	Names []string
}

// ImportStmt is an import statement.
type ImportStmt struct {
	gast.Span
	Names []Alias
}

// ImportFromStmt is a from-import statement; Dots counts the leading
// relative-import dots.
type ImportFromStmt struct {
	gast.Span
	Dots   int
	Module string
	Names  []Alias
	Star   bool
}

// Alias is one `name as alias` entry of an import.
type Alias struct {
	gast.Span
	Name   string
	AsName string
}

// IfStmt is an if statement; elif chains appear as nested IfStmts in
// Orelse, matching the reference AST.
type IfStmt struct {
	gast.Span
	Cond   Expr
	Body   []Stmt
	Orelse []Stmt
}

// WhileStmt is a while loop with an optional else clause.
type WhileStmt struct {
	gast.Span
	Cond   Expr
	Body   []Stmt
	Orelse []Stmt
}

// ForStmt is a for loop with an optional else clause.
type ForStmt struct {
	gast.Span
	Async  bool
	Target Expr
	Iter   Expr
	Body   []Stmt
	Orelse []Stmt
}

// TryStmt is a try statement; a handler with Star set came from
// except*.
type TryStmt struct {
	gast.Span
	Body      []Stmt
	Handlers  []*ExceptHandler
	Orelse    []Stmt
	Finalbody []Stmt
}

// ExceptHandler is one except or except* clause; Name is empty for a
// type-only catch.
type ExceptHandler struct {
	gast.Span
	Star bool
	Type Expr
	Name string
	Body []Stmt
}

// WithStmt is a with statement.
type WithStmt struct {
	gast.Span
	Async bool
	Items []*WithItem
	Body  []Stmt
}

// WithItem is one context manager of a WithStmt; OptionalVars is nil
// without `as`.
type WithItem struct {
	gast.Span
	Context      Expr
	OptionalVars Expr
}

// FunctionDef is a function declaration.
type FunctionDef struct {
	gast.Span
	Decorators []Decorator
	Async      bool
	TypeParams []*TypeParam
	Name       string
	Params     *Parameters
	Returns    Expr
	Body       []Stmt
}

// ClassDef is a class declaration.
type ClassDef struct {
	gast.Span
	Decorators []Decorator
	TypeParams []*TypeParam
	Name       string
	Bases      []Expr
	Keywords   []Keyword
	Body       []Stmt
}

// Decorator is one `@expr` decorator of a declaration.
type Decorator struct {
	gast.Span
	Expr Expr
}

// TypeAliasStmt is a `type name = value` statement.
type TypeAliasStmt struct {
	gast.Span
	TypeParams []*TypeParam
	Name       Expr
	Value      Expr
}

// TypeParam is one generic type parameter with optional bound and
// default.
type TypeParam struct {
	gast.Span
	Name    string
	Bound   Expr
	Default Expr
}

// MatchStmt is a match statement.
type MatchStmt struct {
	gast.Span
	Subject Expr
	Cases   []*MatchCase
}

// MatchCase is one case clause of a MatchStmt; Guard is nil without an
// if clause.
type MatchCase struct {
	gast.Span
	Pattern Pattern
	Guard   Expr
	Body    []Stmt
}

// Parameters is a parameter list split into Python's four sections.
type Parameters struct {
	gast.Span
	PosOnly      []*Param
	PosOrKeyword []*Param
	Vararg       *Param
	KwOnly       []*Param
	Kwarg        *Param
	StarMarker   bool
}

// Param is one parameter; Annotation and Default are nil when absent.
type Param struct {
	gast.Span
	Name       string
	Annotation Expr
	Default    Expr
}

// Keyword is one `name=value` entry of a class definition.
type Keyword struct {
	gast.Span
	Name  string
	Value Expr
}

// Expressions.

// NameExpr is a bare identifier.
type NameExpr struct {
	gast.Span
	ID string
}

// IntLit is an integer literal; Text is the raw spelling.
type IntLit struct {
	gast.Span
	Text string
}

// FloatLit is a float literal; Text is the raw spelling.
type FloatLit struct {
	gast.Span
	Text string
}

// ComplexLit is an imaginary literal; Text is the raw spelling.
type ComplexLit struct {
	gast.Span
	Text string
}

// StringLit is a string literal; Text is the raw source and Value the
// decoded contents.
type StringLit struct {
	gast.Span
	Text  string
	Value string
}

// BytesLit is a bytes literal; Value is the decoded contents.
type BytesLit struct {
	gast.Span
	Text  string
	Value string
}

// BoolLit is True or False.
type BoolLit struct {
	gast.Span
	Value bool
}

// NoneLit is the None literal.
type NoneLit struct{ gast.Span }

// EllipsisLit is the ... literal.
type EllipsisLit struct{ gast.Span }

// JoinedStr is an f-string or a concatenation involving f-strings; the
// parts are StringLit literals and FormattedValue fields.
type JoinedStr struct {
	gast.Span
	Parts []Expr
}

// FormattedValue is one `{...}` field of a JoinedStr; Conversion is
// "s", "r", "a" or empty, FormatSpec is a JoinedStr or nil, and Debug
// marks the `=` specifier.
type FormattedValue struct {
	gast.Span
	Value      Expr
	Conversion string
	FormatSpec Expr
	Debug      bool
}

// TupleExpr is a tuple; Parenthesized distinguishes `(1, 2)` from
// `1, 2`.
type TupleExpr struct {
	gast.Span
	Elts          []Expr
	Parenthesized bool
}

// ListExpr is a list display.
type ListExpr struct {
	gast.Span
	Elts []Expr
}

// SetExpr is a set display.
type SetExpr struct {
	gast.Span
	Elts []Expr
}

// DictItem is one entry of a dict display; Key is nil for a ** unpack.
type DictItem struct {
	gast.Span
	Key   Expr
	Value Expr
}

// DictExpr is a dict display.
type DictExpr struct {
	gast.Span
	Items []*DictItem
}

// StarredExpr is a `*expr` unpack in a target, display, or call.
type StarredExpr struct {
	gast.Span
	Value Expr
}

// AttributeExpr is `value.attr`.
type AttributeExpr struct {
	gast.Span
	Value Expr
	Attr  string
}

// SubscriptExpr is `value[index]`.
type SubscriptExpr struct {
	gast.Span
	Value Expr
	Index Expr
}

// SliceExpr is `lower:upper:step` inside a subscript; absent bounds
// are nil.
type SliceExpr struct {
	gast.Span
	Lower Expr
	Upper Expr
	Step  Expr
}

// CallExpr is `func(args)`.
type CallExpr struct {
	gast.Span
	Func Expr
	Args []*Arg
}

// Arg is one call argument; Star is "", "*", or "**" and Name holds a
// keyword label.
type Arg struct {
	gast.Span
	Name string
	Star string
	Expr Expr
}

// LambdaExpr is a lambda; Body is the result expression.
type LambdaExpr struct {
	gast.Span
	Params *Parameters
	Body   Expr
}

// IfExpExpr is `body if test else orelse`.
type IfExpExpr struct {
	gast.Span
	Test   Expr
	Body   Expr
	Orelse Expr
}

// BinOpExpr is a binary operation; Op is the operator text.
type BinOpExpr struct {
	gast.Span
	Op    string
	Left  Expr
	Right Expr
}

// BoolOpExpr is a chained and/or over two or more values; Op is "and"
// or "or".
type BoolOpExpr struct {
	gast.Span
	Op     string
	Values []Expr
}

// UnaryOpExpr is a prefix operation; Op is "+", "-", "~", or "not".
type UnaryOpExpr struct {
	gast.Span
	Op      string
	Operand Expr
}

// CompareExpr is a comparison chain collecting every operator and
// comparator: a < b <= c is one node with two ops.
type CompareExpr struct {
	gast.Span
	Left        Expr
	Ops         []string
	Comparators []Expr
}

// AwaitExpr is `await value`.
type AwaitExpr struct {
	gast.Span
	Value Expr
}

// YieldExpr is `yield value`; Value is nil for a bare yield.
type YieldExpr struct {
	gast.Span
	Value Expr
}

// YieldFromExpr is `yield from value`.
type YieldFromExpr struct {
	gast.Span
	Value Expr
}

// NamedExpr is the walrus `target := value`.
type NamedExpr struct {
	gast.Span
	Target Expr
	Value  Expr
}

// ComprehensionExpr is a list, set, dict, or generator comprehension;
// Kind is one of "list", "set", "dict", or "generator", and Key is set
// for dict comprehensions.
type ComprehensionExpr struct {
	gast.Span
	Kind       string
	Elt        Expr
	Key        Expr
	Generators []*ComprehensionClause
}

// ComprehensionClause is one `for ... in ...` of a comprehension,
// including its `if` filters.
type ComprehensionClause struct {
	gast.Span
	Async  bool
	Target Expr
	Iter   Expr
	Ifs    []Expr
}

// Patterns.

// MatchValue is a literal or dotted-value pattern.
type MatchValue struct {
	gast.Span
	Value Expr
}

// MatchSingleton is a True, False, or None pattern.
type MatchSingleton struct {
	gast.Span
	Value Expr
}

// MatchSequence is a `[a, *rest]` or open-sequence pattern.
type MatchSequence struct {
	gast.Span
	Patterns []Pattern
}

// MatchStar is a `*name` pattern within a sequence.
type MatchStar struct {
	gast.Span
	Name string
}

// MatchMapping is a `{key: pattern, **rest}` pattern.
type MatchMapping struct {
	gast.Span
	Keys     []Expr
	Patterns []Pattern
	Rest     string
}

// MatchClass is a `Cls(pos, kw=pattern)` pattern.
type MatchClass struct {
	gast.Span
	Cls         Expr
	Patterns    []Pattern
	KwdNames    []string
	KwdPatterns []Pattern
}

// MatchAs is a capture, wildcard, or `pattern as name` binding.
type MatchAs struct {
	gast.Span
	Pattern Pattern
	Name    string
}

// MatchOr is an or-pattern `a | b`.
type MatchOr struct {
	gast.Span
	Patterns []Pattern
}

func (File) stmt()           {}
func (ExpressionStmt) stmt() {}
func (SeqStmt) stmt()        {}
func (AssignStmt) stmt()     {}
func (AugAssignStmt) stmt()  {}
func (AnnAssignStmt) stmt()  {}
func (PassStmt) stmt()       {}
func (BreakStmt) stmt()      {}
func (ContinueStmt) stmt()   {}
func (ReturnStmt) stmt()     {}
func (RaiseStmt) stmt()      {}
func (DelStmt) stmt()        {}
func (AssertStmt) stmt()     {}
func (GlobalStmt) stmt()     {}
func (NonlocalStmt) stmt()   {}
func (ImportStmt) stmt()     {}
func (ImportFromStmt) stmt() {}
func (IfStmt) stmt()         {}
func (WhileStmt) stmt()      {}
func (ForStmt) stmt()        {}
func (TryStmt) stmt()        {}
func (WithStmt) stmt()       {}
func (FunctionDef) stmt()    {}
func (ClassDef) stmt()       {}
func (TypeAliasStmt) stmt()  {}
func (MatchStmt) stmt()      {}

func (NameExpr) expr()          {}
func (IntLit) expr()            {}
func (FloatLit) expr()          {}
func (ComplexLit) expr()        {}
func (StringLit) expr()         {}
func (BytesLit) expr()          {}
func (BoolLit) expr()           {}
func (NoneLit) expr()           {}
func (EllipsisLit) expr()       {}
func (JoinedStr) expr()         {}
func (FormattedValue) expr()    {}
func (TupleExpr) expr()         {}
func (ListExpr) expr()          {}
func (SetExpr) expr()           {}
func (DictExpr) expr()          {}
func (StarredExpr) expr()       {}
func (AttributeExpr) expr()     {}
func (SubscriptExpr) expr()     {}
func (SliceExpr) expr()         {}
func (CallExpr) expr()          {}
func (LambdaExpr) expr()        {}
func (IfExpExpr) expr()         {}
func (BinOpExpr) expr()         {}
func (BoolOpExpr) expr()        {}
func (UnaryOpExpr) expr()       {}
func (CompareExpr) expr()       {}
func (AwaitExpr) expr()         {}
func (YieldExpr) expr()         {}
func (YieldFromExpr) expr()     {}
func (NamedExpr) expr()         {}
func (ComprehensionExpr) expr() {}

func (MatchValue) pattern()     {}
func (MatchSingleton) pattern() {}
func (MatchSequence) pattern()  {}
func (MatchStar) pattern()      {}
func (MatchMapping) pattern()   {}
func (MatchClass) pattern()     {}
func (MatchAs) pattern()        {}
func (MatchOr) pattern()        {}
