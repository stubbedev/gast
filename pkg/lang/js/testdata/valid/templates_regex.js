const name = "world";
const plain = `no substitutions`;
const interp = `hello ${name}`;
const nested = `outer ${`inner ${name}!`} end`;
const deep = `a ${`b ${`c ${name}`}`} d`;
const exprs = `sum: ${1 + 2} and ${`x${y}`}`;
const raw = String.raw`c:\temp\new`;
const multiline = `line 1
line 2`;
tag`a ${1} b ${2} c`;
function tag(strings, ...values) { return [strings, values]; }
const re1 = /ab+c/gi;
const re2 = /[\/]/;
const re3 = /a\/b/;
const div1 = a / b / c;
const div2 = total / count;
const re4 = x === 1 ? /yes/ : /no/;
