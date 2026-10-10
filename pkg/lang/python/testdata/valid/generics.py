# PEP 695 generics and the type statement
type Simple = int
type Param[T] = list[T]
type Bound[T: int] = dict[str, T]
type Multi[A, B: str, C = bytes] = tuple[A, B, C]
type Nested[T] = list[Nested[list[T]]]


class Box[T]:
    def __init__(self, item: T) -> None:
        self.item = item


def identity[T](value: T) -> T:
    return value


def convert[A, B: str](source: A) -> B:
    return B(source)


def defaulted[T=int, U=float](a: T, b: U) -> tuple[T, U]:
    return a, b


alias = Box[int]
x: type[Simple] = Simple
