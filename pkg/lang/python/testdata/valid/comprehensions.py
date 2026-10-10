# comprehensions: list, set, dict, generator, async, nested
squares = [x * x for x in range(10)]
evens = [x for x in range(20) if x % 2 == 0 if x > 4]
flat = [item for row in matrix for item in row if item]
unique = {x % 5 for x in data}
lookup = {k: v for k, v in items}
lookup2 = {k: v for k, v in items if v}
gen = (x * 2 for x in range(5))
first = sum(x for x in range(5) if x)
nested = [[y for y in row] for row in grid]
cond = [x if x else 0 for x in values]
walrus_in_if = [y for x in data if (y := f(x)) is not None]


async def async_comps():
    async_comp = [x async for x in aiter()]
    async_gen = (x async for x in aiter() if x)
    aset = {x async for x in aiter()}
    adict = {x: x async for x in aiter()}
    return async_comp, async_gen, aset, adict
