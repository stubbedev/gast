export {};

const greeting: string = "hello, typescript";
let count: number = 0;
let enabled = true;
const big: bigint = 42n;

function main(): void {
  console.log(`${greeting} ${count} ${enabled} ${big}`);
  count += 1;
}

export default main;
main();
