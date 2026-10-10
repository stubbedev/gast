export {};

interface Comparable<T> {
  compareTo(other: T): number;
}

interface Serializable {
  serialize(): string;
}

function logged<T>(target: T): T {
  return target;
}

@logged
class Base<T> implements Serializable {
  protected readonly id: string;
  static count: number = 0;
  #secret: number = 1;

  constructor(id: string, private readonly label: string, public value: T) {
    this.id = id;
    Base.count++;
  }

  compareTo(other: Base<T>): number {
    return this.label.localeCompare(other.label);
  }

  serialize(): string {
    return `${this.id}:${this.label}`;
  }

  get display(): string {
    return this.label;
  }

  set display(value: string) {
    console.log(value);
  }

  method<U>(input: U): [T, U] {
    return [this.value, input];
  }
}

class Factory<T> extends Base<T[]> {
  static build<T>(items: T[]): Factory<T[]> {
    return new Factory("factory", "base", [items]);
  }
}

abstract class Shape {
  abstract area(): number;
  abstract readonly sides: number;
  describe(): string {
    return `shape with ${this.sides} sides`;
  }
}

class Square extends Shape {
  readonly sides = 4;
  constructor(private size: number) {
    super();
  }
  area(): number {
    return this.size ** 2;
  }
}

class Overloads {
  pick(source: string): number;
  pick(source: number[]): number;
  pick(source: string | number[]): number {
    return typeof source === "string" ? source.length : source[0];
  }
}

class WithIndex {
  [key: string]: string | number;
  length = 0;
}

class Decorated {
  @logged temperature: number = 25;

  @logged
  method(param: string): string {
    return param;
  }
}

declare class Ambient {
  method(): void;
}

const square = new Square(2);
const factory = Factory.build([1]);
const base = new Base<string>("a", "b", "c");
console.log(Shape, Overloads, WithIndex, Decorated, Ambient, square.describe(), factory, base.serialize());
