# break/continue keep loop context through try/finally and with
for item in items:
    try:
        break
    finally:
        cleanup()
    continue


while flag:
    with open(path) as fh:
        if fh:
            continue
        break
else:
    for other in rest:
        break


def f():
    total = 0
    for x in xs:
        for y in ys:
            break
        else:
            continue
        total += x
    return total
