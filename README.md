# gast

A lexer and parser toolkit with pluggable language frontends, producing fully-spanned ASTs. The first frontend is PHP: a complete PHP 8 lexer and recursive-descent parser with syntax validation.

## Layout

```
cmd/gast        CLI entry point
internal/cli    command implementations (validate, tokens, list)
pkg/gast        language-agnostic core: spans, source maps, tokens, lexer and parser cursors, error types
pkg/lang/php    PHP frontend: lexer, parser, AST, validation
```

## Usage

Library:

```go
import (
	"fmt"
	"os"

	"github.com/stubbedev/gast/pkg/gast"
	"github.com/stubbedev/gast/pkg/lang/php"
)

source, _ := os.ReadFile("app.php")

// Syntax validation with caret diagnostics.
if err := php.Validate(string(source)); err != nil {
	fmt.Println(gast.FormatError(err, string(source)))
}

// Or the full AST; every token and node carries a byte-offset span.
file, err := php.Parse(string(source))

// Or just the token stream.
tokens, err := php.Tokenize(string(source))
```

CLI:

```sh
gast validate src/*.php   # exit 0 valid, 1 with diagnostics, 2 usage errors
gast tokens file.php      # one "kind text" pair per token
gast -list                # available frontends
```

Diagnostics render with position and caret:

```
ParseError at 2:7: expected ';' after statement, found end of input
2 | $a = 1
  |       ^
```

## PHP coverage

Targets PHP 8.x syntax:

- Full tag handling: `<?php`, `<?=`, `?>`, inline HTML, tags inside comments
- Numbers: decimal, hex, binary, octal, legacy octal, floats, exponents, PHP's exact underscore and malformed-octal rules (`08` and `1__2` are errors, `08.5` is a float, `1and 2` lexes as `1 and 2`)
- Strings: single- and double-quoted with escape decoding, heredoc, nowdoc, shell exec
- Operators and precedence matching php.net's table, including `.` below `+`/`<<`, right-associative `**` and `??`, keyword `and`/`or`/`xor` looser than assignment
- Classes (abstract, final, readonly, anonymous, promotion), interfaces, traits with adaptation clauses, enums including backed-enum case checks, typed constants
- Attributes, namespaces (braced and simple), use groups with `function`/`const` entries, unions/intersections/DNF types
- Control flow in both brace and alternative syntax, match expressions, closures and arrow functions (incl. `static fn`), first-class callables, named arguments, generators, `throw` as an expression

Not parsed: `__halt_compiler` (everything after it is compiler-terminated data), and interpolation is preserved as raw text rather than an AST structure.

The corpus under `pkg/lang/php/testdata` is cross-checked against the PHP interpreter: every file in `valid/` passes `php -l` and every file in `invalid/` fails it.

## Core (pkg/gast)

- `Span`, `Merge`, and `SourceMap` (byte offsets to 1-based line:column)
- `Cursor`: byte cursor with token-span tracking, `Peek`/`Advance`/`Match`/`MatchWord`, `SkipWhile`, and `Fail` returning a positioned `LexError`
- `TokenCursor`: token cursor with a synthetic end-of-input token, `Check`/`Expect`/`Fail`, `SpanFrom` for node spans
- `LexError`/`ParseError` implementing `Spanned`, and `FormatError` for caret diagnostics

## Adding a frontend

Create `pkg/lang/<name>` with the same shape as `pkg/lang/php`: a kind list (constants spelling each token text exactly once, with the keyword and operator tables derived from them), a lexer over `gast.Cursor`, an AST of nodes embedding `gast.Span`, a parser over `gast.TokenCursor`, and `Parse`/`Validate`/`Tokenize` entry points. Register it in `internal/cli`'s frontend table.

## Development

`just check` runs vet, lint, test, and build. The dev shell provides Go, gopls, golangci-lint, and just via the flake.

## Verification

- `go test ./...`: unit tests for the core cursors, source maps, and error formatting; lexer and parser tests covering token kinds, precedence, and spans; corpus validation tests
- Corpus cross-checked against the installed PHP interpreter
- `golangci-lint run`: clean under the strict config
