package html

import (
	"errors"
	"strings"
	"testing"
	"unsafe"

	"github.com/stubbedev/gast/pkg/gast"
)

// unescapeCases was produced by python3's html.unescape over the HTML5
// entity rules; they double as a differential check of decodeEntities
// in text mode against an independent implementation.
var unescapeCases = []struct{ in, want string }{
	{"&amp;", "&"},
	{"&lt;", "<"},
	{"&gt;", ">"},
	{"&quot;", "\""},
	{"&apos;", "'"},
	{"&nbsp;", "\u00a0"},
	{"&copy;", "\u00a9"},
	{"&Amp;", "&Amp;"},
	{"&amp", "&"},     // legacy form without semicolon decodes in text
	{"&ampx", "&x"},   // longest legacy match, remainder kept
	{"&ampx;", "&x;"}, // semicolon after the match is kept
	{"&notit;", "\u00acit;"},
	{"&amp&lt;", "&<"},
	{"&&amp;", "&&"},
	{"&counter", "&counter"},
	{"&nope;", "&nope;"},
	{"&", "&"}, // nothing nameable follows
	{"a & b", "a & b"},
	{"& ", "& "},
	{"&Cup;", "\u22d3"},
	{"&CounterClockwiseContourIntegral;", "\u2233"},
	{"&#65;", "A"},
	{"&#65", "A"}, // numeric without semicolon decodes
	{"&#x42;", "B"},
	{"&#X41;", "A"},
	{"&#x1F600;", "\U0001F600"},
	{"&#065;", "A"},      // leading zeros are fine
	{"&#000;", "\uFFFD"}, // zero is replaced
	{"&#0;", "\uFFFD"},
	{"&#128;", "\u20ac"}, // C1 range remapped through windows-1252
	{"&#x80;", "\u20ac"},
	{"&#0152;", "\u02dc"},
	{"&#xD800;", "\uFFFD"}, // surrogate
	{"&#x110000;", "\uFFFD"},
	{"&#999999999999;", "\uFFFD"},
	{"&#12x;", "\fx;"}, // digits consumed, x and ; kept
	{"&sup1", "\u00b9"},
	{"&sup1x", "\u00b9x"},
	{"Tom &amp; Jerry &lt;3", "Tom & Jerry <3"},
}

func TestDecodeEntitiesMatchesPythonUnescape(t *testing.T) {
	t.Parallel()
	for _, test := range unescapeCases {
		got, err := decodeEntities(test.in, 0, false)
		if err != nil {
			t.Errorf("%q: unexpected error %v", test.in, err)
			continue
		}
		if got != test.want {
			t.Errorf("%q: got %q, want %q", test.in, got, test.want)
		}
	}
}

func TestDecodeEntitiesAttributeLegacyRule(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"&amp=", "&amp="},     // legacy form followed by '=' stays literal
		{"&ampx", "&ampx"},     // ...and so does one followed by alphanumeric
		{"&ampx;", "&ampx;"},   // the would-be semicolon is just kept
		{"&amp", "&"},          // otherwise legacy forms decode
		{"&amp;", "&"},         // semicolon forms always decode
		{"&amp;x", "&x"},       // ...even before alphanumerics
		{"&#65=x", "A=x"},      // numeric references are not guarded
		{"&copy=r", "&copy=r"}, // legacy guard applies to any named form
	}
	for _, test := range tests {
		got, err := decodeEntities(test.in, 0, true)
		if err != nil {
			t.Errorf("%q: unexpected error %v", test.in, err)
			continue
		}
		if got != test.want {
			t.Errorf("%q: got %q, want %q", test.in, got, test.want)
		}
	}
}

func TestDecodeEntitiesMalformedNumeric(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"&#;", "&#x;", "&# ;", "a &#; b"} {
		_, err := decodeEntities(in, 10, false)
		if err == nil {
			t.Errorf("%q: expected error", in)
			continue
		}
		lexErr := &gast.LexError{}
		if !errors.As(err, &lexErr) {
			t.Errorf("%q: got %T, want *gast.LexError", in, err)
			continue
		}
		want := 10 + strings.Index(in, "&#")
		if lexErr.Pos.Start != want {
			t.Errorf("%q: error at %d, want %d", in, lexErr.Pos.Start, want)
		}
	}
}

// TestDecodeEntitiesFastPath checks that text without "&" comes back
// as the identical string, without allocation.
func TestDecodeEntitiesFastPath(t *testing.T) {
	t.Parallel()
	in := "plain text, no references here"
	got, err := decodeEntities(in, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if !sameStringData(got, in) {
		t.Errorf("got a copy %q, want the original string", got)
	}
}

func sameStringData(a, b string) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	return unsafe.StringData(a) == unsafe.StringData(b)
}

func TestNamedEntitiesTableSize(t *testing.T) {
	t.Parallel()
	if got := len(namedEntities); got != 2231 {
		t.Fatalf("namedEntities has %d entries, want 2231", got)
	}
	for _, key := range []string{"amp", "amp;", "lt;", "CounterClockwiseContourIntegral;"} {
		if _, ok := namedEntities[key]; !ok {
			t.Errorf("table missing key %q", key)
		}
	}
	if got := namedEntities["nbsp;"]; got != "\u00a0" {
		t.Errorf("nbsp; = %q, want U+00A0", got)
	}
}

func TestParseDecodesEntities(t *testing.T) {
	t.Parallel()
	doc, err := Parse(`<p title="&amp;=&#65;">&amp; &lt;tag&gt; &notit; &amp</p>`)
	if err != nil {
		t.Fatal(err)
	}
	p := elementChild(t, bodyOf(t, doc), 0)
	if got, want := p.Attrs[0].Name, "title"; got != want {
		t.Fatalf("attr name: %q, want %q", got, want)
	}
	if got, want := p.Attrs[0].Value, "&=A"; got != want {
		t.Fatalf("attr value: got %q, want %q", got, want)
	}
	text, ok := p.Children[0].(*Text)
	if !ok {
		t.Fatalf("child: %T", p.Children[0])
	}
	if want := `&amp; &lt;tag&gt; &notit; &amp`; text.Text != want {
		t.Fatalf("raw text: got %q, want %q", text.Text, want)
	}
	if want := "& <tag> \u00acit; &"; text.Value != want {
		t.Fatalf("value: got %q, want %q", text.Value, want)
	}
}

func TestParseDecodesRCDATAEntities(t *testing.T) {
	t.Parallel()
	doc, err := Parse(`<textarea>&amp; stays decoded</textarea>`)
	if err != nil {
		t.Fatal(err)
	}
	textarea := elementChild(t, bodyOf(t, doc), 0)
	text, ok := textarea.Children[0].(*Text)
	if !ok {
		t.Fatalf("child: %T", textarea.Children[0])
	}
	if text.Text != "&amp; stays decoded" || text.Value != "& stays decoded" {
		t.Fatalf("rcdata: raw %q value %q", text.Text, text.Value)
	}
}

func TestParseRawTextUndecoded(t *testing.T) {
	t.Parallel()
	doc, err := Parse(`<script>if (a && b) x = "&amp;";</script>`)
	if err != nil {
		t.Fatal(err)
	}
	script := elementChild(t, bodyOf(t, doc), 0)
	raw, ok := script.Children[0].(*RawText)
	if !ok {
		t.Fatalf("child: %T", script.Children[0])
	}
	if raw.Text != `if (a && b) x = "&amp;";` {
		t.Fatalf("script content: %q", raw.Text)
	}
}

func TestParseEntityError(t *testing.T) {
	t.Parallel()
	err := Validate("<p>a &#; b</p>")
	if err == nil || err.Error() != "numeric character reference with no digits" {
		t.Fatalf("got %v, want a numeric-character-reference error", err)
	}
}
