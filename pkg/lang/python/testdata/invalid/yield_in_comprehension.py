# yield cannot appear inside a comprehension's own scope
def f():
    return [(yield) for x in y]
