<?php
#[Attribute(Attribute::TARGET_CLASS)]
final class Table
{
    public function __construct(public string $name) {}
}

#[Table(name: 'users'), Cached(ttl: 60)]
class User
{
    #[Validate]
    public int $id = 0;

    #[Deprecated('use newName')]
    public const OLD = 1;

    #[Route('/users', methods: ['GET'])]
    public function list(): array
    {
        return [];
    }
}

$fn = #[Pure] static function (#[Sensitive] string $secret): string {
    return 'hashed';
};

#[Table(name: 'labels')]
enum Label
{
    case Important;
}
