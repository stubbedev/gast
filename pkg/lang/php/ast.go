package php

import (
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

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

// File is the root node of a parsed source file.
type File struct {
	gast.Span
	Stmts []Stmt
}

// Statements.

// InlineHTMLStmt is source text outside of php tags.
type InlineHTMLStmt struct {
	gast.Span
	Text string
}

// ExpressionStmt is an expression evaluated for its effect.
type ExpressionStmt struct {
	gast.Span
	Expr Expr
}

// EchoStmt is an echo statement.
type EchoStmt struct {
	gast.Span
	Args []Expr
}

// BlockStmt is a braced statement list.
type BlockStmt struct {
	gast.Span
	Stmts []Stmt
}

// NopStmt is an empty statement.
type NopStmt struct{ gast.Span }

// IfStmt is an if with optional elseif/else arms.
type IfStmt struct {
	gast.Span
	Cond    Expr
	Then    Stmt
	ElseIfs []ElseIf
	Else    Stmt
}

// ElseIf is one elseif arm of an IfStmt.
type ElseIf struct {
	gast.Span
	Cond Expr
	Then Stmt
}

// WhileStmt is a while loop.
type WhileStmt struct {
	gast.Span
	Cond Expr
	Body Stmt
}

// DoWhileStmt is a do-while loop.
type DoWhileStmt struct {
	gast.Span
	Cond Expr
	Body Stmt
}

// ForStmt is a for loop.
type ForStmt struct {
	gast.Span
	Init []Expr
	Cond []Expr
	Step []Expr
	Body Stmt
}

// ForeachStmt is a foreach loop.
type ForeachStmt struct {
	gast.Span
	Iter  Expr
	Key   Expr
	Value Expr
	ByRef bool
	Body  Stmt
}

// SwitchStmt is a switch statement.
type SwitchStmt struct {
	gast.Span
	Cond  Expr
	Cases []SwitchCase
}

// SwitchCase is one case or default arm of a SwitchStmt; Cond is nil for
// default.
type SwitchCase struct {
	gast.Span
	Cond Expr
	Body []Stmt
}

// BreakStmt is a break with an optional level.
type BreakStmt struct {
	gast.Span
	Level Expr
}

// ContinueStmt is a continue with an optional level.
type ContinueStmt struct {
	gast.Span
	Level Expr
}

// ReturnStmt is a return with an optional value.
type ReturnStmt struct {
	gast.Span
	Expr Expr
}

// GotoStmt is a goto statement.
type GotoStmt struct {
	gast.Span
	Label string
}

// LabelStmt is a `name:` label.
type LabelStmt struct {
	gast.Span
	Name string
}

// TryStmt is a try with catches and an optional finally.
type TryStmt struct {
	gast.Span
	Body    *BlockStmt
	Catches []*CatchClause
	Finally *BlockStmt
}

// CatchClause is one catch of a TryStmt. Var is nil for a type-only catch.
type CatchClause struct {
	gast.Span
	Types []Name
	Var   *Variable
	Body  *BlockStmt
}

// DeclareStmt is a declare directive with a statement body.
type DeclareStmt struct {
	gast.Span
	Directives []DeclareDirective
	Body       Stmt
}

// DeclareDirective is one `name=value` entry of a declare.
type DeclareDirective struct {
	gast.Span
	Name  string
	Value Expr
}

// FunctionStmt is a named function declaration.
type FunctionStmt struct {
	gast.Span
	Attrs      []*AttributeGroup
	Ref        bool
	Name       string
	Params     []*Param
	ReturnType *Type
	Body       *BlockStmt
}

// ClassStmt is a class declaration.
type ClassStmt struct {
	gast.Span
	Attrs      []*AttributeGroup
	Modifiers  []string
	Name       string
	Extends    *Name
	Implements []Name
	Members    []ClassMember
}

// InterfaceStmt is an interface declaration.
type InterfaceStmt struct {
	gast.Span
	Attrs   []*AttributeGroup
	Name    string
	Extends []Name
	Members []ClassMember
}

// TraitStmt is a trait declaration.
type TraitStmt struct {
	gast.Span
	Attrs   []*AttributeGroup
	Name    string
	Members []ClassMember
}

// EnumStmt is an enum declaration.
type EnumStmt struct {
	gast.Span
	Attrs      []*AttributeGroup
	Name       string
	ScalarType *Type
	Implements []Name
	Members    []ClassMember
}

// NamespaceStmt is a namespace declaration. Body is nil for the simple
// form, and Name is empty for a braced anonymous namespace.
type NamespaceStmt struct {
	gast.Span
	Name string
	Body *BlockStmt
}

// UseStmt is a use import statement.
type UseStmt struct {
	gast.Span
	Kind    string
	Imports []UseImport
}

// UseImport is one imported name of a UseStmt.
type UseImport struct {
	gast.Span
	Name  Name
	Alias string
}

// ConstStmt is a top-level const declaration.
type ConstStmt struct {
	gast.Span
	Consts []ConstEntry
}

// ConstEntry is one name=value pair of a const declaration.
type ConstEntry struct {
	gast.Span
	Name  string
	Value Expr
}

// GlobalStmt is a global variable statement.
type GlobalStmt struct {
	gast.Span
	Vars []Expr
}

// StaticStmt is a static variable statement.
type StaticStmt struct {
	gast.Span
	Vars []StaticVarEntry
}

// StaticVarEntry is one variable of a StaticStmt.
type StaticVarEntry struct {
	gast.Span
	Var  *Variable
	Init Expr
}

// UnsetStmt is an unset statement.
type UnsetStmt struct {
	gast.Span
	Vars []Expr
}

// ClassMember is a member of a class-like declaration.
type ClassMember interface {
	Node
	classMember()
}

// Method is a method declaration; Body is nil for abstract methods.
type Method struct {
	gast.Span
	Attrs      []*AttributeGroup
	Modifiers  []string
	Ref        bool
	Name       string
	Params     []*Param
	ReturnType *Type
	Body       *BlockStmt
}

// Property is a property declaration; Type is nil for untyped var
// declarations.
type Property struct {
	gast.Span
	Attrs     []*AttributeGroup
	Modifiers []string
	Type      *Type
	Vars      []PropertyVar
}

// PropertyVar is one variable of a Property.
type PropertyVar struct {
	gast.Span
	Var     *Variable
	Default Expr
}

// ClassConst is a class constant declaration; Type covers PHP 8.3
// typed constants.
type ClassConst struct {
	gast.Span
	Attrs     []*AttributeGroup
	Modifiers []string
	Type      *Type
	Consts    []ConstEntry
}

// EnumCase is an enum case.
type EnumCase struct {
	gast.Span
	Attrs []*AttributeGroup
	Name  string
	Value Expr
}

// TraitUse is a trait use clause.
type TraitUse struct {
	gast.Span
	Traits      []Name
	Adaptations []TraitAdaptation
}

// TraitAdaptation is an insteadof or as adaptation inside a TraitUse.
type TraitAdaptation interface {
	Node
	traitAdaptation()
}

// TraitInsteadOf is a `method insteadof trait` adaptation.
type TraitInsteadOf struct {
	gast.Span
	Method    MethodRef
	InsteadOf []Name
}

// TraitAs is a `method as alias` adaptation.
type TraitAs struct {
	gast.Span
	Method     MethodRef
	Alias      string
	Visibility string
}

// MethodRef names a method in a trait adaptation, optionally qualified by
// a trait: `Trait::method` or `method`.
type MethodRef struct {
	gast.Span
	Trait  *Name
	Method string
}

// Expressions.

// IntegerLit is an integer literal; Text is the raw spelling.
type IntegerLit struct {
	gast.Span
	Text string
}

// FloatLit is a float literal; Text is the raw spelling.
type FloatLit struct {
	gast.Span
	Text string
}

// StringLit is a string, heredoc, or shell-exec literal.
type StringLit struct {
	gast.Span
	Kind  string
	Text  string
	Value string
}

// BooleanLit is true or false.
type BooleanLit struct {
	gast.Span
	Value bool
}

// NullLit is the null literal.
type NullLit struct{ gast.Span }

// Variable is a $name variable; Name excludes the $.
type Variable struct {
	gast.Span
	Name string
}

// VariableVariable is a $$expr variable.
type VariableVariable struct {
	gast.Span
	Expr Expr
}

// Name is a (possibly qualified) identifier.
type Name struct {
	gast.Span
	Qualified bool
	Parts     []string
}

// String renders the name with namespace separators.
func (n Name) String() string {
	var builder strings.Builder
	if n.Qualified {
		builder.WriteByte('\\')
	}
	for index, part := range n.Parts {
		if index > 0 {
			builder.WriteByte('\\')
		}
		builder.WriteString(part)
	}
	return builder.String()
}

// Identifier is a bare member name such as a method or property name
// after -> or ::.
type Identifier struct {
	gast.Span
	Name string
}

// ConstFetch is a bare constant name.
type ConstFetch struct {
	gast.Span
	Name Name
}

// ClassConstFetch is `class::CONST` (also ::class).
type ClassConstFetch struct {
	gast.Span
	Class Expr
	Const string
}

// StaticPropertyFetch is `class::$prop`.
type StaticPropertyFetch struct {
	gast.Span
	Class Expr
	Prop  *Variable
}

// StaticCall is `class::method(args)`.
type StaticCall struct {
	gast.Span
	Class  Expr
	Method string
	Args   []*Arg
}

// PropertyFetch is `target->name`; Name may be a Variable.
type PropertyFetch struct {
	gast.Span
	Target Expr
	Name   Expr
}

// NullsafePropertyFetch is `target?->name`.
type NullsafePropertyFetch struct {
	gast.Span
	Target Expr
	Name   Expr
}

// MethodCall is `target->method(args)`.
type MethodCall struct {
	gast.Span
	Target Expr
	Method Expr
	Args   []*Arg
}

// NullsafeMethodCall is `target?->method(args)`.
type NullsafeMethodCall struct {
	gast.Span
	Target Expr
	Method Expr
	Args   []*Arg
}

// IndexExpr is `target[index]`; Index is nil for the append form `$a[]`.
type IndexExpr struct {
	gast.Span
	Target Expr
	Index  Expr
}

// CallExpr is `func(args)`.
type CallExpr struct {
	gast.Span
	Func Expr
	Args []*Arg
}

// Arg is one call argument; Name holds a PHP 8 named-argument label.
type Arg struct {
	gast.Span
	Name   string
	Expr   Expr
	ByRef  bool
	Spread bool
}

// ArrayExpr is an array literal, in either [] or array() syntax.
type ArrayExpr struct {
	gast.Span
	Items  []*ArrayItem
	Legacy bool
}

// ArrayItem is one element of an ArrayExpr; Value is nil for empty
// list() slots.
type ArrayItem struct {
	gast.Span
	Key    Expr
	Value  Expr
	ByRef  bool
	Spread bool
}

// BinaryExpr is a binary operation; Op is the operator text.
type BinaryExpr struct {
	gast.Span
	Op    string
	Left  Expr
	Right Expr
}

// UnaryExpr is a prefix or postfix operation; Op is the operator text
// (including print and the include/require family).
type UnaryExpr struct {
	gast.Span
	Op   string
	Expr Expr
	Post bool
}

// AssignExpr is an assignment or compound assignment; Op is the operator
// text and Ref marks `= &expr`.
type AssignExpr struct {
	gast.Span
	Op     string
	Target Expr
	Value  Expr
	Ref    bool
}

// TernaryExpr is `cond ? then : else`; Then is nil for `cond ?: else`.
type TernaryExpr struct {
	gast.Span
	Cond Expr
	Then Expr
	Else Expr
}

// CastExpr is a cast; Cast is the canonical lowercase type name.
type CastExpr struct {
	gast.Span
	Cast string
	Expr Expr
}

// InstanceofExpr is `expr instanceof class`.
type InstanceofExpr struct {
	gast.Span
	Expr  Expr
	Class Expr
}

// IssetExpr is `isset(...)`.
type IssetExpr struct {
	gast.Span
	Args []Expr
}

// EmptyExpr is `empty(...)`.
type EmptyExpr struct {
	gast.Span
	Arg Expr
}

// NewExpr is `new class(args)`.
type NewExpr struct {
	gast.Span
	Class Expr
	Args  []*Arg
}

// AnonymousClassExpr is `new class(args) {...}`.
type AnonymousClassExpr struct {
	gast.Span
	Attrs      []*AttributeGroup
	Modifiers  []string
	Args       []*Arg
	Extends    *Name
	Implements []Name
	Members    []ClassMember
}

// ClosureExpr is a closure literal.
type ClosureExpr struct {
	gast.Span
	Attrs      []*AttributeGroup
	Static     bool
	Ref        bool
	Params     []*Param
	Uses       []ClosureUse
	ReturnType *Type
	Body       *BlockStmt
}

// ArrowFnExpr is an arrow function literal.
type ArrowFnExpr struct {
	gast.Span
	Attrs      []*AttributeGroup
	Static     bool
	Ref        bool
	Params     []*Param
	ReturnType *Type
	Expr       Expr
}

// ClosureUse is one variable of a closure use clause.
type ClosureUse struct {
	gast.Span
	Var   *Variable
	ByRef bool
}

// MatchExpr is a match expression.
type MatchExpr struct {
	gast.Span
	Cond Expr
	Arms []MatchArm
}

// MatchArm is one arm of a MatchExpr; Conds is nil for default.
type MatchArm struct {
	gast.Span
	Conds []Expr
	Body  Expr
}

// YieldExpr is a yield or yield from expression.
type YieldExpr struct {
	gast.Span
	From  bool
	Key   Expr
	Value Expr
}

// ThrowExpr is a throw expression.
type ThrowExpr struct {
	gast.Span
	Expr Expr
}

// Shared node types.

// AttributeGroup is one `#[...]` group.
type AttributeGroup struct {
	gast.Span
	Attrs []*Attribute
}

// Attribute is one attribute of an AttributeGroup.
type Attribute struct {
	gast.Span
	Name Name
	Args []*Arg
}

// Param is a function or closure parameter.
type Param struct {
	gast.Span
	Attrs    []*AttributeGroup
	Type     *Type
	ByRef    bool
	Variadic bool
	Var      *Variable
	Default  Expr
	Promoted []string
}

// Type is a type declaration: nullable, and either a union (|) or
// intersection (&) whose members are flattened into Names.
type Type struct {
	gast.Span
	Nullable  bool
	Separator byte
	Names     []Name
}

func (File) stmt()           {}
func (InlineHTMLStmt) stmt() {}
func (ExpressionStmt) stmt() {}
func (EchoStmt) stmt()       {}
func (BlockStmt) stmt()      {}
func (NopStmt) stmt()        {}
func (IfStmt) stmt()         {}
func (WhileStmt) stmt()      {}
func (DoWhileStmt) stmt()    {}
func (ForStmt) stmt()        {}
func (ForeachStmt) stmt()    {}
func (SwitchStmt) stmt()     {}
func (BreakStmt) stmt()      {}
func (ContinueStmt) stmt()   {}
func (ReturnStmt) stmt()     {}
func (GotoStmt) stmt()       {}
func (LabelStmt) stmt()      {}
func (TryStmt) stmt()        {}
func (DeclareStmt) stmt()    {}
func (FunctionStmt) stmt()   {}
func (ClassStmt) stmt()      {}
func (InterfaceStmt) stmt()  {}
func (TraitStmt) stmt()      {}
func (EnumStmt) stmt()       {}
func (NamespaceStmt) stmt()  {}
func (UseStmt) stmt()        {}
func (ConstStmt) stmt()      {}
func (GlobalStmt) stmt()     {}
func (StaticStmt) stmt()     {}
func (UnsetStmt) stmt()      {}

func (Method) classMember()     {}
func (Property) classMember()   {}
func (ClassConst) classMember() {}
func (EnumCase) classMember()   {}
func (TraitUse) classMember()   {}

func (TraitInsteadOf) traitAdaptation() {}
func (TraitAs) traitAdaptation()        {}

func (IntegerLit) expr()            {}
func (FloatLit) expr()              {}
func (StringLit) expr()             {}
func (BooleanLit) expr()            {}
func (NullLit) expr()               {}
func (Variable) expr()              {}
func (VariableVariable) expr()      {}
func (Identifier) expr()            {}
func (ConstFetch) expr()            {}
func (ClassConstFetch) expr()       {}
func (StaticPropertyFetch) expr()   {}
func (StaticCall) expr()            {}
func (PropertyFetch) expr()         {}
func (NullsafePropertyFetch) expr() {}
func (MethodCall) expr()            {}
func (NullsafeMethodCall) expr()    {}
func (IndexExpr) expr()             {}
func (CallExpr) expr()              {}
func (ArrayExpr) expr()             {}
func (BinaryExpr) expr()            {}
func (UnaryExpr) expr()             {}
func (AssignExpr) expr()            {}
func (TernaryExpr) expr()           {}
func (CastExpr) expr()              {}
func (InstanceofExpr) expr()        {}
func (IssetExpr) expr()             {}
func (EmptyExpr) expr()             {}
func (NewExpr) expr()               {}
func (AnonymousClassExpr) expr()    {}
func (ClosureExpr) expr()           {}
func (ArrowFnExpr) expr()           {}
func (MatchExpr) expr()             {}
func (YieldExpr) expr()             {}
func (ThrowExpr) expr()             {}
