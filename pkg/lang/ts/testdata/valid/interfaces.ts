export {};

interface Shape {
  readonly name: string;
  area(): number;
  scale?(factor: number): this;
}

interface Point {
  x: number;
  y: number;
}

interface Named extends Point, Shape {
  describe(): string;
  [key: string]: unknown;
  (input: string): boolean;
  new (x: number, y: number): Named;
  get origin(): Point;
  set origin(point: Point);
}

interface Callable {
  <T>(value: T): T[];
  (a: number): number;
}

interface Constructor {
  new <T>(value: T): Box<T>;
}

interface Box<T> {
  value: T;
  empty: boolean;
  unwrap(fallback: T): T;
}

interface AbstractFactory {
  new (): object;
}

declare interface Ambient {
  kind: "ambient";
}

export const named = 1;
export default named;
const callable: Callable = ((value: unknown) => [value]) as Callable;
const box: Box<number> = { value: 1, empty: false, unwrap: (fallback) => fallback };
const shape: Shape = { name: "s", area: () => 1 };
console.log(callable, box, shape);
