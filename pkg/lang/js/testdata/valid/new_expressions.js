const a = new Foo;
const b = new Foo();
const c = new Foo(1, 2, ...rest);
const d = new ns.Foo.Bar();
const e = new (factory())();
const f = new new Foo()()();
function who() { return new.target; }
const g = who;
const h = Foo.prototype.bar;
const i = obj?.deep?.chain?.();
const j = obj?.[0];
const k = obj?.prop;
const l = import.meta.url;
const m = import("dynamic");
const n = import(specifier, { with: { type: "json" } });
class Sub extends Base { constructor() { super(1); } }
