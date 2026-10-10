export {};

function first<T>(items: T[]): T {
  return items[0];
}

const pair = <A, B>(a: A, b: B): [A, B] => [a, b];

const bound = first<string>(["a"]);
const nested = first<string[]>([["b"]]);
const mixed = pair<number, string>(1, "one");

const map = new Map<string, number[]>([["a", [1]]]);
const set = new Set<number>([1, 2, 3]);
const readonly: ReadonlyArray<string> = ["a"];
const frozen: readonly string[] = ["b"];

const identity = <T extends object | null = {}>(value: T): T => value;
const constant = <const T>(value: T): T => value;

async function lookup(): Promise<Map<string, number[]>> {
  return map;
}

const nestedGenerics: Array<Array<number>> = [[1], [2, 3]];

const objectMethod = {
  value: 1,
  get<T>(key: string, fallback: T): number | T {
    return key in map ? 1 : fallback;
  },
};

console.log(bound, nested, mixed, map, set, readonly, frozen, identity({}), constant(1), lookup, nestedGenerics, objectMethod.get("x", 0));
