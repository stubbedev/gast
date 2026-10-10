<?php
function no_params() {}

function typed(int $a, string $b = 'x', ?float $c = null, array &$d = [], ...$rest): int|string {
    return $a;
}

function &by_ref_return(): array
{
    global $storage;
    return $storage;
}

$fn = function ($x) use ($a, &$b): int {
    return $x + $a;
};
$static = static function () {};
$arrow = fn($x) => $x * 2;
$arrowRef = fn&($x) => $x;
$typedArrow = fn(int ...$nums): int => array_sum($nums);
$attrClosure = #[Pure] fn() => 1;

$callable = strlen(...);
$firstClass = $obj->method(...);

function variadic_references(string ...$items) {}
function old_style_defaults($a, $b = self::CLASS_CONST) {}

$gen = function () {
    yield;
    yield 1;
    yield 'key' => 'value';
    yield from another_generator();
    return $final;
};
