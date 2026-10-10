# parameter lists: positional-only, keyword-only, variadics, defaults
def full(pos_only, /, plain, default=1, *args, kw_only, kw_default=2, **kwargs):
    return pos_only, plain, args, kwargs


def annotated(x: int = 0, *rest: int, **named: int) -> int:
    return x


async def apipe(a, b=calls(), *, key=None):
    return a


square = lambda x, /, y=2, *args, z, **kw: x * x
