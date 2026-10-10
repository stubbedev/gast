export {};

namespace Outer.Inner {
  export const shared: number = 1;

  export function helper<T>(value: T): T {
    return value;
  }

  export namespace Deepest {
    export const flag = true;
  }
}

namespace Dotted.Name.Space {
  export type Alias = string;
  export const tagged = "dotted";
}

declare namespace Ambient {
  export const version: string;
  export function run(): void;
  namespace Nested {
    export const depth: number;
  }
}

console.log(Outer.Inner.shared, Outer.Inner.Deepest.flag, Dotted.Name.Space.tagged, Outer.Inner.helper(1), Ambient);
