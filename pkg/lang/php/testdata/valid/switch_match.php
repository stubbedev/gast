<?php
switch ($a) {
    case 0:
    case 1:
        echo "zero or one";
        break;
    case 2;
        echo "semicolon case";
        break;
    default:
        echo "other";
}

switch ($a):
    case 1:
        return 1;
    default:
        return 0;
endswitch;

switch (true) {
    case $a: break;
}

$result = match ($code) {
    200, 201 => "ok",
    301, 302 => "redirect",
    default => "unknown",
};

$typed = match (true) {
    $a > 10 => "big",
    $a > 0 => "small",
    default => "nonpositive",
};
