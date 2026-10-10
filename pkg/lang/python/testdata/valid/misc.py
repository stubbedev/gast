# misc: assignments, targets, del, asserts, globals, semicolons, yield
import sys

x = 1
x, y = 1, 2
x = y = z = 0
a, *rest = seq
*leading, last = seq
first, *mid, last = seq
x += 1
x -= 2
x *= 3
x /= 4
x //= 5
x %= 6
x @= 7
x &= 1
x |= 2
x ^= 3
x <<= 4
x >>= 5
x **= 6
obj.attr = 1
obj.attr.nested = 2
d["key"] = 3
d[1:2] = 4
d[::2] = []
(x) = 5
(x,) = (6,)
[x, y] = [7, 8]
x[0], y[1:2] = 9, 10

count: int = 0
name: str
obj.attr: int = 1
d["k"]: int

pass; x = 1; y = 2; print(x, y);

del x
del x, y
del d["key"]
del obj.attr
del seq[0], seq[1:2]

assert condition
assert condition, "message"

global counter


def outer():
    binding = 1

    def inner():
        nonlocal binding
        return binding


def gen():
    yield
    yield 1
    yield 1, 2
    yield from range(10)


t = 1,
t = 1, 2
t = ()
t = (1,)
t = (1, 2)
lst = []
lst = [1, *t, 2]
st = {1, 2, 3}
st = {*t, 4}
dd = {}
dd = {"a": 1, "b": 2, **dd}
dd = dict(a=1)

s = x[1:2:3, ::-1]
s = x[1:]
s = x[:2]
s = x[::2]
s = x[:, 1]
s = x[a, b, c]
s = x[x:y, ...]

w = x @ y
m = (a @ b) @ c

ternary = "yes" if flag else "no"
chain = a if b else c if d else e


def consumer():
    result = (yield value)
    return result
