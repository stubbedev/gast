# nonlocal finds bindings made anywhere in an enclosing function
def outer():
    bound = 1

    def later_reader():
        def inner():
            nonlocal bound
            bound += 1
        return inner
    return later_reader()


def binds_late():
    def inner():
        nonlocal target
        target = 2
    target = 1
    inner()
    return target


def other_bindings():
    import sys as system
    for loop_name in []:
        pass
    with open("f") as ctx_name:
        pass
    try:
        pass
    except ValueError as exc_name:
        pass
    del exc_name

    def inner():
        nonlocal system, loop_name, ctx_name, exc_name
    return inner


def redeclared():
    global shared, shared
    shared = 1


def debug_global():
    global __debug__
    return __debug__
