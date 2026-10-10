const [a] = arr;
const [b = 1] = arr;
const [[c], [d = 2]] = pairs;
const { e } = obj;
const { f = 1 } = obj;
const { g: h } = obj;
const { i: j = 1 } = obj;
const { k: { l } } = obj;
const { m: [n] } = obj;
const { ["o" + "p"]: q } = obj;
const { r, ...others } = obj;
const [...tail] = arr;
const [, second] = arr;
function params({ a, b: { c } }, [d], e = 1, ...f) {}
try { throw 1; } catch ({ message }) {}
for (const [key, value] of entries) {}
