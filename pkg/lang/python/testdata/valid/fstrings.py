# f-strings: PEP 701 nesting, format specs, conversions, debug
name = "world"
d = {"k": 1}
w = 8
x = 3.14159

a = f"hello {name}!"
b = f"{d['k']}"
c = f"{d["k"]}"
e = f"{x:.2f}"
g = f"{x:{w}.3f}"
h = f"{x:>{w}}"
i = f"{name!r}"
j = f"{name=}"
k = f"{name=!s}"
l = f"{name=:>10}"
m = f"{{literal braces}}"
n = F"upper prefix {x}"
o = rf"raw \{x}"
p = f"nested {f"{x}"} quote reuse"
q = f"""multi
line {x}
triple"""
r = f"expr {x + 1:>{w + 2}} spec"
s = "mixed " f"{x}" ' concat'
t = f"{x:{" >"}{w}}"
u = f"esc \n{x} \\ done"
