# comprehensions are their own scopes; class bodies are not functions
class Registry:
    keys = [key for key in source]
    lookup = {key: idx for idx, key in enumerate(keys)}


module_level = [x for x in range(3)]
flat = [item for row in grid for item in row if item]
first = sum(x * 2 for x in module_level)


def scoped():
    return [y for x in data if (y := f(x)) is not None]
