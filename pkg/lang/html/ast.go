package html

import (
	"github.com/stubbedev/gast/pkg/gast"
)

// Node is implemented by every AST node; Pos returns the node's span.
type Node interface {
	Pos() gast.Span
}

// File is the root document node. Children hold at most one Doctype,
// leading comments and whitespace, and the html element - explicit or
// synthesized (a synthesized wrapper carries no span).
type File struct {
	gast.Span
	Children []Node
}

// Doctype is a <!DOCTYPE> declaration.
type Doctype struct {
	gast.Span
	Name     string
	PublicID string
	SystemID string
}

// Element is an element: its start tag name, attributes, and content.
type Element struct {
	gast.Span
	Name     string
	Attrs    []Attribute
	Children []Node
}

// Attribute is one attribute. Value is the decoded value - character
// references resolved per the WHATWG tokenizer rules - without the
// surrounding quotes; it is empty for a valueless attribute. The raw
// source text, entities included, remains available through the span.
type Attribute struct {
	gast.Span
	Name  string
	Value string
}

// Text is character data. Text holds the raw source including any
// character references; Value holds the same data with the references
// decoded.
type Text struct {
	gast.Span
	Text  string
	Value string
}

// Comment is a <!-- --> comment or a bogus <! comment; Text holds the
// content without delimiters.
type Comment struct {
	gast.Span
	Text string
}

// RawText is the verbatim content of a script or style element; Kind
// is the lowercase element name.
type RawText struct {
	gast.Span
	Kind string
	Text string
}

// CDATA is a <![CDATA[ ]]> section; Text holds the content without
// delimiters.
type CDATA struct {
	gast.Span
	Text string
}
