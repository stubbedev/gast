// Package cli implements the gast command line: validating and dumping
// tokens and ASTs for the language frontends built on gast.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/stubbedev/gast/pkg/gast"
	"github.com/stubbedev/gast/pkg/lang/css"
	"github.com/stubbedev/gast/pkg/lang/html"
	"github.com/stubbedev/gast/pkg/lang/js"
	"github.com/stubbedev/gast/pkg/lang/php"
	"github.com/stubbedev/gast/pkg/lang/python"
	"github.com/stubbedev/gast/pkg/lang/sass"
	"github.com/stubbedev/gast/pkg/lang/scss"
	"github.com/stubbedev/gast/pkg/lang/ts"
)

// frontend bundles a language's parse and tokenize entry points.
type frontend struct {
	name       string
	tokenize   func(string) ([]gast.Token, error)
	validate   func(string) error
	extensions []string
}

var frontends = []frontend{
	{name: "php", tokenize: php.Tokenize, validate: php.Validate, extensions: []string{".php"}},
	{name: "javascript", tokenize: js.Tokenize, validate: js.Validate, extensions: []string{".js", ".mjs", ".cjs"}},
	{name: "html", tokenize: html.Tokenize, validate: html.Validate, extensions: []string{".html"}},
	{name: "css", tokenize: css.Tokenize, validate: css.Validate, extensions: []string{".css"}},
	{name: "scss", tokenize: scss.Tokenize, validate: scss.Validate, extensions: []string{".scss"}},
	{name: "sass", tokenize: sass.Tokenize, validate: sass.Validate, extensions: []string{".sass"}},
	{name: "python", tokenize: python.Tokenize, validate: python.Validate, extensions: []string{".py"}},
	{name: "typescript", tokenize: ts.Tokenize, validate: ts.Validate, extensions: []string{".ts"}},
	{name: "typescript", tokenize: ts.Tokenize, validate: ts.ValidateTSX, extensions: []string{".tsx"}},
}

// Run executes the CLI and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gast", flag.ContinueOnError)
	flags.SetOutput(stderr)
	list := flags.Bool("list", false, "list available frontends")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *list {
		for _, f := range frontends {
			fmt.Fprintln(stdout, f.name)
		}
		return 0
	}

	args = flags.Args()
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: gast <command> [file...]\n\ncommands:\n  validate  report syntax errors\n  tokens    print the token stream\n  list      list language frontends")
		return 2
	}

	command, files := args[0], args[1:]
	if len(files) == 0 {
		fmt.Fprintln(stderr, "gast: no input files")
		return 2
	}

	exitCode := 0
	for _, path := range files {
		code := runFile(command, path, stdout, stderr)
		if code > exitCode {
			exitCode = code
		}
	}
	return exitCode
}

func runFile(command, path string, stdout, stderr io.Writer) int {
	f, err := frontendFor(path)
	if err != nil {
		fmt.Fprintf(stderr, "gast: %v\n", err)
		return 2
	}
	source, err := os.ReadFile(path) //nolint:gosec // the path is a user-supplied command line argument
	if err != nil {
		fmt.Fprintf(stderr, "gast: %v\n", err)
		return 2
	}
	switch command {
	case "validate":
		if err := f.validate(string(source)); err != nil {
			fmt.Fprintln(stdout, gast.FormatError(err, string(source)))
			return 1
		}
		fmt.Fprintf(stdout, "%s: ok\n", path)
	case "tokens":
		tokens, err := f.tokenize(string(source))
		if err != nil {
			fmt.Fprintln(stdout, gast.FormatError(err, string(source)))
			return 1
		}
		for _, token := range tokens {
			text := strings.ReplaceAll(token.Text, "\n", "\\n")
			fmt.Fprintf(stdout, "%-14s %q\n", token.Kind, text)
		}
	default:
		fmt.Fprintf(stderr, "gast: unknown command %q\n", command)
		return 2
	}
	return 0
}

func frontendFor(path string) (frontend, error) {
	dot := strings.LastIndexByte(path, '.')
	if dot < 0 {
		return frontend{}, fmt.Errorf("cannot pick a frontend for %s: no file extension", path)
	}
	extension := path[dot:]
	for _, f := range frontends {
		if slices.Contains(f.extensions, extension) {
			return f, nil
		}
	}
	return frontend{}, fmt.Errorf("no frontend for extension %s", extension)
}

// Main wires Run to the process and exits; kept as a thin seam for
// tests.
func Main() {
	if code := Run(os.Args[1:], os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}
