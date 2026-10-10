export {};

function add(a: number, b: number): number {
  return a + b;
}

function greet(name: string, title?: string): string {
  return title ? `${title} ${name}` : name;
}

function sum(...values: number[]): number {
  return values.reduce(add, 0);
}

function log(this: Console, message: string): void {
  this.log(message);
}

function make<T>(value: T): (flag: boolean) => T {
  return function (flag: boolean): T {
    if (!flag) throw new Error("no");
    return value;
  };
}

// Overload signatures followed by an implementation.
function pick(source: string, index: number): string;
function pick(source: number[], index: number): number;
function pick(source: string | number[], index: number): string | number {
  return source[index];
}

async function later(value: number): Promise<number> {
  return Promise.resolve(value);
}

const arrow = (x: number, y = 2, ...rest: number[]): number => x + y + rest.length;
const typed = (x: string): string => x.trim();
const generic = <T,>(value: T): T => value;
const asyncArrow = async (x: number): Promise<number> => x + 1;

declare function ambient(x: number): string;

log.call(console, pick("abc", 1));
console.log(add(1, 2), greet("a"), sum(1, 2, 3), make(1)(true), later, arrow(1), typed("x"), generic(2), asyncArrow, ambient);
