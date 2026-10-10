# strings: prefixes, triples, escapes, concatenation
a = 'single'
b = "double"
c = 'it\'s'
d = "tab\there\n"
e = "hex \x41 unicode \u00e9 big \U0001F600 octal \101"
f = 'raw\\n' + r'raw\n'
g = b'bytes'
h = B"more bytes"
i = rb'raw bytes\n'
j = u'unicode prefix'
k = """triple
quoted with "quotes" and 'quotes'"""
l = '''also triple'''
m = 'adjacent ' "concatenated" 'literals'
n = ('parenthesized '
     'continuation')
o = "\N{GREEK SMALL LETTER ALPHA}"
