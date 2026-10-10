# nonlocal needs a binding in an enclosing function scope
def outer():
    def inner():
        nonlocal missing
    return inner
