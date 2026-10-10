package ts

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

// TypeNode is a node that can appear in a type position.
type TypeNode interface {
	Node
	typeNode()
}

// File is the root node of a parsed source file.
type File struct {
	gast.Span
	Stmts []Stmt
}

// Decorator is one @expression on a class, member, parameter, or
// accessor; both legacy and ECMA decorator forms are accepted
// syntactically.
type Decorator struct {
	gast.Span
	Expr Expr
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
// "let", "const". Declare marks an ambient declaration.
type VarStmt struct {
	gast.Span
	Kind    string
	Declare bool
	Decls   []*VarDeclarator
}

// VarDeclarator is one binding=initializer pair of a VarStmt; Type is
// the optional annotation, Definite the `!` definite-assignment marker,
// and Init nil for bare declarations.
type VarDeclarator struct {
	gast.Span
	Binding  Pattern
	Type     TypeNode
	Definite bool
	Init     Expr
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
// for default.
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
	Type  TypeNode
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
// export default. Body is nil for overloads and ambient declarations,
// and ReturnType carries the annotation of the declaration.
type FunctionDecl struct {
	gast.Span
	Async      bool
	Generator  bool
	Declare    bool
	Name       string
	TypeParams []*TypeParam
	Params     []*Param
	ReturnType TypeNode
	Body       *BlockStmt
}

// ClassDecl is a class declaration, including TypeScript's abstract
// classes, type parameters, implements clause, and decorators.
type ClassDecl struct {
	gast.Span
	Decorators []Decorator
	Abstract   bool
	Declare    bool
	Name       string
	TypeParams []*TypeParam
	Extends    Expr
	Implements []TypeNode
	Body       []ClassMember
}

// InterfaceDecl is an interface declaration; Extends holds the heritage
// list. Declare marks an ambient declaration.
type InterfaceDecl struct {
	gast.Span
	Declare    bool
	Name       string
	TypeParams []*TypeParam
	Extends    []TypeNode
	Body       []TypeMember
}

// TypeAliasDecl is a `type Name<Params> = Type` declaration.
type TypeAliasDecl struct {
	gast.Span
	Declare    bool
	Name       string
	TypeParams []*TypeParam
	Type       TypeNode
}

// EnumDecl is a (possibly const) enum declaration.
type EnumDecl struct {
	gast.Span
	Const   bool
	Declare bool
	Name    string
	Members []*EnumMember
}

// EnumMember is one enum member; Init is nil for auto-incremented
// members.
type EnumMember struct {
	gast.Span
	Name Expr
	Init Expr
}

// NamespaceDecl is a namespace or module declaration; Name may be a
// dotted path and Body holds declarations (nested namespaces flatten
// into statement lists).
type NamespaceDecl struct {
	gast.Span
	Declare bool
	Name    string
	Body    []Stmt
}

// ImportDecl is an import declaration; TypeOnly marks `import type`.
// Default and Namespace hold the local names they bind, "" when absent.
type ImportDecl struct {
	gast.Span
	TypeOnly  bool
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

// ImportEqualsDecl is `import Name = require(source)` or
// `import Name = A.B.C`.
type ImportEqualsDecl struct {
	gast.Span
	TypeOnly bool
	Name     string
	Require  bool
	Source   string
	Target   Expr
}

// ExportNamedStmt is an export of local declarations or an export list;
// Decl is nil for the list form, which may re-export from Source.
// TypeOnly marks `export type`.
type ExportNamedStmt struct {
	gast.Span
	TypeOnly bool
	Decl     Stmt
	Specs    []ExportSpec
	Source   string
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

// ExportAssignStmt is the ambient `export = expression;` form.
type ExportAssignStmt struct {
	gast.Span
	Expr Expr
}

// Class members.

// ClassMember is a member of a class body.
type ClassMember interface {
	Node
	classMember()
}

// MethodDef is a method, constructor, getter, or setter; Kind is one of
// "method", "constructor", "get", "set". Accessibility is "public",
// "private", "protected", or "" and Body is nil for abstract,
// ambient, and overload signatures.
type MethodDef struct {
	gast.Span
	Decorators    []Decorator
	Key           Expr
	Static        bool
	Async         bool
	Generator     bool
	Abstract      bool
	Optional      bool
	Accessibility string
	Kind          string
	TypeParams    []*TypeParam
	Params        []*Param
	ReturnType    TypeNode
	Body          *BlockStmt
}

// PropertyDef is a field declaration; Init is nil for bare fields.
type PropertyDef struct {
	gast.Span
	Decorators    []Decorator
	Key           Expr
	Init          Expr
	Type          TypeNode
	Static        bool
	Computed      bool
	Declare       bool
	Abstract      bool
	Definite      bool
	Optional      bool
	Readonly      bool
	Accessibility string
}

// ClassIndexSig is an index signature member of a class body.
type ClassIndexSig struct {
	gast.Span
	Readonly bool
	KeyName  string
	KeyType  TypeNode
	Type     TypeNode
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

// StringLit is a string literal; Text is the raw spelling.
type StringLit struct {
	gast.Span
	Text string
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
	Key        Expr
	Value      Expr
	Default    Expr
	Kind       string
	Computed   bool
	Shorthand  bool
	Optional   bool
	TypeParams []*TypeParam
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
	Async      bool
	Generator  bool
	Name       string
	TypeParams []*TypeParam
	Params     []*Param
	ReturnType TypeNode
	Body       *BlockStmt
}

// ClassExpr is a class expression; Name is empty for anonymous classes.
type ClassExpr struct {
	gast.Span
	Decorators []Decorator
	Abstract   bool
	Name       string
	TypeParams []*TypeParam
	Extends    Expr
	Implements []TypeNode
	Body       []ClassMember
}

// ArrowExpr is an arrow function; Body is a *BlockStmt or an
// expression, distinguished by Expression. TypeParams carries
// TypeScript's generic arrows.
type ArrowExpr struct {
	gast.Span
	Async      bool
	TypeParams []*TypeParam
	Params     []*Param
	ReturnType TypeNode
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

// CallExpr is callee(args); Optional marks a ?.() call and TypeArgs
// carries an explicit generic instantiation `callee<T>(args)`.
type CallExpr struct {
	gast.Span
	Callee   Expr
	Args     []Expr
	TypeArgs []TypeNode
	Optional bool
}

// NewExpr is new callee(args); Args is nil when no argument list
// follows the callee, and TypeArgs carries `new C<T>(args)`.
type NewExpr struct {
	gast.Span
	Callee   Expr
	Args     []Expr
	TypeArgs []TypeNode
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

// TypeAssertionExpr is `expr as type` (ConstForm marks `as const`) and
// `expr satisfies type` is *SatisfiesExpr; both are produced at the
// conditional precedence level.
type TypeAssertionExpr struct {
	gast.Span
	Expr      Expr
	Type      TypeNode
	ConstForm bool
}

// AngleAssertionExpr is the legacy `<type>expr` assertion.
type AngleAssertionExpr struct {
	gast.Span
	Expr Expr
	Type TypeNode
}

// SatisfiesExpr is `expr satisfies type`.
type SatisfiesExpr struct {
	gast.Span
	Expr Expr
	Type TypeNode
}

// NonNullExpr is the `expr!` non-null assertion.
type NonNullExpr struct {
	gast.Span
	Expr Expr
}

// InstantiationExpr is `expr<T>` without a call, TypeScript's generic
// instantiation expression.
type InstantiationExpr struct {
	gast.Span
	Expr     Expr
	TypeArgs []TypeNode
}

// JSX expressions (TSX mode).

// JSXChild is a node that can appear inside a JSX element: raw text, a
// nested element, or an expression container.
type JSXChild interface {
	Node
	jsxChild()
}

// JSXElement is a JSX element `<name attrs>children</name>`, a
// self-closing `<name attrs />`, or a fragment `<>children</>`
// (Fragment set, Name nil). Text is kept raw: JSX entities such as
// &amp; are not decoded.
type JSXElement struct {
	gast.Span
	Fragment    bool
	Name        *JSXName
	Attrs       []*JSXAttr
	Children    []JSXChild
	SelfClosing bool
}

// JSXName is a JSX element or attribute name: a plain name (`div`), a
// member chain (`A.B.C`, Namespace false), or a namespaced name
// (`svg:path`, Namespace true).
type JSXName struct {
	gast.Span
	Parts     []string
	Namespace bool
}

// JSXAttr is one JSX attribute: `name`, `name="v"`, `name={expr}`, or
// a spread `{...expr}` (Spread set, Name invalid). Value is nil for
// shorthand boolean attributes; string values keep their raw quoted
// text and entities are not decoded.
type JSXAttr struct {
	gast.Span
	Name   JSXName
	Value  Expr
	Spread Expr
}

// JSXText is raw text between children; Text is the undecoded source
// slice, so raw '<' and '>' appear literally and entities stay raw.
type JSXText struct {
	gast.Span
	Text string
}

// JSXExprContainer is a `{expr}` child or attribute value; Expr is nil
// only for the empty `{}` child container, which is legal so JSX
// comments `{/* ... */}` have somewhere to live.
type JSXExprContainer struct {
	gast.Span
	Expr Expr
}

// Shared nodes.

// Param is one function parameter: a binding with an optional type
// annotation, default, and rest marker. TypeScript additions are the
// optional `?`, the `this` parameter, parameter-property modifiers
// (Accessibility and Readonly), and decorators.
type Param struct {
	gast.Span
	Decorators    []Decorator
	Binding       Pattern
	Type          TypeNode
	Default       Expr
	Accessibility string
	Readonly      bool
	Optional      bool
	Rest          bool
	This          bool
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

// Types.

// TypeRef is a (possibly qualified) type name with optional type
// arguments, e.g. `string`, `A.B.C<T>`, `any`, or `this`.
type TypeRef struct {
	gast.Span
	Parts    []string
	TypeArgs []TypeNode
}

// LiteralType is a string, number, bigint, or boolean literal type;
// Text is the raw literal spelling and Neg marks a negative number.
type LiteralType struct {
	gast.Span
	Text string
	Neg  bool
}

// ArrayType is elem[], optionally readonly.
type ArrayType struct {
	gast.Span
	Elem     TypeNode
	Readonly bool
}

// TupleType is a tuple type `[A, B?, ...C]`, optionally readonly, whose
// members may be named.
type TupleType struct {
	gast.Span
	Readonly bool
	Elems    []*TupleElement
}

// TupleElement is one tuple member; Name is empty when the element is
// unnamed.
type TupleElement struct {
	gast.Span
	Name     string
	Type     TypeNode
	Optional bool
	Rest     bool
}

// FunctionType is a function type `(params) => ret`, a constructor
// type `new (params) => ret`, or `abstract new (params) => ret`.
type FunctionType struct {
	gast.Span
	Abstract   bool
	New        bool
	TypeParams []*TypeParam
	Params     []*Param
	ReturnType TypeNode
}

// ObjectType is an object type literal `{ members }`.
type ObjectType struct {
	gast.Span
	Members []TypeMember
}

// TypeMember is a member of an object type or interface body.
type TypeMember interface {
	Node
	typeMember()
}

// PropertySig is a property member `key [?]: type`.
type PropertySig struct {
	gast.Span
	Key      Expr
	Optional bool
	Readonly bool
	Type     TypeNode
}

// MethodSig is a method member `key [?]<T>(params): ret`.
type MethodSig struct {
	gast.Span
	Key        Expr
	Optional   bool
	Readonly   bool
	TypeParams []*TypeParam
	Params     []*Param
	ReturnType TypeNode
}

// CallSig is a call signature `<T>(params): ret`.
type CallSig struct {
	gast.Span
	TypeParams []*TypeParam
	Params     []*Param
	ReturnType TypeNode
}

// ConstructSig is a construct signature `new <T>(params): ret`, with
// Abstract for `abstract new`.
type ConstructSig struct {
	gast.Span
	Abstract   bool
	TypeParams []*TypeParam
	Params     []*Param
	ReturnType TypeNode
}

// GetSig is a getter signature `get key(): type`.
type GetSig struct {
	gast.Span
	Key  Expr
	Type TypeNode
}

// SetSig is a setter signature `set key(value: type)`.
type SetSig struct {
	gast.Span
	Key   Expr
	Param *Param
}

// IndexSig is an index signature `[key: KeyType]: type`, optionally
// readonly, in an object type or interface body.
type IndexSig struct {
	gast.Span
	Readonly bool
	KeyName  string
	KeyType  TypeNode
	Type     TypeNode
}

// UnionType is `a | b | ...`; IntersectionType is `a & b & ...`.
type UnionType struct {
	gast.Span
	Types []TypeNode
}

// IntersectionType is `a & b & ...`.
type IntersectionType struct {
	gast.Span
	Types []TypeNode
}

// TypeOperator is a type operator: keyof, unique, or readonly
// (including typeof queries, whose operand is a TypeRef).
type TypeOperator struct {
	gast.Span
	Op   string
	Type TypeNode
}

// IndexedAccessType is obj[index].
type IndexedAccessType struct {
	gast.Span
	Obj   TypeNode
	Index TypeNode
}

// ConditionalType is `check extends ext ? yes : no`.
type ConditionalType struct {
	gast.Span
	Check   TypeNode
	Extends TypeNode
	True    TypeNode
	False   TypeNode
}

// InferType is `infer name` with an optional `extends` constraint.
type InferType struct {
	gast.Span
	Name       string
	Constraint TypeNode
}

// MappedType is `{ [key in source as alias]mod value }`. ReadonlyMod is
// "", "readonly", "+readonly", or "-readonly" and OptionalMod "", "?",
// "+?", or "-?".
type MappedType struct {
	gast.Span
	ReadonlyMod string
	KeyName     string
	In          TypeNode
	As          TypeNode
	OptionalMod string
	Value       TypeNode
}

// TemplateLiteralType is a template literal type whose substitutions
// are types; Quasis follows the TemplateLit layout.
type TemplateLiteralType struct {
	gast.Span
	Quasis []string
	Types  []TypeNode
}

// ParenType is a parenthesized type.
type ParenType struct {
	gast.Span
	Type TypeNode
}

// ImportType is `import("./m").A<T>`.
type ImportType struct {
	gast.Span
	Source   string
	Parts    []string
	TypeArgs []TypeNode
}

// TypePredicate is a return-type predicate `name is type` or
// `asserts name is type`; the name may be "this".
type TypePredicate struct {
	gast.Span
	Name    string
	Asserts bool
	Type    TypeNode
}

// TypeParam is one type parameter `<const T in out extends X = D>`;
// Const, In, and Out are the variance and const modifiers.
type TypeParam struct {
	gast.Span
	Name       string
	Const      bool
	In         bool
	Out        bool
	Constraint TypeNode
	Default    TypeNode
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
func (InterfaceDecl) stmt()     {}
func (TypeAliasDecl) stmt()     {}
func (EnumDecl) stmt()          {}
func (NamespaceDecl) stmt()     {}
func (ImportDecl) stmt()        {}
func (ImportEqualsDecl) stmt()  {}
func (ExportNamedStmt) stmt()   {}
func (ExportAllStmt) stmt()     {}
func (ExportDefaultStmt) stmt() {}
func (ExportAssignStmt) stmt()  {}

func (MethodDef) classMember()     {}
func (PropertyDef) classMember()   {}
func (ClassIndexSig) classMember() {}
func (StaticBlock) classMember()   {}

func (IdentExpr) expr()            {}
func (BlockStmt) expr()            {} // arrow function block bodies
func (PrivateName) expr()          {}
func (ThisExpr) expr()             {}
func (SuperExpr) expr()            {}
func (NumberLit) expr()            {}
func (BigIntLit) expr()            {}
func (StringLit) expr()            {}
func (RegexLit) expr()             {}
func (BoolLit) expr()              {}
func (NullLit) expr()              {}
func (TemplateLit) expr()          {}
func (TaggedTemplate) expr()       {}
func (ArrayExpr) expr()            {}
func (SpreadElement) expr()        {}
func (ObjectExpr) expr()           {}
func (Property) expr()             {}
func (SpreadProperty) expr()       {}
func (ParenExpr) expr()            {}
func (FunctionExpr) expr()         {}
func (ClassExpr) expr()            {}
func (ArrowExpr) expr()            {}
func (UnaryExpr) expr()            {}
func (UpdateExpr) expr()           {}
func (BinaryExpr) expr()           {}
func (AssignExpr) expr()           {}
func (CondExpr) expr()             {}
func (SequenceExpr) expr()         {}
func (CallExpr) expr()             {}
func (NewExpr) expr()              {}
func (MemberExpr) expr()           {}
func (YieldExpr) expr()            {}
func (AwaitExpr) expr()            {}
func (ImportCall) expr()           {}
func (MetaProperty) expr()         {}
func (TypeAssertionExpr) expr()    {}
func (AngleAssertionExpr) expr()   {}
func (SatisfiesExpr) expr()        {}
func (NonNullExpr) expr()          {}
func (InstantiationExpr) expr()    {}
func (JSXElement) expr()           {}
func (JSXExprContainer) expr()     {}
func (JSXElement) jsxChild()       {}
func (JSXText) jsxChild()          {}
func (JSXExprContainer) jsxChild() {}

func (IdentExpr) pattern()      {}
func (ThisExpr) pattern()       {}
func (ObjectPattern) pattern()  {}
func (ArrayPattern) pattern()   {}
func (RestElement) pattern()    {}
func (DefaultPattern) pattern() {}

func (TypeRef) typeNode()             {}
func (LiteralType) typeNode()         {}
func (ArrayType) typeNode()           {}
func (TupleType) typeNode()           {}
func (FunctionType) typeNode()        {}
func (ObjectType) typeNode()          {}
func (UnionType) typeNode()           {}
func (IntersectionType) typeNode()    {}
func (TypeOperator) typeNode()        {}
func (IndexedAccessType) typeNode()   {}
func (ConditionalType) typeNode()     {}
func (InferType) typeNode()           {}
func (MappedType) typeNode()          {}
func (TemplateLiteralType) typeNode() {}
func (ParenType) typeNode()           {}
func (ImportType) typeNode()          {}
func (TypePredicate) typeNode()       {}

func (PropertySig) typeMember()  {}
func (MethodSig) typeMember()    {}
func (CallSig) typeMember()      {}
func (ConstructSig) typeMember() {}
func (GetSig) typeMember()       {}
func (SetSig) typeMember()       {}
func (IndexSig) typeMember()     {}
