if (a) { b(); } else if (c) d(); else e();
while (cond) { if (done) break; else continue; }
do { body(); } while (cond);
for (let i = 0, j = 9; i < j; i++, j--) step(i, j);
for (const key in obj) use(key);
for (const item of list) use(item);
outer: for (const a of as) {
  inner: for (const b of bs) {
    if (skip(b)) continue outer;
    if (stop(a)) break outer;
  }
}
switch (pick) {
  case 1:
  case 2:
    one();
    break;
  case x:
    two();
  default:
    rest();
}
try { risky(); } catch (e) { handle(e); } finally { cleanup(); }
try { a(); } catch { b(); } finally { c(); }
with (Math) { r = cos(PI); }
debugger;
