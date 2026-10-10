<?php
namespace App;

use App\Model\User;
use App\Service as S;
use function App\helpers\format;
use const App\Limits\MAX;
use App\{Model, Repository\UserRepository};
use App\{
    Model\Post,
    Repository\CommentRepository as Comments,
    function App\helpers\debug,
};

const APP_NAME = 'app';

class Namespaced {}

function namespaced_function(): void {}

$anonymous = new class {};
$withArgs = new class(1, 2) extends Base implements I {
    private int $x;
    public function method(): int
    {
        return $this->x;
    }
};
$readonlyAnon = new readonly class {};

$obj = new Namespaced();
$var = new $className();
$derived = new $factory->build();
$nested = new ($get('cls'))();
$self = new self;
$parent = new parent;
$static = new static;
