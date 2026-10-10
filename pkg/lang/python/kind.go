// Package python implements a Python 3.12/3.13 frontend for gast: a
// lexer with INDENT/DEDENT and PEP 701 f-string tokens, a
// recursive-descent parser producing a spanned AST, and syntax
// validation.
package python

// Token kinds for tokens whose text is not fixed punctuation: names,
// literals, indentation, and the f-string token family.
const (
	KindEOF           = "eof"
	KindName          = "name"
	KindInt           = "int"
	KindFloat         = "float"
	KindComplex       = "complex"
	KindString        = "string" // includes prefix characters and quotes
	KindBytes         = "bytes"  // includes prefix characters and quotes
	KindFStringStart  = "fstring_start"
	KindFStringMiddle = "fstring_middle"
	KindFStringEnd    = "fstring_end"
	KindConversion    = "conversion" // !s, !r, !a inside an f-string field

	KindIndent  = "indent"
	KindDedent  = "dedent"
	KindNewline = "newline"
)

// Punctuation and operator kinds; the constant value is the exact token
// text.
const (
	LParen           = "("
	RParen           = ")"
	LBracket         = "["
	RBracket         = "]"
	LBrace           = "{"
	RBrace           = "}"
	Comma            = ","
	Colon            = ":"
	Dot              = "."
	Semicolon        = ";"
	Assign           = "="
	Arrow            = "->"
	Walrus           = ":="
	Ellipsis         = "..."
	Plus             = "+"
	Minus            = "-"
	Star             = "*"
	DoubleStar       = "**"
	Slash            = "/"
	DoubleSlash      = "//"
	Percent          = "%"
	At               = "@"
	Ampersand        = "&"
	Pipe             = "|"
	Caret            = "^"
	Tilde            = "~"
	LeftShift        = "<<"
	RightShift       = ">>"
	Less             = "<"
	Greater          = ">"
	LessEqual        = "<="
	GreaterEqual     = ">="
	EqualEqual       = "=="
	NotEqual         = "!="
	AddAssign        = "+="
	SubAssign        = "-="
	MulAssign        = "*="
	DivAssign        = "/="
	FloorDivAssign   = "//="
	ModAssign        = "%="
	MatMulAssign     = "@="
	BitAndAssign     = "&="
	BitOrAssign      = "|="
	BitXorAssign     = "^="
	LeftShiftAssign  = "<<="
	RightShiftAssign = ">>="
	PowAssign        = "**="
)

// Keyword kinds, matched case-sensitively during lexing; the kind is
// the exact spelling.
const (
	KindFalse    = "False"
	KindNone     = "None"
	KindTrue     = "True"
	KindAnd      = "and"
	KindAs       = "as"
	KindAssert   = "assert"
	KindAsync    = "async"
	KindAwait    = "await"
	KindBreak    = "break"
	KindClass    = "class"
	KindContinue = "continue"
	KindDef      = "def"
	KindDel      = "del"
	KindElif     = "elif"
	KindElse     = "else"
	KindExcept   = "except"
	KindFinally  = "finally"
	KindFor      = "for"
	KindFrom     = "from"
	KindGlobal   = "global"
	KindIf       = "if"
	KindImport   = "import"
	KindIn       = "in"
	KindIs       = "is"
	KindLambda   = "lambda"
	KindNonlocal = "nonlocal"
	KindNot      = "not"
	KindOr       = "or"
	KindPass     = "pass"
	KindRaise    = "raise"
	KindReturn   = "return"
	KindTry      = "try"
	KindWhile    = "while"
	KindWith     = "with"
	KindYield    = "yield"
)

// Soft keywords: not reserved, so they lex as names and are recognized
// by spelling in the positions that use them.
const (
	wordMatch = "match"
	wordCase  = "case"
	wordType  = "type"
	wildcard  = "_"
)

// keywords maps reserved spellings to their kind.
var keywords = buildKeywordMap()

func buildKeywordMap() map[string]string {
	keywordKinds := []string{
		KindFalse, KindNone, KindTrue, KindAnd, KindAs, KindAssert,
		KindAsync, KindAwait, KindBreak, KindClass, KindContinue, KindDef,
		KindDel, KindElif, KindElse, KindExcept, KindFinally, KindFor,
		KindFrom, KindGlobal, KindIf, KindImport, KindIn, KindIs,
		KindLambda, KindNonlocal, KindNot, KindOr, KindPass, KindRaise,
		KindReturn, KindTry, KindWhile, KindWith, KindYield,
	}
	m := make(map[string]string, len(keywordKinds))
	for _, kind := range keywordKinds {
		m[kind] = kind
	}
	return m
}

// comparison operator spellings that span multiple tokens.
const (
	opIn    = "in"
	opNotIn = "not in"
	opIs    = "is"
	opIsNot = "is not"
)
