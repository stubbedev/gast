package gast

import "strings"

// Token is a lexical token: its kind, raw source text, and span.
// Kind is a plain string so each frontend can define its own set of
// kind constants.
type Token struct {
	Kind string
	Text string
	Span Span
}

// Describe renders a token for use in error messages.
func Describe(token Token) string {
	if token.Text == "" {
		return "end of input"
	}
	replacer := strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	return "'" + replacer.Replace(token.Text) + "'"
}
