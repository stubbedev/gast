// Package ts implements a TypeScript 5.x frontend for gast: a lexer,
// a recursive-descent parser producing a spanned AST, and syntax
// validation. Parsing comes in two modes: plain TypeScript
// (Parse/Validate), where the ambiguous '<' is resolved by bounded
// speculative parsing into a generic arrow, a legacy angle-bracket
// assertion, or a comparison, and TSX (ParseTSX/ValidateTSX), where
// JSX elements and fragments are expressions and angle-bracket
// assertions are illegal, matching tsc.
//
// Lexing reuses the JavaScript frontend: TypeScript shares the
// JavaScript lexical grammar, so pkg/lang/js tokenizes TypeScript
// sources directly. The parser is self-contained because TypeScript's
// grammar threads types through every binding, member, and call site,
// which the JavaScript parser's AST cannot express.
package ts

import (
	"github.com/stubbedev/gast/pkg/gast"
	"github.com/stubbedev/gast/pkg/lang/js"
)

// Token kinds, re-exported from the JavaScript lexer so the sets cannot
// drift apart. TypeScript's additional keywords (type, interface, enum,
// namespace, declare, abstract, readonly, public, private, protected,
// implements, satisfies, as, is, keyof, infer, asserts, unique, const
// type parameters, ...) are contextual: they lex as identifiers and are
// recognized by spelling at the positions that use them.
const (
	KindEOF            = js.KindEOF
	KindIdent          = js.KindIdent
	KindPrivateName    = js.KindPrivateName
	KindNum            = js.KindNum
	KindBigInt         = js.KindBigInt
	KindString         = js.KindString
	KindRegex          = js.KindRegex
	KindHashbang       = js.KindHashbang
	KindNoSubTemplate  = js.KindNoSubTemplate
	KindTemplateHead   = js.KindTemplateHead
	KindTemplateMiddle = js.KindTemplateMiddle
	KindTemplateTail   = js.KindTemplateTail

	LParen              = js.LParen
	RParen              = js.RParen
	LBrace              = js.LBrace
	RBrace              = js.RBrace
	LBracket            = js.LBracket
	RBracket            = js.RBracket
	Semicolon           = js.Semicolon
	Colon               = js.Colon
	Comma               = js.Comma
	Dot                 = js.Dot
	Ellipsis            = js.Ellipsis
	Question            = js.Question
	OptChain            = js.OptChain
	Bang                = js.Bang
	Tilde               = js.Tilde
	Plus                = js.Plus
	Minus               = js.Minus
	Asterisk            = js.Asterisk
	Slash               = js.Slash
	Percent             = js.Percent
	At                  = js.At
	Lt                  = js.Lt
	Gt                  = js.Gt
	Assign              = js.Assign
	Ampersand           = js.Ampersand
	Pipe                = js.Pipe
	Caret               = js.Caret
	Arrow               = js.Arrow
	Incr                = js.Incr
	Decr                = js.Decr
	Pow                 = js.Pow
	Nullish             = js.Nullish
	LogicalAnd          = js.LogicalAnd
	LogicalOr           = js.LogicalOr
	Equal               = js.Equal
	StrictEqual         = js.StrictEqual
	NotEqual            = js.NotEqual
	StrictNotEqual      = js.StrictNotEqual
	LtEqual             = js.LtEqual
	GtEqual             = js.GtEqual
	ShiftLeft           = js.ShiftLeft
	ShiftRight          = js.ShiftRight
	UnsignedShift       = js.UnsignedShift
	ShiftLeftAssign     = js.ShiftLeftAssign
	ShiftRightAssign    = js.ShiftRightAssign
	UnsignedShiftAssign = js.UnsignedShiftAssign
	AddAssign           = js.AddAssign
	SubAssign           = js.SubAssign
	MulAssign           = js.MulAssign
	DivAssign           = js.DivAssign
	ModAssign           = js.ModAssign
	BitAndAssign        = js.BitAndAssign
	BitOrAssign         = js.BitOrAssign
	BitXorAssign        = js.BitXorAssign
	PowAssign           = js.PowAssign
	LogicalAndAssign    = js.LogicalAndAssign
	LogicalOrAssign     = js.LogicalOrAssign
	NullishAssign       = js.NullishAssign

	KindBreak      = js.KindBreak
	KindCase       = js.KindCase
	KindCatch      = js.KindCatch
	KindClass      = js.KindClass
	KindConst      = js.KindConst
	KindContinue   = js.KindContinue
	KindDebugger   = js.KindDebugger
	KindDefault    = js.KindDefault
	KindDelete     = js.KindDelete
	KindDo         = js.KindDo
	KindElse       = js.KindElse
	KindExport     = js.KindExport
	KindExtends    = js.KindExtends
	KindFinally    = js.KindFinally
	KindFor        = js.KindFor
	KindFunction   = js.KindFunction
	KindIf         = js.KindIf
	KindImport     = js.KindImport
	KindIn         = js.KindIn
	KindInstanceof = js.KindInstanceof
	KindNew        = js.KindNew
	KindReturn     = js.KindReturn
	KindSuper      = js.KindSuper
	KindSwitch     = js.KindSwitch
	KindThis       = js.KindThis
	KindThrow      = js.KindThrow
	KindTry        = js.KindTry
	KindTypeof     = js.KindTypeof
	KindVar        = js.KindVar
	KindVoid       = js.KindVoid
	KindWhile      = js.KindWhile
	KindWith       = js.KindWith

	KindTrue  = js.KindTrue
	KindFalse = js.KindFalse
	KindNull  = js.KindNull
)

// isReservedWord reports whether a token is a reserved word, valid only
// as an identifierName (property and member keys): the JavaScript
// keyword kinds spell their token text, so a token whose kind equals
// its alphabetic text is one of them.
func isReservedWord(tok gast.Token) bool {
	return tok.Text != "" && tok.Kind == tok.Text && isAlphaByte(tok.Text[0])
}

func isAlphaByte(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}
