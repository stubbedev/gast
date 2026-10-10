import type { ID as Identifier } from "./type_aliases";
import type Default from "./hello";
import type * as NS from "./variables";
import { type Inline, regular } from "./type_aliases";
import * as everything from "./generics";
import defaultExport, { named as renamed } from "./interfaces";
import sideEffectOnly from "./assertions";

export type { Identifier };
export type * from "./type_aliases";
export { Inline };
export { regular as irregular } from "./type_aliases";
export * from "./generics";
export * as star from "./tuples";

const dynamic: typeof import("./type_aliases") = await import("./type_aliases");

console.log(everything, defaultExport, renamed, sideEffectOnly, regular, dynamic);
