import { fileURLToPath } from "node:url";
export async function resolve(spec) {
  return new URL(spec, import.meta.url).href;
}
export function sync(spec) {
  return fileURLToPath(resolve(spec));
}
