<?php

/**
 * Test bootstrap: autoloads the module's src/ classes and, when the real
 * psr/log is absent (running outside an OpenEMR stack), defines a minimal
 * Psr\Log\LoggerInterface + NullLogger so HubClient can be exercised standalone.
 */

declare(strict_types=1);

spl_autoload_register(static function (string $class): void {
    $prefix = 'OpenEMR\\Modules\\Hl7Hub\\';
    if (!str_starts_with($class, $prefix)) {
        return;
    }
    $relative = substr($class, strlen($prefix));
    if (str_starts_with($relative, 'Tests\\')) {
        return; // test classes are loaded by PHPUnit directly
    }
    $file = __DIR__ . '/../src/' . str_replace('\\', '/', $relative) . '.php';
    if (is_file($file)) {
        require $file;
    }
});

// Minimal PSR-3 stubs, only if the real package isn't autoloadable.
if (!interface_exists(\Psr\Log\LoggerInterface::class)) {
    require __DIR__ . '/stubs/psr_log.php';
}
