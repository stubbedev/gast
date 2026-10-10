<?php
$a = 'single \n stays literal, only \\ and \' escape';
$b = "double \n \t \r \v \f \e \\ \$ \" oct\101 hex\x41 unicode\u{1F600}";
$c = "interpolated $b and {$a->prop} and {$a['key']} and ${notChecked}";
$d = <<<HEREDOC
line one $b
line two
HEREDOC;
$e = <<<"ID"
quoted heredoc \n
ID;
$f = <<<'NOW'
no escapes \n $raw
NOW;
$g = `ls -la {$dir}`;
$h = <<<INDENT
    body
    INDENT . "after";
