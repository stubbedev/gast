// Package html implements an HTML frontend for gast: a tokenizer, a
// tree-building parser producing a spanned AST, and syntax validation.
// It targets a practical WHATWG subset: strict about structure (end
// tags must match, elements must close, attributes must be unique and
// well quoted) and lenient about details browsers recover from
// (implicit sibling closing, optional html/head/body wrappers, a raw
// "<" in text). Entities in text and attribute values are kept as raw
// source text; they are never decoded into nodes.
package html

// Token kinds. Tag punctuation constants spell their token text
// exactly once; the lexer and parser derive every spelling from them
// so the sets cannot drift apart.
const (
	KindEOF = "eof"

	KindText    = "text"    // character data; entities stay raw
	KindRawText = "raw"     // script/style content, verbatim
	KindRCDATA  = "rcdata"  // title/textarea content; entities stay raw
	KindComment = "comment" // <!-- --> and bogus <! comments
	KindCDATA   = "cdata"   // <![CDATA[ ]]>
	KindDoctype = "doctype" // <!DOCTYPE ...>

	KindTagOpen    = "tag_open"     // <div, text includes the "<"
	KindEndTagOpen = "end_tag_open" // </div, text includes the "</"
	KindAttr       = "attr"         // name or name=value, raw source
	KindTagClose   = "tag_close"    // >
	KindSelfClose  = "self_close"   // />
)

// Markup delimiters and keywords; single sources of truth for every
// spelling used by the lexer and the parser.
const (
	lt           = "<"
	gt           = ">"
	slash        = "/"
	bang         = "!"
	endTagPrefix = lt + slash

	commentStart  = lt + bang + "--"
	commentEnd    = "--" + gt
	cdataStart    = lt + bang + "[CDATA["
	cdataEnd      = "]]" + gt
	doctypeStart  = lt + bang + "doctype"
	tagCloseText  = gt
	selfCloseText = slash + gt

	elementHTML = "html"
	elementHead = "head"
	elementBody = "body"
	elementP    = "p"
)

// Element-name sets, all lowercase; names are matched case-insensitively.
var (
	// voidElements have no content and must not have an end tag.
	voidElements = elementSet(
		"area", "base", "br", "col", "embed", "hr", "img", "input",
		"link", "meta", "param", "source", "track", "wbr",
	)

	// rawTextElements hold verbatim content ended only by their own
	// end tag.
	rawTextElements = elementSet("script", "style")

	// rcdataElements hold character data (kept raw here) ended only by
	// their own end tag.
	rcdataElements = elementSet("title", "textarea")

	// headElements may be placed in a synthesized head when the
	// document has no explicit html wrapper.
	headElements = elementSet("base", "link", "meta", "style", "title")

	// implicitlyClosable elements may be closed implicitly by an outer
	// element's end tag or at end of input; every other element must
	// be closed explicitly.
	implicitlyClosable = elementSet(
		"p", "li", "dd", "dt", "option", "optgroup", "td", "th", "tr",
		"tbody", "thead", "tfoot", "head", "body", "html",
	)
)

func elementSet(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// implicitClose maps a starting element name to the open elements it
// implicitly closes, mirroring the spec's sibling-closing rules.
var implicitClose = buildImplicitClose()

func buildImplicitClose() map[string][]string {
	closes := make(map[string][]string, 64)
	for _, name := range []string{
		"address", "article", "aside", "blockquote", "details", "div",
		"dl", "fieldset", "figcaption", "figure", "footer", "form",
		"h1", "h2", "h3", "h4", "h5", "h6", "header", "hgroup", "hr",
		"main", "menu", "nav", "ol", "p", "pre", "section", "table",
		"ul",
	} {
		closes[name] = []string{elementP}
	}
	closes["li"] = []string{"li", elementP}
	closes["dt"] = []string{"dt", "dd", elementP}
	closes["dd"] = []string{"dt", "dd", elementP}
	closes["td"] = []string{"td", "th", elementP}
	closes["th"] = []string{"td", "th", elementP}
	closes["tr"] = []string{"tr", "td", "th"}
	closes["option"] = []string{"option"}
	closes["optgroup"] = []string{"optgroup", "option"}
	for _, name := range []string{"tbody", "thead", "tfoot"} {
		closes[name] = []string{"tbody", "thead", "tfoot", "tr", "td", "th"}
	}
	return closes
}
