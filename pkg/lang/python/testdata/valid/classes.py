# classes: bases, keywords, methods, decorators, generics
class Empty:
    pass


class Base:
    attr = 1

    def method(self):
        return self.attr


class Derived(Base):
    def method(self):
        return super().method() + 1


class WithKeywords(Base, metaclass=Meta, **kwargs):
    Constant = "c"

    @staticmethod
    def factory():
        return WithKeywords()

    @classmethod
    def create(cls):
        return cls()

    @property
    def value(self):
        return self.Constant


class StarBases(*bases):
    pass


@register
class Decorated(Base):
    pass


class Generic[T]:
    def item(self, value: T) -> T:
        return value


class Bounded[T: Comparable]:
    pass


class WithDefault[T=int, U: Iterable[T] = list]:
    pass
