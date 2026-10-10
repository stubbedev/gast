<?php
$name = 'world';
$obj = (object) ['a' => (object) ['b' => 'deep'], 'items' => ['x', 'y'], 'n' => null];
$arr = ['key' => 'value', 0 => 'zero', 'name' => 'boom'];
$i = 0;

// Simple interpolation: $name, one property level, one index level.
$a = "hello $name!";
$b = "prop $obj->a and nullsafe $obj?->n";
$c = "offset $arr[key], $arr[0], $arr[-1] and $arr[$i]";
$d = "second level stays literal: $obj->a->b and $arr[key][x]";

// ${...} forms: name, name with index, and variable variables.
$e = "${name} and ${arr[0]}";
$f = "${'name'} and ${$name}";

// {$...} complex syntax with full expressions.
$g = "{$obj->a->b} {$obj->items[1]} {$arr[$name]}";
$h = "{$obj->items[$i + 0]}";
$i2 = "{$name[0]}";
$j = "literal braces { $name } and escaped \$name";
$k = "no interpolation: price 5$ and lone $";
$l = "{$name}{$name}";

// Heredoc interpolates, nowdoc does not.
$m = <<<DOC
line $name and {$obj->a}
second $arr[key]
DOC;
$n = <<<'DOC'
raw $name and {$obj->a}
DOC;

// Shell exec strings interpolate like double quotes.
$o = `ls {$dir} $name`;

// Escapes inside literal chunks still decode.
$p = "tab\t$name\nnewline \x41 \101 \u{42}";
