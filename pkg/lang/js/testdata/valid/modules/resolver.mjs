import { resolve } from "./helpers.mjs";
export const url = await resolve("./data.json");
export default async function main(paths) {
  const out = [];
  for await (const p of paths) {
    out.push(await resolve(p));
  }
  return out;
}
