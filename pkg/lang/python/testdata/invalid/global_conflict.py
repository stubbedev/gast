# a name cannot be both global and nonlocal, nor assigned first
def f():
    x = 1
    global x
