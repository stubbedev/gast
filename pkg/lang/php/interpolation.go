package php

import (
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

// parseStringParts splits an interpolated string body into literal and
// expression parts. body is the raw text between the delimiters and base
// the absolute offset of body[0] in source, so every part spans into the
// original source. Bodies without interpolation return nil parts.
func parseStringParts(source, body string, base int) ([]StringPart, error) {
	if !hasInterpolation(body) {
		return nil, nil
	}
	var parts []StringPart
	literal := 0 // start of the pending literal run in body
	flush := func(end int) {
		if end > literal {
			chunk := body[literal:end]
			parts = append(parts, &LiteralPart{
				Span:  bodySpan(base, literal, end),
				Text:  chunk,
				Value: decodeEscapes(chunk),
			})
		}
	}
	index := 0
	for index < len(body) {
		c := body[index]
		if c == '\\' {
			index += 2
			continue
		}
		var part *ExprPart
		var err error
		var end int
		braced := index+1 < len(body) && body[index+1] == '{'
		switch {
		case c == '{' && index+1 < len(body) && body[index+1] == '$':
			part, end, err = scanComplexPart(source, body, base, index)
		case c == '$' && braced:
			part, end, err = scanDollarBracePart(source, body, base, index)
		case c == '$' && startsSimpleVar(body, index):
			part, end, err = scanSimplePart(body, base, index)
		default:
			index++
			continue
		}
		if err != nil {
			return nil, err
		}
		flush(index)
		parts = append(parts, part)
		index = end
		literal = index
	}
	flush(len(body))
	return parts, nil
}

// hasInterpolation reports whether body contains an interpolation start:
// a {$ sequence or a $ beginning simple syntax. It is the
// zero-allocation fast path.
func hasInterpolation(body string) bool {
	for index := 0; index < len(body); index++ {
		switch body[index] {
		case '\\':
			index++
		case '$':
			if startsSimpleVar(body, index) || (index+1 < len(body) && body[index+1] == '{') {
				return true
			}
		case '{':
			if index+1 < len(body) && body[index+1] == '$' {
				return true
			}
		}
	}
	return false
}

// startsSimpleVar reports whether the $ at index begins the simple
// interpolation syntax: $name or $$name.
func startsSimpleVar(body string, index int) bool {
	if index+1 >= len(body) {
		return false
	}
	if body[index+1] == '$' {
		return index+2 < len(body) && isIdentStart(body[index+2])
	}
	return isIdentStart(body[index+1])
}

// scanSimplePart parses the simple syntax starting at a $: $name or
// $$name with at most one accessor, either ->prop / ?->prop or [index],
// as in "$x", "$$x", "$obj->prop", "$x?->p", "$arr[key]", "$arr[-1]",
// and "$arr[$i]". An -> not followed by a name stays literal text.
func scanSimplePart(body string, base, start int) (*ExprPart, int, error) {
	index := start + 1
	var expr Expr
	if body[index] == '$' {
		index++
		nameStart := index
		index += takeIdentBytes(body, index)
		inner := &Variable{
			Span: bodySpan(base, nameStart, index),
			Name: body[nameStart:index],
		}
		expr = &VariableVariable{Span: bodySpan(base, start, index), Expr: inner}
	} else {
		nameStart := index
		index += takeIdentBytes(body, index)
		expr = &Variable{
			Span: bodySpan(base, start, index),
			Name: body[nameStart:index],
		}
	}
	expr, end, err := scanSimpleAccessor(body, base, start, expr, index)
	if err != nil {
		return nil, 0, err
	}
	return exprPart(body, base, start, end, expr), end, nil
}

// scanSimpleAccessor consumes the single accessor php allows after a
// simple variable: one property arrow or one array index. Anything else,
// including a second accessor, stays literal.
func scanSimpleAccessor(body string, base, start int, expr Expr, index int) (Expr, int, error) {
	if strings.HasPrefix(body[index:], Arrow) || strings.HasPrefix(body[index:], NullsafeArrow) {
		nullsafe := body[index] == '?'
		prefix := Arrow
		if nullsafe {
			prefix = NullsafeArrow
		}
		nameStart := index + len(prefix)
		if nameStart < len(body) && isIdentStart(body[nameStart]) {
			end := nameStart + takeIdentBytes(body, nameStart)
			name := &Identifier{Span: bodySpan(base, nameStart, end), Name: body[nameStart:end]}
			span := bodySpan(base, start, end)
			if nullsafe {
				expr = &NullsafePropertyFetch{Span: span, Target: expr, Name: name}
			} else {
				expr = &PropertyFetch{Span: span, Target: expr, Name: name}
			}
			return expr, end, nil
		}
		return expr, index, nil
	}
	if index < len(body) && body[index] == '[' {
		indexExpr, end, err := scanEncapsIndex(body, base, index)
		if err != nil {
			return nil, 0, err
		}
		return &IndexExpr{
			Span:   bodySpan(base, start, end),
			Target: expr,
			Index:  indexExpr,
		}, end, nil
	}
	return expr, index, nil
}

// scanEncapsIndex parses the restricted index at body[bracket] == '[' of
// simple interpolation and of the ${name[...]} form: a bare word, a
// number with optional leading '-', or $name, closed by ']'. It returns
// the index expression and the body offset after ']'.
func scanEncapsIndex(body string, base, bracket int) (Expr, int, error) {
	first := bracket + 1
	if first >= len(body) {
		return nil, 0, partError(body, base, first, len(body), "expected array index in string interpolation")
	}
	var indexExpr Expr
	end := first
	switch c := body[first]; {
	case isIdentStart(c):
		end += takeIdentBytes(body, first)
		word := body[first:end]
		indexExpr = &StringLit{Span: bodySpan(base, first, end), Kind: StringBare, Text: word, Value: word}
	case isDigit(c) || (c == '-' && first+1 < len(body) && isDigit(body[first+1])):
		end += takeDigits(body, first)
		indexExpr = &IntegerLit{Span: bodySpan(base, first, end), Text: body[first:end]}
	case c == '$' && first+1 < len(body) && isIdentStart(body[first+1]):
		end += 1 + takeIdentBytes(body, first+1)
		indexExpr = &Variable{Span: bodySpan(base, first, end), Name: body[first+1 : end]}
	default:
		return nil, 0, partError(body, base, first, first+1, "expected array index in string interpolation")
	}
	if end >= len(body) || body[end] != ']' {
		return nil, 0, partError(body, base, end, len(body), "expected ']' after array index in string interpolation")
	}
	return indexExpr, end + 1, nil
}

// scanDollarBracePart parses the ${...} form starting at ${: a variable
// name, optionally with one restricted index (php's deprecated ${var}
// spelling, as in "${x}" and "${x[0]}"), or for any other content a
// variable variable built from the full expression inside the braces, as
// in "${'name'}" and "${$x}".
func scanDollarBracePart(source, body string, base, start int) (*ExprPart, int, error) {
	end, ok := matchingBrace(body, start+1)
	if !ok {
		return nil, 0, partError(body, base, start, len(body), "unterminated ${ in string interpolation")
	}
	var expr Expr
	if name, hasIndex, simple := bareName(body[start+2 : end]); simple {
		nameStart := start + 2
		varExpr := &Variable{
			Span: bodySpan(base, nameStart, nameStart+len(name)),
			Name: name,
		}
		expr = varExpr
		if hasIndex {
			bracket := nameStart + len(name)
			indexExpr, indexEnd, err := scanEncapsIndex(body, base, bracket)
			if err != nil {
				return nil, 0, err
			}
			expr = &IndexExpr{
				Span:   bodySpan(base, start, indexEnd),
				Target: varExpr,
				Index:  indexExpr,
			}
		}
	} else {
		inner, err := parseInterpExpression(source, base+start+2, base+end, false)
		if err != nil {
			return nil, 0, err
		}
		expr = &VariableVariable{Span: bodySpan(base, start, end), Expr: inner}
	}
	return exprPart(body, base, start, end+1, expr), end + 1, nil
}

// scanComplexPart parses the {$...} form starting at {: the full
// expression up to the matching brace, including array access, method
// calls, static access, and nested interpolation.
func scanComplexPart(source, body string, base, start int) (*ExprPart, int, error) {
	end, ok := matchingBrace(body, start)
	if !ok {
		return nil, 0, partError(body, base, start, len(body), "unterminated {$ in string interpolation")
	}
	// The expression starts right after the {$ pair.
	expr, err := parseInterpExpression(source, base+start+1, base+end, true)
	if err != nil {
		return nil, 0, err
	}
	return exprPart(body, base, start, end+1, expr), end + 1, nil
}

// parseInterpExpression tokenizes and parses the expression between two
// absolute offsets of source with the php lexer and parser, stopping at
// the '}' at end. The restricted flag selects the {$...} grammar, which
// rejects binary and ternary operators and bare literals; ${...}
// accepts any expression.
func parseInterpExpression(source string, start, end int, restricted bool) (Expr, error) {
	tokens, stop, err := tokenizeInterpolation(source, start)
	if err != nil {
		return nil, err
	}
	if stop != end {
		return nil, &gast.ParseError{Message: "unterminated interpolation", Pos: gast.Span{Start: start, End: end}}
	}
	inner := newParser(tokens, source)
	if restricted {
		return inner.parseEncapsExpr()
	}
	expr, err := inner.parseExpr()
	if err != nil {
		return nil, err
	}
	if !inner.AtEnd() {
		return nil, inner.Fail("unexpected " + gast.Describe(inner.Current()) + " in string interpolation")
	}
	return expr, nil
}

// matchingBrace returns the index of the brace closing the '{' at open,
// skipping escapes, quoted strings, and nested braces, or false when
// the body ends first.
func matchingBrace(body string, open int) (int, bool) {
	depth := 1
	for index := open + 1; index < len(body); index++ {
		switch body[index] {
		case '\\':
			index++
		case '\'', '"':
			index = skipQuoted(body, index)
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index, true
			}
		}
	}
	return 0, false
}

// skipQuoted returns the index of the closing quote of the string
// starting at start, or the last body index when unterminated.
func skipQuoted(body string, start int) int {
	quote := body[start]
	for index := start + 1; index < len(body); index++ {
		switch body[index] {
		case '\\':
			index++
		case quote:
			return index
		}
	}
	return len(body) - 1
}

// bareName reports the leading variable name of content when the
// remainder is empty or a bracketed index - the shapes ${...} takes
// without parsing a full expression - and whether an index follows.
func bareName(content string) (name string, hasIndex, ok bool) {
	name = content[:takeIdentBytes(content, 0)]
	if name == "" {
		return "", false, false
	}
	rest := content[len(name):]
	if rest == "" {
		return name, false, true
	}
	bracketed := rest[0] == '[' && rest[len(rest)-1] == ']'
	return name, bracketed, bracketed
}

// takeIdentBytes returns the length of the identifier run at start.
func takeIdentBytes(body string, start int) int {
	end := start
	for end < len(body) && isIdentByte(body[end]) {
		end++
	}
	return end - start
}

// takeDigits returns the length of the optionally '-'-prefixed digit run
// at start.
func takeDigits(body string, start int) int {
	end := start
	if end < len(body) && body[end] == '-' {
		end++
	}
	for end < len(body) && isDigit(body[end]) {
		end++
	}
	return end - start
}

// exprPart builds the expression part for body[start:end].
func exprPart(body string, base, start, end int, expr Expr) *ExprPart {
	return &ExprPart{
		Span: bodySpan(base, start, end),
		Raw:  body[start:end],
		Expr: expr,
	}
}

// bodySpan returns the span of body[start:end] in the source, where
// base is the absolute offset of body[0].
func bodySpan(base, start, end int) gast.Span {
	return gast.Span{Start: base + start, End: base + end}
}

// partError builds a positioned interpolation error over body[from:to].
func partError(body string, base, from, to int, message string) error {
	if to > len(body) {
		to = len(body)
	}
	if from >= to {
		from = to - 1
	}
	if from < 0 {
		from = 0
	}
	return &gast.ParseError{
		Message: message + ", found " + describeBodyByte(body, from),
		Pos:     gast.Span{Start: base + from, End: base + to},
	}
}

// describeBodyByte renders the byte at from for an error message.
func describeBodyByte(body string, from int) string {
	if from >= len(body) {
		return "end of string"
	}
	return "'" + string(body[from]) + "'"
}
