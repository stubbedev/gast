# gast

A lexer and parser toolkit with pluggable language frontends, producing fully-spanned ASTs. Frontends: PHP 8, JavaScript (ES2024), TypeScript 5, HTML, CSS, SCSS, SASS, and Python 3.12+.

## Layout

```
cmd/gast           CLI entry point
internal/cli       command implementations (validate, tokens, list)
internal/sasscore  shared CSS/SCSS/SASS scanning and parsing core
pkg/gast           language-agnostic core: spans, source maps, tokens, lexer and parser cursors, error types
pkg/lang/php       PHP 8 frontend
pkg/lang/js        JavaScript frontend
pkg/lang/ts        TypeScript frontend
pkg/lang/html      HTML frontend
pkg/lang/css       CSS frontend
pkg/lang/scss      SCSS frontend
pkg/lang/sass      SASS (indented syntax) frontend
pkg/lang/python    Python frontend
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

Every frontend exposes the same three entry points, so consuming code is
frontend-agnostic.

CLI:

```sh
gast validate src/*.php         # exit 0 valid, 1 with diagnostics, 2 usage errors
gast tokens file.php            # one "kind text" pair per token
gast -list                      # available frontends
```

The frontend is picked by file extension:

| Extension | Frontend |
| --- | --- |
| `.php` | php |
| `.js`, `.mjs`, `.cjs` | javascript |
| `.ts` | typescript |
| `.html` | html |
| `.css` | css |
| `.scss` | scss |
| `.sass` | sass |
| `.py` | python |

Diagnostics render with position and caret:

```
ParseError at 2:7: expected ';' after statement, found end of input
2 | $a = 1
  |       ^
```

## Correctness and cross-validation

Every frontend's corpus under `pkg/lang/<lang>/testdata` is
cross-checked against the language's reference implementation before
being asserted: valid files must pass the reference and invalid files
must fail it.

| Frontend | Reference |
| --- | --- |
| php | `php -l` (PHP 8.5) |
| js | `node --check` (Node 24) |
| ts | `tsc --noEmit` (TypeScript 5) |
| css, scss, sass | `sass` compiler (dart-sass) |
| python | `python3 ast.parse` (CPython 3.14) |
| html | hand-designed strict rules (no strict reference validator exists) |

All lexers and parsers are single-pass with bounded lookahead, no
backtracking, and no rescanning; each package ships a `BenchmarkParse`.

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

## JavaScript coverage

- ES2024 script and module syntax: import/export in all forms, dynamic import, import.meta, new.target
- Regex-vs-division disambiguation, template literals with arbitrary nesting, numeric separators, BigInt, private names, hashbang
- Full ASI including restricted productions (return/throw/break/continue/yield, postfix ++/--, arrow bodies)
- Correct precedence and associativity (right-assoc `**` with the unary restriction, left-assoc `??` and ternary chains), optional chaining and nullish mixing rules
- Classes (static blocks, private members, computed keys), generators, async/await, destructuring everywhere, labeled statements, getters/setters, tagged templates
- Assignment-target early errors

## TypeScript coverage

- Full type grammar: unions/intersections, tuples with names and rest, mapped and conditional types with `infer`, template literal types, `keyof`/`typeof`/indexed access, generics with defaults and `const` type params, `import()` types
- Interfaces, type aliases, enums (incl. `const enum`), namespaces, `declare`, abstract classes, parameter properties, access modifiers, decorators
- `satisfies`, `as const`, non-null `!`, definite assignment `let x!: T`, overload signatures, `export =` / `import x = require()`
- `x as T` is fully supported; legacy `<T>x` assertions use a bounded heuristic (no TSX)

## HTML coverage

- WHATWG-subset tokenizer: doctypes, comments, bogus comments, CDATA, attribute quoting variants, raw-text elements (script/style) and RCDATA (title/textarea)
- Tree building with implicit closing rules (p, li, td/tr, dd/dt, option), void elements, synthesized html/head/body wrappers
- Strict failure on: unclosed elements at EOF, stray or mismatched end tags, unclosed attribute quotes, end tags with attributes, void elements with close tags, duplicate attributes, misplaced doctypes
- Entities stay raw; no foreign-content (SVG/MathML) rules

## CSS / SCSS / SASS coverage

- CSS Syntax Level 3 lexing (dimensions with exponents, custom properties as raw streams, url(), unicode-range, `!important`) and a selector subset (combinators, attribute operators, pseudo functions)
- At-rules: media, supports, import, charset, font-face, keyframes, page, layer, container, property, unknown rules
- SCSS adds nesting, `$variables` with `!default`/`!global`, `#{}` interpolation, `&`, mixins/functions with `@content`, control at-rules, `@extend`, `%placeholders`, `@use`/`@forward`, `@at-root`, `//` comments
- SASS is the same grammar on the indentation-based dialect (INDENT/DEDENT lexing, tab/space-mix and inconsistent-dedent errors)

## Python coverage

- Python 3.12/3.13: INDENT/DEDENT lexing, PEP 701 f-strings with nested quotes and format specs, all literal prefixes, walrus, match statements with the full pattern grammar, PEP 695 generics, async, decorators, comprehensions, starred targets
- Signature Python precedence: chained comparisons as one node, `-2**2 == -(2**2)`, right-associative power

Not enforced (compile-time, not parse-time, in CPython): symbol-table rules like `return` outside a function, duplicate parameters.

## Core (pkg/gast)

- `Span`, `Merge`, and `SourceMap` (byte offsets to 1-based line:column)
- `Cursor`: byte cursor with token-span tracking, `Peek`/`Advance`/`Match`/`MatchWord`, `SkipWhile`, and `Fail` returning a positioned `LexError`
- `TokenCursor`: token cursor with a synthetic end-of-input token, `Check`/`Expect`/`Fail`, `SpanFrom` for node spans
- `LexError`/`ParseError` implementing `Spanned`, and `FormatError` for caret diagnostics

## Adding a frontend

Create `pkg/lang/<name>` with the same shape as the existing ones: a
kind list (constants spelling each token text exactly once, with the
keyword and operator tables derived from them), a lexer over
`gast.Cursor`, an AST of nodes embedding `gast.Span`, a parser over
`gast.TokenCursor`, and `Parse`/`Validate`/`Tokenize` entry points.
Register it in `internal/cli`'s frontend table.

## Development

`just check` runs vet, lint, test, and build. The dev shell provides Go, gopls, golangci-lint, and just via the flake.

## Verification

- `go test ./...`: per-package unit tests plus corpus validation tests
- Corpora cross-checked against the reference implementations listed above
- `golangci-lint run`: clean under the strict config
