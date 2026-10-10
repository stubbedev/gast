export {};

enum Direction {
  Up,
  Down,
  Left = 10,
  Right,
}

const enum Fast {
  A,
  B = A + 1,
  C = "c",
  D = B * 2,
}

enum Stringy {
  Yes = "yes",
  No = "no",
}

enum Mixed {
  Zero,
  Word = "word",
  After = 5,
}

enum Computed {
  A = 1,
  B = A + 1,
  C = "c".length,
}

declare enum Ambient {
  Member,
}

console.log(Direction.Up, Fast.C, Stringy.Yes, Mixed.After, Computed.C, Ambient.Member);
