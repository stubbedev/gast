<?php
if ($a > $b) {
    echo "a";
} elseif ($a == $b) {
    echo "b";
} else {
    echo "c";
}

if ($a):
    echo "alt";
elseif ($b):
    echo "alt2";
else:
    echo "alt3";
endif;

while (true) {
    break;
}

while (false):
    continue;
endwhile;

do {
    echo "once";
} while ($cond);

for ($i = 0; $i < 10; $i++) {
    echo $i;
}

for (;;);
for ($i = 0, $j = 1; $i < $j; $i++, $j--):
    echo $i;
endfor;

foreach ($list as $item) {
    echo $item;
}
foreach ($list as $key => $value) {}
foreach ($list as &$ref) {}
foreach ($list as $key => &$ref) {}
foreach ($data as [$id, $name]) {}
foreach ($data as list($id, $name)) {}
foreach ($list as $item):
    echo $item;
endforeach;

goto end;
echo "skipped";
end:
echo "done";

label_only: {
    echo "block";
}
