# functions: parameter forms, annotations, decorators, nested defs
def simple():
    return 1


def positional(a, b=2, /, c=3):
    return a + b + c


def keywords(*, key=1, other):
    return key + other


def variadic(*args, **kwargs):
    return args, kwargs


def mixed(a, b=1, /, c=2, *args, d, e=5, **kwargs):
    return (a, b, c, args, d, e, kwargs)


def kwonly_after_star(a, *, b):
    return a + b


def annotated(x: int, y: str = "s") -> bool:
    return True


def no_return():
    pass


def early():
    return
    return 1, 2


@decorator
def decorated():
    pass


@decorator(arg=1)
@other
async def async_decorated():
    pass


def nested():
    def inner():
        nonlocal_var = 1

        def deepest():
            return nonlocal_var

        return deepest

    return inner


result = lambda: 0
result = lambda x: x + 1
result = lambda x, y=2, *args, z, **kw: (x, y, args, z, kw)
