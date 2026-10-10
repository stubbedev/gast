package sasscore

import (
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
)

// SelectorList is a comma-separated selector list.
type SelectorList struct {
	gast.Span
	Complexes []*ComplexSelector
}

// ComplexSelector is one complex selector: compound selectors joined
// by combinators. Combinators[i] joins Compounds[i] and
// Compounds[i+1]; descendant selectors use a whitespace combinator.
type ComplexSelector struct {
	gast.Span
	Leading     string // leading combinator of a relative selector, as in :has(> img)
	Compounds   []*CompoundSelector
	Combinators []string
}

// CompoundSelector is a sequence of simple selectors with no
// combinators: a type or universal selector followed by id, class,
// attribute, and pseudo parts. Interp holds the raw tokens of #{}
// interpolation parts.
type CompoundSelector struct {
	gast.Span
	Type        string
	Universal   bool
	Parent      bool // SCSS &
	Placeholder bool // SCSS %name
	ID          string
	Classes     []string
	Attributes  []*AttributeSelector
	Pseudos     []*PseudoSelector
	Interp      []gast.Token
}

// AttributeSelector is an [attribute] selector; Op is one of "",
// "=", "~=", "^=", "$=", "*=", "|=".
type AttributeSelector struct {
	gast.Span
	Name   string
	Op     string
	Value  string
	Quoted bool
	Flag   string // trailing i or s modifier
}

// PseudoSelector is a :class or ::element pseudo. Selector holds the
// parsed inner selector list for selector-taking pseudos such as
// :is(); Args holds the raw argument tokens otherwise.
type PseudoSelector struct {
	gast.Span
	Name     string
	Element  bool
	Args     []gast.Token
	Selector *SelectorList
}

// parseSelectorList parses a non-empty selector token stream.
func parseSelectorList(tokens []gast.Token) (*SelectorList, error) {
	if len(tokens) == 0 {
		return nil, &gast.ParseError{Message: "empty selector"}
	}
	q := gast.NewTokenCursor(tokens, KindEOF)
	list := &SelectorList{}
	for {
		complex, err := parseComplexSelector(q)
		if err != nil {
			return nil, err
		}
		list.Complexes = append(list.Complexes, complex)
		if !q.Match(Comma) {
			break
		}
	}
	list.Span = gast.Merge(tokens[0].Span, tokens[len(tokens)-1].Span)
	return list, nil
}

func parseComplexSelector(q *gast.TokenCursor) (*ComplexSelector, error) {
	complex := &ComplexSelector{}
	if isCombinator(q.Current().Kind) {
		// Relative selector, as inside :has(> img).
		complex.Leading = q.Advance().Kind
	}
	pending := ""
	for {
		if len(complex.Compounds) > 0 && isCombinator(q.Current().Kind) {
			pending = q.Advance().Kind
		}
		compound, err := parseCompoundSelector(q)
		if err != nil {
			return nil, err
		}
		if len(complex.Compounds) > 0 {
			complex.Combinators = append(complex.Combinators, pending)
		}
		complex.Compounds = append(complex.Compounds, compound)
		if q.AtEnd() || q.Check(Comma) {
			break
		}
		pending = Descendant
	}
	complex.Span = gast.Merge(complex.Compounds[0].Span,
		complex.Compounds[len(complex.Compounds)-1].Span)
	return complex, nil
}

func parseCompoundSelector(q *gast.TokenCursor) (*CompoundSelector, error) {
	c := &CompoundSelector{}
	start := q.Current().Span
	saw := false
loop:
	for !q.AtEnd() && !q.Check(Comma) {
		tok := q.Current()
		adjacent := tok.Span.Start == q.Peek(-1).Span.End
		switch tok.Kind {
		case Asterisk:
			if saw {
				break loop
			}
			c.Universal = true
			q.Advance()
			saw = true
		case KindIdent:
			// An identifier continues a compound only when glued to the
			// previous part, as in the SCSS suffixes &__element, &-mod,
			// and .#{name}-mod.
			if saw {
				if adjacent && (c.Parent || len(c.Interp) > 0) {
					c.Interp = append(c.Interp, tok)
					q.Advance()
					continue
				}
				break loop
			}
			c.Type = tok.Text
			q.Advance()
			saw = true
		case Minus:
			// A hyphen glued to the compound continues it when an
			// interpolated suffix follows: .#{name}-#{$mod}.
			if !adjacent || !q.CheckAhead(1, KindInterpOpen) {
				break loop
			}
			q.Advance()
			c.Interp = append(c.Interp, collectInterpolation(q)...)
		case Ampersand:
			if saw {
				// A glued & continues the compound (.nav&); a spaced one
				// starts a new compound.
				if !adjacent || c.Parent || len(c.Pseudos) > 0 || len(c.Attributes) > 0 {
					break loop
				}
			}
			c.Parent = true
			q.Advance()
			saw = true
		case Percent:
			q.Advance()
			name, err := q.Expect(KindIdent, "placeholder name")
			if err != nil {
				return nil, err
			}
			c.Placeholder = true
			c.Type = name.Text
			saw = true
		case KindHashID:
			c.ID = tok.Text[1:]
			q.Advance()
			saw = true
		case Dot:
			q.Advance()
			switch {
			case q.Check(KindIdent):
				c.Classes = append(c.Classes, q.Advance().Text)
			case q.Check(KindInterpOpen):
				c.Interp = append(c.Interp, collectInterpolation(q)...)
			default:
				return nil, q.Fail("expected class name")
			}
			saw = true
		case LBracket:
			attr, err := parseAttributeSelector(q)
			if err != nil {
				return nil, err
			}
			c.Attributes = append(c.Attributes, attr)
			saw = true
		case Colon:
			pseudo, err := parsePseudoSelector(q)
			if err != nil {
				return nil, err
			}
			c.Pseudos = append(c.Pseudos, pseudo)
			saw = true
		case KindInterpOpen:
			c.Interp = append(c.Interp, collectInterpolation(q)...)
			saw = true
		default:
			break loop
		}
	}
	if !saw {
		return nil, q.Fail("expected selector, found " + gast.Describe(q.Current()))
	}
	c.Span = q.SpanFrom(start)
	return c, nil
}

func parseAttributeSelector(q *gast.TokenCursor) (*AttributeSelector, error) {
	open := q.Advance() // [
	name, err := q.Expect(KindIdent, "attribute name")
	if err != nil {
		return nil, err
	}
	attr := &AttributeSelector{Name: name.Text}
	switch {
	case q.Check(Equals):
		attr.Op = q.Advance().Text
	case isAttrOpStart(q.Current().Kind) && q.CheckAhead(1, Equals):
		attr.Op = q.Advance().Text + q.Advance().Text
	}
	if attr.Op != "" {
		value := q.Advance()
		switch value.Kind {
		case KindString:
			attr.Value = value.Text[1 : len(value.Text)-1]
			attr.Quoted = true
		case KindIdent, KindNumber:
			attr.Value = value.Text
		default:
			return nil, q.FailAt(value.Span, "expected attribute value")
		}
		if q.Check(KindIdent) {
			flag := q.Current().Text
			if strings.EqualFold(flag, "i") || strings.EqualFold(flag, "s") {
				attr.Flag = strings.ToLower(flag)
				q.Advance()
			}
		}
	}
	if !q.Match(RBracket) {
		return nil, q.Fail("expected ']'")
	}
	attr.Span = q.SpanFrom(open.Span)
	return attr, nil
}

// isAttrOpStart reports whether kind may begin a compound attribute
// operator such as ~= or $=.
func isAttrOpStart(kind string) bool {
	switch kind {
	case Tilde, Caret, Dollar, Asterisk, Pipe:
		return true
	}
	return false
}

func parsePseudoSelector(q *gast.TokenCursor) (*PseudoSelector, error) {
	first := q.Advance() // :
	pseudo := &PseudoSelector{Element: q.Match(Colon)}
	nameTok := q.Advance()
	if nameTok.Kind != KindIdent && nameTok.Kind != KindFunction {
		return nil, q.FailAt(nameTok.Span, "expected pseudo-class name")
	}
	if nameTok.Kind == KindFunction {
		pseudo.Name = strings.TrimSuffix(nameTok.Text, LParen)
		inner := collectGroup(q, 1)
		if selectorPseudos[strings.ToLower(pseudo.Name)] {
			selector, err := parseSelectorList(inner)
			if err != nil {
				return nil, err
			}
			pseudo.Selector = selector
		} else {
			pseudo.Args = inner
		}
	} else {
		pseudo.Name = nameTok.Text
	}
	pseudo.Span = q.SpanFrom(first.Span)
	return pseudo, nil
}

// collectInterpolation collects one #{} group through its matching },
// delimiters included.
func collectInterpolation(q *gast.TokenCursor) []gast.Token {
	open := q.Advance()
	tokens := append([]gast.Token{open}, collectGroup(q, 1)...)
	if close := q.Peek(-1); close.Kind == RBrace {
		tokens = append(tokens, close)
	}
	return tokens
}

// collectGroup collects the tokens of a bracketed group, consuming
// through the matching closing token, which is excluded from the
// result. openers counts groups whose opening token was already
// consumed, such as a function's name(.
func collectGroup(q *gast.TokenCursor, openers int) []gast.Token {
	var tokens []gast.Token
	depth := openers
	for !q.AtEnd() {
		tok := q.Current()
		switch tok.Kind {
		case KindInterpOpen, KindFunction, LParen, LBracket, LBrace:
			depth++
		case RParen, RBracket, RBrace:
			depth--
			if depth == 0 {
				q.Advance()
				return tokens
			}
		}
		tokens = append(tokens, q.Advance())
	}
	return tokens
}

// isCombinator reports whether kind is an explicit selector
// combinator.
func isCombinator(kind string) bool {
	switch kind {
	case Gt, Plus, Tilde:
		return true
	}
	return false
}
