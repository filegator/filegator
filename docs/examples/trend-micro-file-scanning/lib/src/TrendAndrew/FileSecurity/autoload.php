<?php
/**
 * Trend Micro Vision One File Security SDK for PHP
 *
 * Simple PSR-4 compatible autoloader for the SDK.
 * Use this if you're not using Composer.
 *
 * @package TrendAndrew\FileSecurity
 * @license MIT
 */

spl_autoload_register(function ($class) {
    // Base namespace prefix
    $prefix = 'TrendAndrew\\FileSecurity\\';

    // Base directory for the namespace prefix
    $baseDir = __DIR__ . '/';

    // Does the class use the namespace prefix?
    $len = strlen($prefix);
    if (strncmp($prefix, $class, $len) !== 0) {
        // No, move to the next registered autoloader
        return;
    }

    // Get the relative class name
    $relativeClass = substr($class, $len);

    // Replace namespace separators with directory separators,
    // append with .php
    $file = $baseDir . str_replace('\\', '/', $relativeClass) . '.php';

    // If the file exists, require it
    if (file_exists($file)) {
        require $file;
    }
});
