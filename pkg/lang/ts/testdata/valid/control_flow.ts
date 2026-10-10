export {};

function check(score: number): string {
  if (score < 50) {
    return "low";
  } else if (score < 80) {
    return "medium";
  }
  return "high";
}

for (let i = 0; i < 3; i++) {
  console.log(i);
}

for (const key in { a: 1 }) {
  console.log(key);
}

for (const item of [1, 2, 3] as const) {
  console.log(item);
}

async function drain(): Promise<void> {
  for await (const chunk of []) {
    console.log(chunk);
  }
}

switch (check(75)) {
  case "low":
  case "medium":
    break;
  default:
    break;
}

try {
  throw new Error("boom");
} catch (error: unknown) {
  console.log(String(error));
} finally {
  console.log("done");
}

const label = check(10);
loop: while (true) {
  break loop;
}

do {
  console.log(label);
} while (false);

const matrix: number[][] = [[1]];
const spread = [...matrix[0], ...[2]];
const object = { ...spread, computed: matrix.length };
console.log(check, drain, matrix, spread, object);
