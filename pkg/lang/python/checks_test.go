package python

import (
	"testing"
)

// TestSymbolChecks runs one source per rule through Validate; each
// invalid source carries CPython's exact compile-time message and each
// valid source a legal form that is easy to reject by accident.
func TestSymbolChecks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		// return / yield placement
		{"return at module", "return 1\n", "'return' outside function"},
		{"return in class", "class C:\n    return 1\n", "'return' outside function"},
		{"return in function", "def f():\n    return 1\n", ""},
		{"yield at module", "yield 1\n", "'yield' outside function"},
		{"yield in class", "class C:\n    yield\n", "'yield' outside function"},
		{"yield from at module", "yield from x\n", "'yield from' outside function"},
		{"yield in lambda", "def f():\n    return lambda: (yield)\n", ""},
		{"yield in comprehension element", "def f():\n    return [(yield) for x in y]\n", "'yield' inside comprehension"},
		{"yield in first iterable", "def f():\n    return [x for x in (yield)]\n", ""},

		// break / continue placement
		{"break outside loop", "break\n", "'break' outside loop"},
		{"continue outside loop", "continue\n", "'continue' not properly in loop"},
		{"break in loop else", "for x in y:\n    pass\nelse:\n    break\n", "'break' outside loop"},
		{"continue in while else", "while x:\n    pass\nelse:\n    continue\n", "'continue' not properly in loop"},
		{"break in try finally in loop", "for x in y:\n    try:\n        break\n    finally:\n        pass\n", ""},
		{"break in with in loop", "for x in y:\n    with a:\n        break\n", ""},
		{"break in nested loop", "for x in y:\n    for z in w:\n        break\n", ""},
		{"break in def inside loop", "for x in y:\n    def f():\n        break\n", "'break' outside loop"},

		// duplicate parameters
		{"duplicate plain", "def f(x, x): pass\n", "duplicate argument \"x\" in function definition"},
		{"duplicate positional-only", "def f(x, /, x): pass\n", "duplicate argument \"x\" in function definition"},
		{"duplicate keyword-only", "def f(*, x, x): pass\n", "duplicate argument \"x\" in function definition"},
		{"duplicate via vararg", "def f(x, *x): pass\n", "duplicate argument \"x\" in function definition"},
		{"duplicate via kwarg", "def f(x, **x): pass\n", "duplicate argument \"x\" in function definition"},
		{"duplicate across sections", "def f(x, /, y, *, y): pass\n", "duplicate argument \"y\" in function definition"},
		{"duplicate in lambda", "lambda x, x: 1\n", "duplicate argument \"x\" in function definition"},
		{"distinct sections", "def f(a, /, b, *c, d, **e): pass\n", ""},

		// __debug__ and keyword targets
		{"assign to debug", "__debug__ = 1\n", "cannot assign to __debug__"},
		{"aug-assign to debug", "__debug__ += 1\n", "cannot assign to __debug__"},
		{"annotate debug", "__debug__: int\n", "cannot assign to __debug__"},
		{"del debug", "del __debug__\n", "cannot delete __debug__"},
		{"for target debug", "for __debug__ in x: pass\n", "cannot assign to __debug__"},
		{"walrus debug", "(__debug__ := 1)\n", "cannot assign to __debug__"},
		{"assign to None", "None = 1\n", "cannot assign to literal"},
		{"global debug", "def f():\n    global __debug__\n", ""},

		// starred targets
		{"multiple stars", "a, *b, *c = x\n", "multiple starred expressions in assignment"},
		{"bare star", "*a = x\n", "starred assignment target must be in a list or tuple"},
		{"bare star in for", "for *a in x: pass\n", "starred assignment target must be in a list or tuple"},
		{"paren star target", "(*b) = x\n", "cannot use starred expression here"},
		{"paren star element", "a, (*b) = x\n", "cannot use starred expression here"},
		{"paren star in for", "for (*a) in x: pass\n", "cannot use starred expression here"},
		{"starred value", "x = (*b)\n", "cannot use starred expression here"},
		{"trailing comma star", "*a, = x\n", ""},
		{"star in paren tuple", "(a, *b) = x\n", ""},
		{"star in list target", "[*a] = x\n", ""},
		{"nested plain star", "(a, (b, *c)) = x\n", ""},
		{"star in display value", "x = [*b, 1]\n", ""},
		{"star in subscript", "x = a[*b]\n", ""},

		// await and async constructs
		{"await at module", "await x\n", "'await' outside function"},
		{"await in sync def", "def f():\n    await x\n", "'await' outside async function"},
		{"await in lambda in async", "async def f():\n    lambda: await x\n", "'await' outside async function"},
		{"await in sync default", "def f(x=await y): pass\n", "'await' outside function"},
		{"await in async default", "async def f(x=await y): pass\n", "'await' outside function"},
		{"await in async comp", "async def f():\n    return [await x for x in y]\n", ""},
		{"await in comp in sync def", "def f():\n    return [await x for x in y]\n", "'await' outside async function"},
		{"await through class", "async def f():\n    class C:\n        await x\n", "'await' outside function"},
		{"async comp at module", "[x async for x in y]\n", "asynchronous comprehension outside of an asynchronous function"},
		{"async comp in sync def", "def f():\n    return [x async for x in y]\n", "asynchronous comprehension outside of an asynchronous function"},
		{"async comp in lambda in async", "async def f():\n    lambda: [x async for x in y]\n", "asynchronous comprehension outside of an asynchronous function"},
		{"nested async comp", "async def f():\n    return [[x async for x in y] for z in w]\n", ""},
		{"async for statement in sync def", "def f():\n    async for x in y: pass\n", "'async for' outside async function"},
		{"async with statement in sync def", "def f():\n    async with x: pass\n", "'async with' outside async function"},

		// import placement
		{"star import in def", "def f():\n    from x import *\n", "import * only allowed at module level"},
		{"star import in class", "class C:\n    from x import *\n", "import * only allowed at module level"},
		{"star import in if at module", "if x:\n    from x import *\n", ""},
		{"future after statement", "x = 1\nfrom __future__ import annotations\n", "from __future__ imports must occur at the beginning of the file"},
		{"future after other import", "from x import y\nfrom __future__ import annotations\n", "from __future__ imports must occur at the beginning of the file"},
		{"future in function", "def f():\n    from __future__ import annotations\n", "from __future__ imports must occur at the beginning of the file"},
		{"future after docstring", "\"doc\"\nfrom __future__ import annotations\n", ""},
		{"future after docstring then code", "\"doc\"\nfrom __future__ import annotations\nx = 1\nfrom __future__ import division\n", "from __future__ imports must occur at the beginning of the file"},
		{"future with alias", "from __future__ import annotations as m\n", ""},
		{"f-string docstring is not a docstring", "f\"doc\"\nfrom __future__ import annotations\n", "from __future__ imports must occur at the beginning of the file"},

		// global / nonlocal
		{"nonlocal at module", "nonlocal x\n", "nonlocal declaration not allowed at module level"},
		{"nonlocal without binding", "def f():\n    nonlocal x\n", "no binding for nonlocal \"x\" found"},
		{"nonlocal with later binding", "def f():\n    def g():\n        nonlocal x\n    x = 1\n", ""},
		{"nonlocal across class boundary", "def f():\n    x = 1\n    class C:\n        nonlocal x\n", ""},
		{"class binding is not nonlocal-visible", "def f():\n    class C:\n        x = 1\n    def g():\n        nonlocal x\n", "no binding for nonlocal \"x\" found"},
		{"global hides nonlocal binding", "def a():\n    x = 1\n    def b():\n        global x\n        def c():\n            nonlocal x\n", "no binding for nonlocal \"x\" found"},
		{"parameter and global", "def f(x):\n    global x\n", "name \"x\" is parameter and global"},
		{"parameter and nonlocal", "def f(x):\n    nonlocal x\n", "name \"x\" is parameter and nonlocal"},
		{"assigned before global", "def f():\n    x = 1\n    global x\n", "name \"x\" is assigned to before global declaration"},
		{"augmented before global", "def f():\n    x += 1\n    global x\n", "name \"x\" is assigned to before global declaration"},
		{"global then assign", "def f():\n    global x\n    x = 1\n", ""},
		{"global and nonlocal", "def f():\n    global x\n    nonlocal x\n", "name \"x\" is nonlocal and global"},
		{"nonlocal and global", "def f():\n    nonlocal x\n    global x\n", "name \"x\" is nonlocal and global"},
		{"global repeated", "def f():\n    global x, x\n", ""},
		{"annotated then global", "def f():\n    x: int = 1\n    global x\n", "annotated name \"x\" can't be global"},
		{"global then annotated", "def f():\n    global x\n    x: int = 1\n", "annotated name \"x\" can't be global"},
		{"annotated then nonlocal", "def f():\n    x: int\n    nonlocal x\n", "annotated name \"x\" can't be nonlocal"},

		// annotation, augmented, and del targets handled by the parser
		{"annotate tuple", "(a, b): int\n", "only a single target can be annotated"},
		{"annotate attribute", "x.y: int\n", ""},
		{"aug-assign to literal", "1 += 1\n", "cannot assign to literal"},
		{"aug-assign to call", "f() += 1\n", "cannot assign to function call"},
		{"aug-assign to paren name", "(a) += 1\n", ""},
		{"del call", "del f()\n", "cannot delete function call"},

		// module docstring edge
		{"yield at module in tuple", "x = (yield)\n", "'yield' outside function"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(test.source)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %q, got none", test.wantErr)
			}
			if err.Error() != test.wantErr {
				t.Fatalf("error %q, want %q", err.Error(), test.wantErr)
			}
		})
	}
}
