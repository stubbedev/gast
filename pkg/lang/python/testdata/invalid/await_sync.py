# a sync def is not an async scope, not even for its defaults
def f(x=await y):
    return x
