export {};

const empty: object = {};
const bag: { [key: string]: number; known: number } = { known: 1 };
bag.extra = 2;

const object: object = empty as { a?: number };
const template = `a${1 + 1}b${"c"}d`;
const tagged = String.raw`raw\n${template}`;
const regex = /ab+c/gi;
const division = 4 / 2 / 1;
const generator = function* (start: number): Generator<number, void, void> {
  yield start;
  yield* [1, 2];
  return;
};

async function* aggro(): AsyncGenerator<number> {
  yield 1;
}

const yielder = generator(0);
const first = yielder.next();
const proto = { base: 1 };
const heir = Object.create(proto);
const cls = class Inline extends Object {};
const anon = class {};

console.log(empty, bag, object, template, tagged, regex, division, yielder, first, proto, heir, cls, anon, aggro, generator);
