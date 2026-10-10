# starred targets in their legal shapes
*rest, = items
head, *mid, tail = items
(a, *b) = items
(a, (b, *c)) = items
[*spread] = items
*leading, last = items

# starred values are fine inside displays and tuples
mixed = [1, *rest, 2]
spread = (*rest, 3)
unpack = {*rest, 4}
sliced = items[*rest]
