package html

import (
	"testing"

	"github.com/stubbedev/gast/pkg/gast"
)

func kinds(source string) ([]string, error) {
	tokens, err := Tokenize(source)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		result = append(result, token.Kind)
	}
	return result, nil
}

func assertKinds(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d tokens %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: got %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}

func TestTokenizeDocument(t *testing.T) {
	t.Parallel()
	got, err := kinds(`<!DOCTYPE html><p class="x">hi</p>`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		KindDoctype, KindTagOpen, KindAttr, KindTagClose,
		KindText, KindEndTagOpen, KindTagClose, KindEOF,
	}
	assertKinds(t, got, want)
}

func TestTokenizeAttributeStyles(t *testing.T) {
	t.Parallel()
	tokens, err := Tokenize(`<div id=a class='b c' data-x="d e" hidden></div>`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`id=a`, `class='b c'`, `data-x="d e"`, `hidden`}
	var attrs []string
	for _, token := range tokens {
		if token.Kind == KindAttr {
			attrs = append(attrs, token.Text)
		}
	}
	if len(attrs) != len(want) {
		t.Fatalf("got attributes %q, want %q", attrs, want)
	}
	for i := range want {
		if attrs[i] != want[i] {
			t.Fatalf("attribute %d: got %q, want %q (%q)", i, attrs[i], want[i], attrs)
		}
	}
}

func TestTokenizeSelfClose(t *testing.T) {
	t.Parallel()
	got, err := kinds(`<br/><img src="x.png" />`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		KindTagOpen, KindSelfClose,
		KindTagOpen, KindAttr, KindSelfClose, KindEOF,
	}
	assertKinds(t, got, want)
}

func TestTokenizeRawText(t *testing.T) {
	t.Parallel()
	source := `<script>var a = "</scr" + "ipt>", b = 1 < 2; if (c </scriptx) {}</script>`
	tokens, err := Tokenize(source)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{KindTagOpen, KindTagClose, KindRawText, KindEndTagOpen, KindTagClose, KindEOF}
	got := make([]string, 0, len(tokens))
	for _, token := range tokens {
		got = append(got, token.Kind)
	}
	assertKinds(t, got, want)
	raw := tokens[2]
	wantText := `var a = "</scr" + "ipt>", b = 1 < 2; if (c </scriptx) {}`
	if raw.Text != wantText {
		t.Fatalf("raw text: got %q, want %q", raw.Text, wantText)
	}
	wantSpan := gast.Span{Start: len("<script>"), End: len(source) - len("</script>")}
	if raw.Span != wantSpan {
		t.Fatalf("raw span: got %v, want %v", raw.Span, wantSpan)
	}
}

func TestTokenizeRawTextCaseInsensitiveEnd(t *testing.T) {
	t.Parallel()
	got, err := kinds(`<STYLE>p>a{}</STYLE >`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{KindTagOpen, KindTagClose, KindRawText, KindEndTagOpen, KindTagClose, KindEOF}
	assertKinds(t, got, want)
}

func TestTokenizeRCDATA(t *testing.T) {
	t.Parallel()
	source := `<textarea>a < b &amp; <i>raw</i></textarea>`
	tokens, err := Tokenize(source)
	if err != nil {
		t.Fatal(err)
	}
	if tokens[2].Kind != KindRCDATA || tokens[2].Text != `a < b &amp; <i>raw</i>` {
		t.Fatalf("rcdata token: %q %q", tokens[2].Kind, tokens[2].Text)
	}
}

func TestTokenizeCommentsAndCDATA(t *testing.T) {
	t.Parallel()
	got, err := kinds(`<!-- one --><!bogus><![CDATA[x < y]]>`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{KindComment, KindComment, KindCDATA, KindEOF}
	assertKinds(t, got, want)
}

func TestTokenizeTextAngles(t *testing.T) {
	t.Parallel()
	source := `a < b, c <> d, 5 <6, x </ y <em>e</em>`
	tokens, err := Tokenize(source)
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Kind != KindText {
		t.Fatalf("first token: %q", tokens[0].Kind)
	}
	wantText := `a < b, c <> d, 5 <6, x </ y `
	if tokens[0].Text != wantText {
		t.Fatalf("text: got %q, want %q", tokens[0].Text, wantText)
	}
	if tokens[1].Kind != KindTagOpen {
		t.Fatalf("second token: %q", tokens[1].Kind)
	}
}

func TestTokenizeErrors(t *testing.T) {
	t.Parallel()
	valid := map[string]string{
		`<div class="x">ok</div>`: "",
		`<!doctype html>`:         "",
		`<!-- comment -->`:        "",
		`<![CDATA[data]]>`:        "",
		`<script></script>`:       "",
		`<script>x</script>`:      "",
		`<textarea></textarea>`:   "",
	}
	for source := range valid {
		if _, err := Tokenize(source); err != nil {
			t.Errorf("%q: unexpected error %v", source, err)
		}
	}
	invalid := map[string]string{
		`<div class="x>text</div>`: "unclosed quote in attribute value",
		`<div class='x>`:           "unclosed quote in attribute value",
		`<div`:                     "unterminated tag",
		`<div class="x"`:           "unterminated tag",
		`<!-- dangling`:            "unterminated comment",
		`<!bogus dangling`:         "unterminated comment",
		`<![CDATA[dangling`:        "unterminated CDATA section",
		`<!doctype html`:           "unterminated doctype",
		`<script>var x = 1;`:       "unterminated script element",
		`<style>a{}`:               "unterminated style element",
		`<textarea>text`:           "unterminated textarea element",
	}
	for source, message := range invalid {
		_, err := Tokenize(source)
		if err == nil {
			t.Errorf("%q: expected error", source)
			continue
		}
		if err.Error() != message {
			t.Errorf("%q: got %q, want %q", source, err.Error(), message)
		}
	}
}

func TestTokenizeSpans(t *testing.T) {
	t.Parallel()
	source := `<p class="a">x</p>`
	tokens, err := Tokenize(source)
	if err != nil {
		t.Fatal(err)
	}
	open := tokens[0]
	if open.Text != "<p" || open.Span != (gast.Span{Start: 0, End: 2}) {
		t.Fatalf("tag open: %q %v", open.Text, open.Span)
	}
	attr := tokens[1]
	if attr.Text != `class="a"` || attr.Span != (gast.Span{Start: 3, End: 12}) {
		t.Fatalf("attr: %q %v", attr.Text, attr.Span)
	}
	text := tokens[3]
	if text.Text != "x" || text.Span != (gast.Span{Start: 13, End: 14}) {
		t.Fatalf("text: %q %v", text.Text, text.Span)
	}
	end := tokens[4]
	if end.Text != "</p" || end.Span != (gast.Span{Start: 14, End: 17}) {
		t.Fatalf("end tag open: %q %v", end.Text, end.Span)
	}
}
