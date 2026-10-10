// Package sasscore implements the lexer, parser, and AST shared by the
// CSS, SCSS, and SASS frontends. CSS Syntax Level 3 is the base
// dialect; SCSS adds Sass tokens and constructs, and the indented SASS
// dialect layers newline and indentation sensitivity on top of the
// same scanner.
package sasscore

// Token kinds for the named tokens. Punctuation and delimiter kinds
// spell their exact token text, following the gast convention, so a
// token's kind and text never drift apart.
const (
	KindEOF          = "eof"
	KindIdent        = "ident"      // identifier, including --custom-property names
	KindFunction     = "function"   // name( — text includes the open parenthesis
	KindAtKeyword    = "at_keyword" // @name — text includes the @
	KindHash         = "hash"       // #123 — unrestricted hash token
	KindHashID       = "id_hash"    // #name — hash that is a valid identifier
	KindString       = "string"     // quoted string — text includes the quotes
	KindURL          = "url"        // url(...) — one token, raw or quoted
	KindNumber       = "number"
	KindPercentage   = "percentage"    // text includes the %
	KindDimension    = "dimension"     // number plus unit
	KindUnicodeRange = "unicode_range" // U+4E00-9FFF, U+3??
	KindCDO          = "cdo"           // <!--
	KindCDC          = "cdc"           // -->
	KindVariable     = "variable"      // $name — SCSS and SASS only
	KindInterpOpen   = "interp_open"   // #{ — SCSS and SASS only
	KindNewline      = "newline"       // indented dialect only
	KindIndent       = "indent"        // indented dialect only
	KindDedent       = "dedent"        // indented dialect only
)

// Punctuation kinds; the constant value is the exact token text.
const (
	LParen    = "("
	RParen    = ")"
	LBrace    = "{"
	RBrace    = "}"
	LBracket  = "["
	RBracket  = "]"
	Semicolon = ";"
	Colon     = ":"
	Comma     = ","
	Dot       = "."
	Plus      = "+"
	Minus     = "-"
	Asterisk  = "*"
	Slash     = "/"
	Caret     = "^"
	Dollar    = "$"
	Percent   = "%"
	Pipe      = "|"
	Tilde     = "~"
	Equals    = "="
	Lt        = "<"
	Gt        = ">"
	Bang      = "!"
	Ampersand = "&"
	HashMark  = "#"
	At        = "@"
)

// Token spellings assembled from the punctuation constants.
const (
	CDOText         = Lt + Bang + Minus + Minus
	CDCText         = Minus + Minus + Gt
	InterpOpenText  = HashMark + LBrace
	CustomPropStart = Minus + Minus
)

// Descendant is the combinator joining two compound selectors by
// whitespace.
const Descendant = " "

// Dialect selects the feature set of a frontend.
type Dialect struct {
	SCSS     bool // Sass tokens: $variables, #{} interpolation, // comments
	Indented bool // newline- and indentation-sensitive SASS syntax
}

// Dialects of the three frontends.
var (
	DialectCSS  = Dialect{}
	DialectSCSS = Dialect{SCSS: true}
	DialectSass = Dialect{SCSS: true, Indented: true}
)

// blockAtRules are at-rule names that must carry a block.
var blockAtRules = map[string]bool{
	"media":     true,
	"supports":  true,
	"container": true,
	"keyframes": true,
	"page":      true,
	"font-face": true,
}

// preludeAtRules are block at-rules whose prelude may not be empty.
var preludeAtRules = map[string]bool{
	"media":     true,
	"supports":  true,
	"container": true,
}

// keyframesAtRules are at-rule names whose blocks hold keyframe rules
// instead of declarations.
var keyframesAtRules = map[string]bool{
	"keyframes":         true,
	"-moz-keyframes":    true,
	"-ms-keyframes":     true,
	"-o-keyframes":      true,
	"-webkit-keyframes": true,
}

// selectorPseudos are pseudo-class functions whose arguments are
// selector lists rather than plain values.
var selectorPseudos = map[string]bool{
	"is":      true,
	"not":     true,
	"has":     true,
	"where":   true,
	"matches": true,
	"any":     true,
	"current": true,
	"past":    true,
	"future":  true,
}
