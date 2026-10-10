export {};

export type ID = string | number;
export type Inline = number;
export const regular = 1;
type Maybe<T> = T | null | undefined;
type Pair = { first: number; second: number };
type Both = Pair & { extra?: boolean };
type Keys = keyof Pair;
type Coord = Pair["first"];
type Checked = ID extends string ? "yes" : "no";
type Flatten<T> = T extends Array<infer U> ? U : T;
type Limited = Flatten<number[]>;
type Template = `${ID}-${"a" | "b"}`;
type NegLiteral = -1;
type BigLiteral = 9007199254740991n;
type TrueLit = true;
type StrLit = "quoted";
type Query = typeof String;
declare const uniqueSym: unique symbol;
type Deep = { nested: { deeper: Array<Keys> } };
type Imported = typeof import("./modules");
type Refined<T> = T extends (infer U extends number) ? U : never;
type MappedAll = { [K in keyof Pair as `get${string & K}`]: () => number };
type Partialish = { [K in keyof Pair]?: Pair[K] };
type Grow = { +readonly [K in keyof Pair]-?: Pair[K] };
type Shrink = { -readonly [K in keyof Pair]+?: Pair[K] };
type Rec = { [K in ID]: ID extends string ? number : bigint };
type Merged = (string | number) & (number | boolean);
type Fn = (x: number, y?: number, ...rest: number[]) => number;
type Ctor = new (x: number) => object;
type AbstractCtor = abstract new (x: number) => object;
type Predicate = (x: unknown) => x is string;
type Asserting = (x: unknown) => asserts x is string;
type ThisPredicate = (x: ThisPredicate) => x is ThisPredicate;
type Nested = Merged extends string | number ? ID : Keys;
type WithDefault<T = ID> = T;
type Constrained<T extends object = Pair> = keyof T;
interface Variance<in out T> {
  m(x: T): void;
}

declare const value: MappedAll;
const constrained: Constrained = "first";
const variance: Variance<string> = { m: (x) => x };
console.log(value, constrained, variance, uniqueSym);
