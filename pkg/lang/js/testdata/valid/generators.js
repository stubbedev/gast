function* fib() {
  let [a, b] = [0, 1];
  while (true) {
    yield a;
    [a, b] = [b, a + b];
  }
}
function* delegate() {
  yield* fib();
  yield* [1, 2, 3];
  yield* "abc";
}
const g = delegate();
const first = g.next().value;
const expr = function* () { yield 1; };
const obj = { *m() { yield this; } };
class C { *gen() { yield* other(); } }
