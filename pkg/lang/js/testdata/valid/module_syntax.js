import defaultExport from "module-a";
import * as namespace from "module-b";
import { named, renamed as local } from "module-c";
import def, { a, b as c } from "module-d";
import def2, * as ns2 from "module-e";
import "side-effect-only";
import { "strange name" as ok } from "module-f";
export const value = 1;
export var mutable = 2;
export let reassigned = 3;
export function fn() {}
export class Klass {}
export async function afn() {}
export { mutable as alias };
export { unrelated } from "module-g";
export * from "module-h";
export * as bundle from "module-i";
export { a as "renamed export" } from "module-j";
