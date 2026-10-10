export {};

let a: string = "x";
let b: number = 1.5e3;
let c: bigint = 10n;
let d: boolean = false;
let e: null = null;
let f: undefined = undefined;
let g: symbol = Symbol("s");
let h: object = {};
let i: any = null;
let j: unknown = a;
let k: never = (() => {
  throw new Error("unreachable");
})();
let l: void = undefined;
let m!: string;
const n = "literal" as const;
let o = "raw" satisfies string;
m = n;
console.log(a, b, c, d, e, f, g, h, i, j, k, l, m, n, o);
