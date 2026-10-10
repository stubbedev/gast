class Base { base() { return 1; } }
class Point extends Base {
  static origin = new Point(0, 0);
  #x;
  #y;
  static #count = 0;
  constructor(x = 0, y = 0) {
    super();
    this.#x = x;
    this.#y = y;
  }
  get x() { return this.#x; }
  set x(value) { this.#x = value; }
  static get count() { return Point.#count; }
  #norm() { return Math.hypot(this.#x, this.#y); }
  async load() { return fetch(this.#norm()); }
  *walk() { yield this.#x; yield this.#y; }
  static {
    Point.#count = 0;
  }
  ["com" + "puted"]() {}
  3() {}
  "quoted"() {}
}
const Anonymous = class extends Base { m() {} };
const Named = class NamedExpr { m() {} };
new class { run() { return 1; } }().run();
