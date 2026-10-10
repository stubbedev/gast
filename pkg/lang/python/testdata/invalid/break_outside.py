# break and continue must sit inside a loop body, not its else clause
while cond():
    pass
else:
    break
