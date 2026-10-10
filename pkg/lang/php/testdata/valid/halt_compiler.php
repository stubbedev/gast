<?php

namespace App\Tool {
    const EMBED = true;
}

namespace Other {
    function id($x) { return $x; }
}

// After every namespace block closes, the halt is at the outermost
// scope. Everything after the call is compiler-halted raw data: tags,
// dollars, and other bytes that would otherwise fail to lex.
__halt_compiler(); this is raw data ?> not a tag switch <?php $$$
"unterminated string and 'mixed quotes stay raw
#[also not an attribute
