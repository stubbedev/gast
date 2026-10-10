# yield belongs to functions, including lambdas and defaults
def gen():
    yield
    yield 1, 2
    yield from range(3)
    sent = yield "ready"
    return (yield from tail())


def reader():
    items = [x for x in (yield provider())]
    fn = lambda: (yield)
    return items, fn


async def apipe():
    value = (yield)
    return
