const o = {
  shorthand,
  key: value,
  [computed]: 1,
  "string key": 2,
  42: 3,
  method() { return this.key; },
  async am() { return await p; },
  *gen() { yield 1; },
  async *agen() { yield await p; },
  get accessor() { return 1; },
  set accessor(v) {},
  ...spreadSource,
};
({ a, b } = pair);
({ a: renamed, b: nested.deep = 1, ...others } = pair);
[a, , b, ...rest] = list;
