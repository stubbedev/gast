<?php
abstract class AbstractBase
{
    public const VERSION = '1.0';
    protected const int TYPED_CONST = 1;
    private const HIDDEN = 2;

    public static array $shared = [];
    protected ?string $name = null;
    private readonly int $id;
    var $legacyVar;
    public int $a, $b = 2;
    static $untyped;

    abstract protected function templateMethod(): void;

    public final function finalMethod(): static
    {
        return $this;
    }

    public function __construct(int $id)
    {
        $this->id = $id;
    }
}

final class Concrete extends AbstractBase implements Countable, JsonSerializable
{
    use SomeTrait, OtherTrait {
        SomeTrait::method insteadof OtherTrait;
        OtherTrait::method as aliased;
        OtherTrait::other as public;
        SomeTrait::third as protected renamed;
    }

    public const public_const = 1;

    protected function templateMethod(int $level = 0): void {}

    public function jsonSerialize(): mixed
    {
        return [];
    }

    public function count(): int
    {
        return 0;
    }
}

class WithConstructorPromotion
{
    public function __construct(
        private readonly int $id,
        protected string $name = 'x',
        public array $tags = [],
        int $unpromoted = 0,
    ) {
    }
}

readonly class ValueObject
{
    public function __construct(
        public int $x,
        public int $y,
    ) {
    }
}

interface Repository extends Countable
{
    public function find(int $id): ?object;
    public const LIMIT = 10;
}

trait Loggable
{
    protected array $logs = [];

    public function log(string $message): void
    {
        $this->logs[] = $message;
    }
}

enum Suit: string
{
    case Hearts = 'H';
    case Spades = 'S';

    public function color(): string
    {
        return 'color';
    }
}

enum Status
{
    case Draft;
    case Published;
}

enum IntEnum: int implements HasValue
{
    case One = 1;

    public function value(): int
    {
        return $this->value;
    }
}
