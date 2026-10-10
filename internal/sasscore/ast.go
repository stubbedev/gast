package sasscore

import (
	"github.com/stubbedev/gast/pkg/gast"
)

// Node is implemented by every AST node; Pos returns the node's span.
type Node interface {
	Pos() gast.Span
}

// Stylesheet is the root node: a parsed source file.
type Stylesheet struct {
	gast.Span
	Nodes []Node
}

// Block is a braced or indented body of nested nodes.
type Block struct {
	gast.Span
	Nodes []Node
}

// QualifiedRule is a selector prelude plus its declarations block.
type QualifiedRule struct {
	gast.Span
	Selectors *SelectorList
	Block     *Block
}

// Declaration is a property or custom-property declaration. Name is
// the property name tokens (a single identifier except for
// interpolated SCSS names); Value is the raw token stream of the value.
type Declaration struct {
	gast.Span
	Name      []gast.Token
	Custom    bool
	Value     []gast.Token
	Important bool
}

// AtRule is an at-rule without dedicated structure: @import, @charset,
// @media, and unknown at-rules in both their block and semicolon
// forms.
type AtRule struct {
	gast.Span
	Name    string // lowercase, without the @
	Prelude []gast.Token
	Block   *Block
}

// KeyframeRule is one `from, 50% { ... }` rule inside @keyframes.
type KeyframeRule struct {
	gast.Span
	Selectors []gast.Token
	Block     *Block
}

// Sass statements.

// VariableDecl is a $variable declaration with its !default and
// !global flags.
type VariableDecl struct {
	gast.Span
	Name    string
	Value   []gast.Token
	Default bool
	Global  bool
}

// Param is a mixin or function parameter with an optional default;
// Rest marks a trailing $args... parameter.
type Param struct {
	gast.Span
	Name    string
	Default []gast.Token
	Rest    bool
}

// MixinRule is an @mixin definition.
type MixinRule struct {
	gast.Span
	Name   string
	Params []Param
	Block  *Block
}

// IncludeRule is an @include invocation with an optional content
// block.
type IncludeRule struct {
	gast.Span
	Name  string
	Args  []gast.Token
	Block *Block
}

// FunctionRule is an @function definition.
type FunctionRule struct {
	gast.Span
	Name   string
	Params []Param
	Block  *Block
}

// ReturnRule is a @return statement.
type ReturnRule struct {
	gast.Span
	Value []gast.Token
}

// IfRule is @if with its else-if clauses and optional else block.
type IfRule struct {
	gast.Span
	Cond    []gast.Token
	Then    *Block
	ElseIfs []ElseIf
	Else    *Block
}

// ElseIf is one @else if clause.
type ElseIf struct {
	gast.Span
	Cond  []gast.Token
	Block *Block
}

// EachRule is @each; Rest marks `...` list destructuring.
type EachRule struct {
	gast.Span
	Vars  []string
	Rest  bool
	Expr  []gast.Token
	Block *Block
}

// ForRule is @for; Inclusive distinguishes through from to.
type ForRule struct {
	gast.Span
	Var       string
	From, To  []gast.Token
	Inclusive bool
	Block     *Block
}

// WhileRule is @while.
type WhileRule struct {
	gast.Span
	Cond  []gast.Token
	Block *Block
}

// ExtendRule is @extend with its selector token stream.
type ExtendRule struct {
	gast.Span
	Selector []gast.Token
}

// AtRootRule is @at-root with an optional query and block.
type AtRootRule struct {
	gast.Span
	Query []gast.Token
	Block *Block
}

// ConfigPair is one $name: value entry of a @use or @forward
// configuration.
type ConfigPair struct {
	gast.Span
	Name  string
	Value []gast.Token
}

// UseRule is @use.
type UseRule struct {
	gast.Span
	URL       gast.Token
	Namespace string
	Config    []ConfigPair
}

// ForwardRule is @forward.
type ForwardRule struct {
	gast.Span
	URL    gast.Token
	Prefix string
	Show   []string
	Hide   []string
	Config []ConfigPair
}
