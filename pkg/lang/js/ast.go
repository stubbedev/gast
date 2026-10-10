package js

import (
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

// Pattern is a node that can appear in a binding position.
type Pattern interface {
	Node
	pattern()
}

// File is the root node of a parsed source file.
type File struct {
	gast.Span
	Stmts []Stmt
}

// Statements.

// BlockStmt is a braced statement list.
type BlockStmt struct {
	gast.Span
	Stmts []Stmt
}

// EmptyStmt is a stray semicolon.
type EmptyStmt struct{ gast.Span }

// DebuggerStmt is a debugger statement.
type DebuggerStmt struct{ gast.Span }

// ExpressionStmt is an expression evaluated for its effect.
type ExpressionStmt struct {
	gast.Span
	Expr Expr
}

// VarStmt is a var, let, or const declaration; Kind is one of "var",
// "let", "const".
type VarStmt struct {
	gast.Span
	Kind  string
	Decls []*VarDeclarator
}

// VarDeclarator is one binding=initializer pair of a VarStmt; Init is
// nil for bare declarations.
type VarDeclarator struct {
	gast.Span
	Binding Pattern
	Init    Expr
}

// IfStmt is an if with an optional else arm.
type IfStmt struct {
	gast.Span
	Cond Expr
	Then Stmt
	Else Stmt
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

// ForStmt is a three-clause for loop; any of Init, Test, and Update may
// be nil. Init is either a *VarStmt (without a semicolon) or an
// *ExpressionStmt.
type ForStmt struct {
	gast.Span
	Init   Stmt
	Test   Expr
	Update Expr
	Body   Stmt
}

// ForInStmt is a for-in loop; Left is a *VarStmt or a target expression.
type ForInStmt struct {
	gast.Span
	Left  Stmt
	Right Expr
	Body  Stmt
}

// ForOfStmt is a for-of loop, with Await set for for-await-of; Left is
// a *VarStmt or a target expression.
type ForOfStmt struct {
	gast.Span
	Await bool
	Left  Stmt
	Right Expr
	Body  Stmt
}

// SwitchStmt is a switch statement.
type SwitchStmt struct {
	gast.Span
	Disc  Expr
	Cases []*SwitchCase
}

// SwitchCase is one case or default arm of a SwitchStmt; Test is nil
// for default. Fallthrough is legal, so a case body is a plain
// statement list.
type SwitchCase struct {
	gast.Span
	Test Expr
	Body []Stmt
}

// TryStmt is a try with an optional catch and finally.
type TryStmt struct {
	gast.Span
	Block   *BlockStmt
	Catch   *CatchClause
	Finally *BlockStmt
}

// CatchClause is the catch of a TryStmt; Param is nil for an optional
// catch binding.
type CatchClause struct {
	gast.Span
	Param Pattern
	Block *BlockStmt
}

// ThrowStmt is a throw statement.
type ThrowStmt struct {
	gast.Span
	Expr Expr
}

// ReturnStmt is a return with an optional value.
type ReturnStmt struct {
	gast.Span
	Expr Expr
}

// BreakStmt is a break with an optional label.
type BreakStmt struct {
	gast.Span
	Label string
}

// ContinueStmt is a continue with an optional label.
type ContinueStmt struct {
	gast.Span
	Label string
}

// LabeledStmt is a `label:` statement.
type LabeledStmt struct {
	gast.Span
	Label string
	Body  Stmt
}

// WithStmt is a with statement (sloppy mode).
type WithStmt struct {
	gast.Span
	Object Expr
	Body   Stmt
}

// FunctionDecl is a function declaration; Name is empty only for
// export default.
type FunctionDecl struct {
	gast.Span
	Async     bool
	Generator bool
	Name      string
	Params    []*Param
	Body      *BlockStmt
}

// ClassDecl is a class declaration.
type ClassDecl struct {
	gast.Span
	Name    string
	Extends Expr
	Body    []ClassMember
}

// ImportDecl is an import declaration; Default and Namespace hold the
// local names they bind, "" when absent.
type ImportDecl struct {
	gast.Span
	Default   string
	Namespace string
	Named     []ImportSpec
	Source    string
}

// ImportSpec is one named import; Imported is the exported name (or
// module string name) and Local the local binding.
type ImportSpec struct {
	gast.Span
	Imported string
	Local    string
}

// ExportNamedStmt is an export of local declarations or an export list;
// Decl is nil for the list form, which may re-export from Source.
type ExportNamedStmt struct {
	gast.Span
	Decl   Stmt
	Specs  []ExportSpec
	Source string
}

// ExportSpec is one entry of an export list; Local is the local name
// (or a module string name in export-from) and Exported the exported
// name, equal to Local when no "as" clause appears.
type ExportSpec struct {
	gast.Span
	Local    string
	Exported string
}

// ExportAllStmt is `export * [as name] from source`.
type ExportAllStmt struct {
	gast.Span
	Exported string
	Source   string
}

// ExportDefaultStmt is `export default ...`; Decl holds a
// FunctionDecl or ClassDecl when the default exports an anonymous
// declaration, otherwise Value holds the expression.
type ExportDefaultStmt struct {
	gast.Span
	Decl  Stmt
	Value Expr
}

// Class members.

// ClassMember is a member of a class body.
type ClassMember interface {
	Node
	classMember()
}

// MethodDef is a method, constructor, getter, or setter; Kind is one of
// "method", "constructor", "get", "set".
type MethodDef struct {
	gast.Span
	Key       Expr
	Static    bool
	Async     bool
	Generator bool
	Kind      string
	Params    []*Param
	Body      *BlockStmt
}

// PropertyDef is a field declaration; Init is nil for bare fields.
type PropertyDef struct {
	gast.Span
	Key      Expr
	Init     Expr
	Static   bool
	Computed bool
}

// StaticBlock is a class static initialization block.
type StaticBlock struct {
	gast.Span
	Stmts []Stmt
}

// Expressions.

// IdentExpr is an identifier reference; it doubles as a binding
// pattern.
type IdentExpr struct {
	gast.Span
	Name string
}

// PrivateName is a #name reference; Name excludes the #.
type PrivateName struct {
	gast.Span
	Name string
}

// ThisExpr is the this keyword.
type ThisExpr struct{ gast.Span }

// SuperExpr is the super keyword; only valid in member and call
// positions.
type SuperExpr struct{ gast.Span }

// NumberLit is a numeric literal; Text is the raw spelling.
type NumberLit struct {
	gast.Span
	Text string
}

// BigIntLit is a BigInt literal; Text is the raw spelling with the n
// suffix.
type BigIntLit struct {
	gast.Span
	Text string
}

// StringLit is a string literal; Text is the raw spelling and Value
// the decoded escapes.
type StringLit struct {
	gast.Span
	Text  string
	Value string
}

// RegexLit is a regex literal; Text is the raw spelling including
// slashes and flags.
type RegexLit struct {
	gast.Span
	Text string
}

// BoolLit is true or false.
type BoolLit struct {
	gast.Span
	Value bool
}

// NullLit is the null literal.
type NullLit struct{ gast.Span }

// TemplateLit is a template literal; Quasis holds the raw text between
// substitutions, so len(Quasis) == len(Exprs)+1.
type TemplateLit struct {
	gast.Span
	Quasis []string
	Exprs  []Expr
}

// TaggedTemplate is tag`...`; the template parts are laid out like
// TemplateLit.
type TaggedTemplate struct {
	gast.Span
	Tag    Expr
	Quasis []string
	Exprs  []Expr
}

// ArrayExpr is an array literal; a nil element is a hole, and spread
// elements are *SpreadElement.
type ArrayExpr struct {
	gast.Span
	Elements []Expr
}

// SpreadElement is a ...expr spread in call arguments and array
// literals.
type SpreadElement struct {
	gast.Span
	Expr Expr
}

// ObjectExpr is an object literal; Props holds *Property and
// *SpreadProperty nodes.
type ObjectExpr struct {
	gast.Span
	Props []Expr
}

// Property is one property of an ObjectExpr; Kind is "init", "get",
// "set", or "method". Default holds a pattern initializer in the
// destructuring cover grammar and is only legal when the object is an
// assignment target.
type Property struct {
	gast.Span
	Key       Expr
	Value     Expr
	Default   Expr
	Kind      string
	Computed  bool
	Shorthand bool
}

// SpreadProperty is ...expr in an object literal, a rest in the
// destructuring cover grammar.
type SpreadProperty struct {
	gast.Span
	Expr Expr
}

// ParenExpr is a parenthesized expression.
type ParenExpr struct {
	gast.Span
	Expr Expr
}

// FunctionExpr is a function expression; Name is empty for anonymous
// functions.
type FunctionExpr struct {
	gast.Span
	Async     bool
	Generator bool
	Name      string
	Params    []*Param
	Body      *BlockStmt
}

// ClassExpr is a class expression; Name is empty for anonymous
// classes.
type ClassExpr struct {
	gast.Span
	Name    string
	Extends Expr
	Body    []ClassMember
}

// ArrowExpr is an arrow function; Body is a *BlockStmt or an
// expression, distinguished by Expression.
type ArrowExpr struct {
	gast.Span
	Async      bool
	Params     []*Param
	Body       Expr
	Expression bool
}

// UnaryExpr is a prefix operation; Op includes the keyword operators
// delete, typeof, void, and await.
type UnaryExpr struct {
	gast.Span
	Op   string
	Expr Expr
}

// UpdateExpr is ++ or --, prefix or postfix.
type UpdateExpr struct {
	gast.Span
	Op     string
	Expr   Expr
	Prefix bool
}

// BinaryExpr is a binary, logical, or nullish operation; Op is the
// operator text, including the keyword operators in and instanceof.
type BinaryExpr struct {
	gast.Span
	Op    string
	Left  Expr
	Right Expr
}

// AssignExpr is an assignment or compound assignment; Op is the
// operator text. Target stays in the expression cover grammar:
// destructuring sources appear as ObjectExpr and ArrayExpr.
type AssignExpr struct {
	gast.Span
	Op     string
	Target Expr
	Value  Expr
}

// CondExpr is cond ? then : else.
type CondExpr struct {
	gast.Span
	Cond Expr
	Then Expr
	Else Expr
}

// SequenceExpr is a comma-separated expression sequence.
type SequenceExpr struct {
	gast.Span
	Exprs []Expr
}

// CallExpr is callee(args); Optional marks a ?.() call.
type CallExpr struct {
	gast.Span
	Callee   Expr
	Args     []Expr
	Optional bool
}

// NewExpr is new callee(args); Args is nil when no argument list
// follows the callee.
type NewExpr struct {
	gast.Span
	Callee Expr
	Args   []Expr
}

// MemberExpr is obj.prop or obj[prop]; Optional marks a ?. access.
type MemberExpr struct {
	gast.Span
	Obj      Expr
	Prop     Expr
	Computed bool
	Optional bool
}

// YieldExpr is a yield or yield* expression.
type YieldExpr struct {
	gast.Span
	Delegate bool
	Arg      Expr
}

// AwaitExpr is an await expression.
type AwaitExpr struct {
	gast.Span
	Arg Expr
}

// ImportCall is a dynamic import(source).
type ImportCall struct {
	gast.Span
	Source Expr
}

// MetaProperty is new.target or import.meta.
type MetaProperty struct {
	gast.Span
	Meta string
	Prop string
}

// Shared nodes.

// Param is one function parameter; Rest marks ...param, which cannot
// carry a Default.
type Param struct {
	gast.Span
	Binding Pattern
	Default Expr
	Rest    bool
}

// ObjectPattern is an object destructuring binding.
type ObjectPattern struct {
	gast.Span
	Props []*BindingProp
}

// BindingProp is one property of an ObjectPattern; Value nil marks a
// shorthand binding. Rest is the ...rest property, which must be last.
type BindingProp struct {
	gast.Span
	Key      Expr
	Value    Pattern
	Default  Expr
	Computed bool
	Rest     bool
}

// ArrayPattern is an array destructuring binding; a nil element is a
// hole.
type ArrayPattern struct {
	gast.Span
	Elements []Pattern
}

// RestElement is a ...rest binding.
type RestElement struct {
	gast.Span
	Arg Pattern
}

// DefaultPattern is an array-pattern element with a default
// initializer, `[a = 1]`.
type DefaultPattern struct {
	gast.Span
	Binding Pattern
	Default Expr
}

func (File) stmt()              {}
func (BlockStmt) stmt()         {}
func (EmptyStmt) stmt()         {}
func (DebuggerStmt) stmt()      {}
func (ExpressionStmt) stmt()    {}
func (VarStmt) stmt()           {}
func (IfStmt) stmt()            {}
func (WhileStmt) stmt()         {}
func (DoWhileStmt) stmt()       {}
func (ForStmt) stmt()           {}
func (ForInStmt) stmt()         {}
func (ForOfStmt) stmt()         {}
func (SwitchStmt) stmt()        {}
func (TryStmt) stmt()           {}
func (ThrowStmt) stmt()         {}
func (ReturnStmt) stmt()        {}
func (BreakStmt) stmt()         {}
func (ContinueStmt) stmt()      {}
func (LabeledStmt) stmt()       {}
func (WithStmt) stmt()          {}
func (FunctionDecl) stmt()      {}
func (ClassDecl) stmt()         {}
func (ImportDecl) stmt()        {}
func (ExportNamedStmt) stmt()   {}
func (ExportAllStmt) stmt()     {}
func (ExportDefaultStmt) stmt() {}

func (MethodDef) classMember()   {}
func (PropertyDef) classMember() {}
func (StaticBlock) classMember() {}

func (IdentExpr) expr()      {}
func (BlockStmt) expr()      {} // arrow function block bodies
func (PrivateName) expr()    {}
func (ThisExpr) expr()       {}
func (SuperExpr) expr()      {}
func (NumberLit) expr()      {}
func (BigIntLit) expr()      {}
func (StringLit) expr()      {}
func (RegexLit) expr()       {}
func (BoolLit) expr()        {}
func (NullLit) expr()        {}
func (TemplateLit) expr()    {}
func (TaggedTemplate) expr() {}
func (ArrayExpr) expr()      {}
func (SpreadElement) expr()  {}
func (ObjectExpr) expr()     {}
func (Property) expr()       {}
func (SpreadProperty) expr() {}
func (ParenExpr) expr()      {}
func (FunctionExpr) expr()   {}
func (ClassExpr) expr()      {}
func (ArrowExpr) expr()      {}
func (UnaryExpr) expr()      {}
func (UpdateExpr) expr()     {}
func (BinaryExpr) expr()     {}
func (AssignExpr) expr()     {}
func (CondExpr) expr()       {}
func (SequenceExpr) expr()   {}
func (CallExpr) expr()       {}
func (NewExpr) expr()        {}
func (MemberExpr) expr()     {}
func (YieldExpr) expr()      {}
func (AwaitExpr) expr()      {}
func (ImportCall) expr()     {}
func (MetaProperty) expr()   {}

func (IdentExpr) pattern()      {}
func (ObjectPattern) pattern()  {}
func (ArrayPattern) pattern()   {}
func (RestElement) pattern()    {}
func (DefaultPattern) pattern() {}
