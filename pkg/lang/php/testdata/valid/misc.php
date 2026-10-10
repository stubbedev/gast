<?php
declare(strict_types=1);

namespace {
    $global = 'top level without namespace braces body';
}

namespace Nested\Deep {
    const NESTED = 1;
}

namespace Single {
    declare(ticks=1) {
        echo 'unused';
    }
}

namespace {
    $html = <<<'HTML'
<div class="gast">
    parsed
</div>
HTML;
}
