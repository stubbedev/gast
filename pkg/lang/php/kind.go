// Package php implements a PHP frontend for gast: a lexer, a
// recursive-descent parser producing a spanned AST, and syntax
// validation. It targets PHP 8 syntax.
package php

// Token kinds. Every kind constant spells its token text exactly once;
// the keyword map, cast map, and lexer operator table are derived from
// these constants so the sets cannot drift apart.
const (
	KindEOF      = "eof"
	KindVariable = "variable" // $name, text includes the $
	KindIdent    = "ident"    // identifier; qualified names lex as ident \ ident

	KindOpenTag       = "open_tag"       // <?php
	KindOpenTagEcho   = "open_tag_echo"  // <?=
	KindCloseTag      = "close_tag"      // ?>
	KindInlineHTML    = "inline_html"    // everything outside php tags
	KindAttributeOpen = "attribute_open" // #[
	KindCast          = "cast"           // (int) and friends, text includes parens

	KindInt    = "int"
	KindFloat  = "float"
	KindString = "string" // single-quoted, double-quoted, heredoc, shell exec
)

// Punctuation and operator kinds; the constant value is the exact token
// text.
const (
	LParen           = "("
	RParen           = ")"
	LBrace           = "{"
	RBrace           = "}"
	LBracket         = "["
	RBracket         = "]"
	Semicolon        = ";"
	Colon            = ":"
	Comma            = ","
	Dollar           = "$"
	Backslash        = "\\"
	Ampersand        = "&"
	Pipe             = "|"
	Caret            = "^"
	Tilde            = "~"
	At               = "@"
	Question         = "?"
	Bang             = "!"
	Dot              = "."
	Plus             = "+"
	Minus            = "-"
	Asterisk         = "*"
	Slash            = "/"
	Percent          = "%"
	Lt               = "<"
	Gt               = ">"
	Assign           = "="
	Concat           = Dot
	Arrow            = Minus + Gt
	NullsafeArrow    = Question + Arrow
	DoubleColon      = Colon + Colon
	DoubleArrow      = Assign + Gt
	Ellipsis         = Dot + Dot + Dot
	Incr             = Plus + Plus
	Decr             = Minus + Minus
	ConcatAssign     = Concat + Assign
	Pow              = Asterisk + Asterisk
	AddAssign        = Plus + Assign
	SubAssign        = Minus + Assign
	MulAssign        = Asterisk + Assign
	DivAssign        = Slash + Assign
	ModAssign        = Percent + Assign
	BitAndAssign     = Ampersand + Assign
	BitOrAssign      = Pipe + Assign
	BitXorAssign     = Caret + Assign
	ShiftLeft        = Lt + Lt
	ShiftRight       = Gt + Gt
	LtEqual          = Lt + Assign
	GtEqual          = Gt + Assign
	ShiftLeftAssign  = ShiftLeft + Assign
	ShiftRightAssign = ShiftRight + Assign
	PowAssign        = Pow + Assign
	CoalesceAssign   = Question + Question + Assign
	Coalesce         = Question + Question
	Spaceship        = Lt + Assign + Gt
	Equal            = Assign + Assign
	Identical        = Equal + Assign
	NotEqual         = Bang + Assign
	NotIdentical     = NotEqual + Assign
	AngleNotEqual    = Lt + Gt
	LogicalAnd       = Ampersand + Ampersand
	LogicalOr        = Pipe + Pipe
	HeredocStart     = Lt + Lt + Lt
	OpenTagText      = Lt + Question + "php"
	OpenTagEchoText  = Lt + Question + Assign
	CloseTagText     = Question + Gt
)

// Keyword kinds, matched case-insensitively during lexing; the kind is
// the lowercase spelling.
const (
	KindAbstract    = "abstract"
	KindAnd         = "and"
	KindArray       = "array"
	KindAs          = "as"
	KindBreak       = "break"
	KindCallable    = "callable"
	KindCase        = "case"
	KindCatch       = "catch"
	KindClass       = "class"
	KindClone       = "clone"
	KindConst       = "const"
	KindContinue    = "continue"
	KindDeclare     = "declare"
	KindDefault     = "default"
	KindDo          = "do"
	KindEcho        = "echo"
	KindElse        = "else"
	KindElseIf      = "elseif"
	KindEmpty       = "empty"
	KindEndDeclare  = "enddeclare"
	KindEndFor      = "endfor"
	KindEndForeach  = "endforeach"
	KindEndIf       = "endif"
	KindEndSwitch   = "endswitch"
	KindEndWhile    = "endwhile"
	KindExtends     = "extends"
	KindFinal       = "final"
	KindFinally     = "finally"
	KindFn          = "fn"
	KindFor         = "for"
	KindForeach     = "foreach"
	KindFunction    = "function"
	KindGlobal      = "global"
	KindGoto        = "goto"
	KindIf          = "if"
	KindImplements  = "implements"
	KindInclude     = "include"
	KindIncludeOnce = "include_once"
	KindInstanceof  = "instanceof"
	KindInsteadOf   = "insteadof"
	KindInterface   = "interface"
	KindIsset       = "isset"
	KindList        = "list"
	KindMatch       = "match"
	KindNamespace   = "namespace"
	KindNew         = "new"
	KindOr          = "or"
	KindPrint       = "print"
	KindPrivate     = "private"
	KindProtected   = "protected"
	KindPublic      = "public"
	KindRequire     = "require"
	KindRequireOnce = "require_once"
	KindReturn      = "return"
	KindStatic      = "static"
	KindSwitch      = "switch"
	KindThrow       = "throw"
	KindTrait       = "trait"
	KindTry         = "try"
	KindUnset       = "unset"
	KindUse         = "use"
	KindVar         = "var"
	KindWhile       = "while"
	KindXor         = "xor"
	KindYield       = "yield"

	KindTrue  = "true"
	KindFalse = "false"
	KindNull  = "null"
)

// Contextual words: not reserved, so they lex as identifiers and are
// recognized by spelling in the positions that use them.
const (
	wordEnum     = "enum"
	wordReadonly = "readonly"
	wordFrom     = "from"
)

// keywords maps lowercase reserved spellings to their kind.
var keywords = buildKeywordMap()

func buildKeywordMap() map[string]string {
	keywordKinds := []string{
		KindAbstract, KindAnd, KindArray, KindAs, KindBreak, KindCallable,
		KindCase, KindCatch, KindClass, KindClone, KindConst, KindContinue,
		KindDeclare, KindDefault, KindDo, KindEcho, KindElse, KindElseIf,
		KindEmpty, KindEndDeclare, KindEndFor, KindEndForeach, KindEndIf,
		KindEndSwitch, KindEndWhile, KindExtends, KindFinal, KindFinally,
		KindFn, KindFor, KindForeach, KindFunction, KindGlobal, KindGoto,
		KindIf, KindImplements, KindInclude, KindIncludeOnce, KindInstanceof,
		KindInsteadOf, KindInterface, KindIsset, KindList, KindMatch,
		KindNamespace, KindNew, KindOr, KindPrint, KindPrivate, KindProtected,
		KindPublic, KindRequire, KindRequireOnce, KindReturn, KindStatic,
		KindSwitch, KindThrow, KindTrait, KindTry, KindUnset, KindUse,
		KindVar, KindWhile, KindXor, KindYield, KindTrue, KindFalse, KindNull,
	}
	m := make(map[string]string, len(keywordKinds))
	for _, kind := range keywordKinds {
		m[kind] = kind
	}
	return m
}

// Canonical cast names.
const (
	CastInt    = "int"
	CastBool   = "bool"
	CastFloat  = "float"
	CastString = "string"
	CastArray  = "array"
	CastObject = "object"
	CastUnset  = "unset"
)

// casts maps each accepted cast spelling, lowercased, to the canonical
// name.
var casts = map[string]string{
	"int":     CastInt,
	"integer": CastInt,
	"bool":    CastBool,
	"boolean": CastBool,
	"float":   CastFloat,
	"double":  CastFloat,
	"real":    CastFloat,
	"string":  CastString,
	"binary":  CastString,
	"array":   CastArray,
	"object":  CastObject,
	"unset":   CastUnset,
}
