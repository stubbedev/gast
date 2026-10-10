# del targets: names, attributes, subscripts, tuples
x = y = 1
d = {}
obj = type("Obj", (), {"attr": 1})()
seq = [0, 1, 2]

del x
del x, y
del (x, y)
del d["key"]
del obj.attr
del seq[0], seq[1:2]
