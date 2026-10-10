# control flow: if/elif/else, while/else, for/else, break, continue
if a:
    pass
elif b:
    pass
elif c:
    pass
else:
    pass

if x: pass
else: pass

n = 0
while n < 10:
    n += 1
    if n % 2:
        continue
    if n > 8:
        break
else:
    n = 0

for i in range(10):
    for j in range(i):
        if j > 3:
            break
else:
    i = 0

total = 0
for x, y in pairs:
    total += x * y
for (x, y) in pairs:
    total -= x
for k, v in d.items():
    total += v

while chunk := read(8192):
    process(chunk)
