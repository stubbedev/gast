# an annotated name cannot be declared global afterwards
def f():
    x: int = 1
    global x
