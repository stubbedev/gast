function f(a, b = 2, ...rest) { return a + b; }
function* gen(a) {
  yield a;
  yield* other(a);
  const x = yield;
}
const anon = function named(n) { return n; };
const iife = (function () { return 1; })();
(async function () { await tick(); })();
const arrow = (a, b) => a + b;
const single = x => x * 2;
const asyncArrow = async (a, b) => await combine(a, b);
const asyncSingle = async x => x;
const blockArrow = (n) => { return n; };
const destructure = ({ a, b: { c = 1 }, d = 2 }, [e, , f, ...g]) => a + c + e;
const nested = (a = (b = 1) => b) => a();
function curried(a) {
  return function (b) {
    return function* (c) {
      yield a + b + c;
    };
  };
}
