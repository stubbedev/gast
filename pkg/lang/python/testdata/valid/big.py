# big: a broad exercise of the grammar; also the benchmark target
from dataclasses import dataclass
from typing import Any


@dataclass(frozen=True)
class Record:
    name: str
    values: list[int] = field(default_factory=list)

    def total(self) -> int:
        return sum(v for v in self.values if v > 0)


records = [Record(f"r{i}", [i, i * 2, -i]) for i in range(20)]
lookup = {r.name: r.total() for r in records if r.total() > 0}


def process(items, *, limit=10, **options):
    processed = []
    for index, item in enumerate(items):
        match item:
            case int() as n if n < limit:
                processed.append(("small", n))
            case [str() as head, *tail]:
                processed.append(("list", head, len(tail)))
            case {"kind": kind, **rest}:
                processed.append(("map", kind, sorted(rest)))
            case Record(name=name, values=values):
                processed.append(("record", name, sum(values)))
            case _:
                processed.append(("other", item))
    return processed


class Registry[T]:
    def __init__(self):
        self._items: dict[str, T] = {}

    def add(self, key: str, item: T) -> "Registry[T]":
        self._items[key] = item
        return self

    def __getitem__(self, key: str) -> T:
        return self._items[key]

    def __iter__(self):
        return iter(self._items.items())


async def run(registry):
    async with session() as s:
        results = [item async for _, item in registry]
    try:
        await finalize(results)
    except* ValueError as group:
        for exc in group.exceptions:
            log(exc)
    finally:
        results = None


value = (
    ((1 + 2) * 3 ** 2 - 4 // 2)
    % 5
    + len([x for x in range(10) if x % 3 == 0])
)
text = f"{'nested ' + f'{value}':>{10 + 5}}"
matrix = [[i * j for j in range(5)] for i in range(5)]
transposed = [list(row) for row in zip(*matrix)]
sliced = matrix[1:4:2, ::-1]
packed = (*matrix, *transposed)
combined = {**{"a": 1}, "b": 2, **{"c": 3}}


def fib():
    a, b = 0, 1
    while True:
        yield a
        a, b = b, a + b


first_ten = [next(stream := fib()) for _ in range(10)]
del value, text
assert first_ten[0] == 0, "fibonacci starts at zero"
