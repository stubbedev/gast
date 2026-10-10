export {};

type Tuple = [number, string];
type Named = [x: number, y: number];
type Optional = [first?: string, ...rest: number[]];
type Mixed = [a: number, b?: string, ...c: boolean[]];
type RoArray = readonly number[];
type RoTuple = readonly [number, string];
type Nested = [[number, number], [string, string]];
type ArrayOf = Array<Tuple>;
type Indexed = Tuple[0];
type Long = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];

const pair: Tuple = [1, "a"];
const named: Named = [1, 2];
const values: RoArray = [1, 2, 3];
const matrix: Nested = [[1, 2], ["a", "b"]];
const withRest: Mixed = [1, "x", true, false];

function swap([a, b]: Tuple): [string, number] {
  return [b, a];
}

function destructure({ x, y: renamed = 0 }: { x: number; y?: number }): number {
  return x + renamed;
}

console.log(pair, named, values, matrix, withRest, swap(pair), destructure({ x: 1 }));
