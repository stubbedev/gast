export {};

const input = "value";
const len = (input as string).length;
const narrowed = <string>input;
const chained = narrowed!.length;
const maybe: string | null = null;
const safe = maybe?.length ?? 0;
const mixed = maybe?.[0] ?? "fallback";
const strict = <number | null>len!;
const asConst = { a: 1, b: [2, 3] } as const;
const satisfiesShape = { a: 1 } satisfies { a: number };
const cast = len as unknown as string;
const generic = <T,>(x: T): T => x;
const applied = generic<number>(1);
const instantiated = generic<string>;
const comparison = 1 < 2 && 3 > 2 && 2 <= 2 && 3 >= 3;
const shift = 1 << 2 >> 1 >>> 0;
const optional = asConst.b?.[0] ?? 0;
const assertion = applied satisfies number satisfies number;
const nested = (<number>applied + 1) as number;
const conditional = comparison ? (len as number) : 0;

export default conditional;

console.log(len, narrowed, chained, safe, mixed, strict, asConst, satisfiesShape, cast, applied, instantiated, comparison, shift, optional, assertion, nested, conditional, generic);
