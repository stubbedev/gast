# match statements: the full pattern grammar
def classify(value):
    match value:
        case 1:
            return "one"
        case 2 | 3 | 4:
            return "small"
        case -5:
            return "negative"
        case 1 + 2j:
            return "complex"
        case "text" | b"bytes":
            return "literal"
        case None | True | False:
            return "singleton"
        case Point(x=0, y=0):
            return "origin"
        case Point(x, y):
            return "point"
        case [1, 2, *rest]:
            return "seq with head"
        case [*leading, last]:
            return "seq with tail"
        case (a, b):
            return "pair"
        case (a):
            return "group"
        case (a,):
            return "one tuple"
        case {"key": v, "other": w, **rest}:
            return "mapping"
        case Color.RED:
            return "value"
        case [x] as y:
            return "as"
        case str() as s if len(s) > 3:
            return "guard"
        case _:
            return "wildcard"


match x, y:
    case 0, 0:
        pass

match command.split():
    case [action]:
        pass
    case [action, obj]:
        pass
    case _:
        pass
