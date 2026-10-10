// Package js implements a JavaScript (ES2024) frontend for gast: a
// lexer, a recursive-descent parser producing a spanned AST, and
// syntax validation. It targets script and module syntax.
package js

// Token kinds. Every kind constant spells its token text exactly once;
// the keyword map and the lexer operator table are derived from these
// constants so the sets cannot drift apart.
const (
	KindEOF            = "eof"
	KindIdent          = "ident"           // identifier, including contextual keywords
	KindPrivateName    = "private_name"    // #name, text includes the #
	KindNum            = "number"          // decimal, hex, octal, and binary literals
	KindBigInt         = "bigint"          // integer literal with the n suffix
	KindString         = "string"          // single- or double-quoted
	KindRegex          = "regex"           // /pattern/flags, text is the raw spelling
	KindHashbang       = "hashbang"        // #! comment at the very start of input
	KindNoSubTemplate  = "no_sub"          // `text` without substitutions
	KindTemplateHead   = "template_head"   // `text${
	KindTemplateMiddle = "template_middle" // }text${
	KindTemplateTail   = "template_tail"   // }text`
)

// Punctuation and operator kinds; the constant value is the exact token
// text.
const (
	LParen              = "("
	RParen              = ")"
	LBrace              = "{"
	RBrace              = "}"
	LBracket            = "["
	RBracket            = "]"
	Semicolon           = ";"
	Colon               = ":"
	Comma               = ","
	Dot                 = "."
	Ellipsis            = "..."
	Question            = "?"
	OptChain            = "?."
	Bang                = "!"
	Tilde               = "~"
	Plus                = "+"
	Minus               = "-"
	Asterisk            = "*"
	Slash               = "/"
	Percent             = "%"
	At                  = "@" // decorator introducer (stage-3 decorators)
	Lt                  = "<"
	Gt                  = ">"
	Assign              = "="
	Ampersand           = "&"
	Pipe                = "|"
	Caret               = "^"
	Arrow               = "=>"
	Incr                = "++"
	Decr                = "--"
	Pow                 = "**"
	Nullish             = "??"
	LogicalAnd          = "&&"
	LogicalOr           = "||"
	Equal               = "=="
	StrictEqual         = "==="
	NotEqual            = "!="
	StrictNotEqual      = "!=="
	LtEqual             = "<="
	GtEqual             = ">="
	ShiftLeft           = "<<"
	ShiftRight          = ">>"
	UnsignedShift       = ">>>"
	ShiftLeftAssign     = "<<="
	ShiftRightAssign    = ">>="
	UnsignedShiftAssign = ">>>="
	AddAssign           = "+="
	SubAssign           = "-="
	MulAssign           = "*="
	DivAssign           = "/="
	ModAssign           = "%="
	BitAndAssign        = "&="
	BitOrAssign         = "|="
	BitXorAssign        = "^="
	PowAssign           = "**="
	LogicalAndAssign    = "&&="
	LogicalOrAssign     = "||="
	NullishAssign       = "??="
)

// Keyword kinds; the kind is the exact spelling, matched
// case-sensitively during lexing. Contextual keywords (let, static,
// async, await, yield, of, get, set, from, as) are not reserved and
// lex as identifiers, recognized by spelling at the positions that use
// them.
const (
	KindBreak      = "break"
	KindCase       = "case"
	KindCatch      = "catch"
	KindClass      = "class"
	KindConst      = "const"
	KindContinue   = "continue"
	KindDebugger   = "debugger"
	KindDefault    = "default"
	KindDelete     = "delete"
	KindDo         = "do"
	KindElse       = "else"
	KindExport     = "export"
	KindExtends    = "extends"
	KindFinally    = "finally"
	KindFor        = "for"
	KindFunction   = "function"
	KindIf         = "if"
	KindImport     = "import"
	KindIn         = "in"
	KindInstanceof = "instanceof"
	KindNew        = "new"
	KindReturn     = "return"
	KindSuper      = "super"
	KindSwitch     = "switch"
	KindThis       = "this"
	KindThrow      = "throw"
	KindTry        = "try"
	KindTypeof     = "typeof"
	KindVar        = "var"
	KindVoid       = "void"
	KindWhile      = "while"
	KindWith       = "with"

	KindTrue  = "true"
	KindFalse = "false"
	KindNull  = "null"
)

// keywords maps reserved spellings to their kind.
var keywords = buildKeywordMap()

func buildKeywordMap() map[string]string {
	keywordKinds := []string{
		KindBreak, KindCase, KindCatch, KindClass, KindConst, KindContinue,
		KindDebugger, KindDefault, KindDelete, KindDo, KindElse, KindExport,
		KindExtends, KindFinally, KindFor, KindFunction, KindIf, KindImport,
		KindIn, KindInstanceof, KindNew, KindReturn, KindSuper, KindSwitch,
		KindThis, KindThrow, KindTry, KindTypeof, KindVar, KindVoid, KindWhile,
		KindWith, KindTrue, KindFalse, KindNull,
	}
	m := make(map[string]string, len(keywordKinds))
	for _, kind := range keywordKinds {
		m[kind] = kind
	}
	return m
}

// endsExpression reports whether a token of the given kind may be the
// last token of a complete expression, which decides the regex-versus-
// division and await-identifier disambiguation: after such a token a
// `/` is division and `await` is a plain identifier; anywhere else a
// `/` starts a regex literal and `await` is a unary operator.
func endsExpression(kind, text string) bool {
	switch kind {
	case KindIdent, KindPrivateName, KindNum, KindBigInt, KindString,
		KindRegex, KindTemplateTail, KindNoSubTemplate, RParen, RBracket,
		Incr, Decr, KindTrue, KindFalse, KindNull, KindThis, KindSuper:
		return kind != KindIdent || text != "of"
	}
	return false
}
