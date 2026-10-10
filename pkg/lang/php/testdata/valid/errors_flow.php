<?php
try {
    risky();
} catch (SpecificException $e) {
    handle($e);
} catch (AnotherException | ThirdException $e) {
    handleOther($e);
} catch (Exception) {
    handleAnonymous();
} finally {
    cleanup();
}

$value = $condition ? throw new InvalidArgumentException('bad') : $fallback;

throw new RuntimeException('message', 0, $previous);

$result = $obj instanceof Concrete;
$result = $obj instanceof $className;
$result = $obj instanceof \App\Concrete;

echo PHP_VERSION . PHP_EOL . __LINE__ . __FILE__ . __DIR__ . __CLASS__ . __FUNCTION__ . __METHOD__ . __NAMESPACE__;

include 'file.php';
include_once 'other.php';
require __DIR__ . '/required.php';
require_once 'once.php';
$loaded = include 'maybe.php';

exit;
die;
exit(1);
die('message');
