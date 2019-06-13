<?php

/*
 * WARNING!
 *
 * Do not use this index.php as an entry point on production.
 *
 * Instead, set your website document root to /dist directory.
 *
 */

define('APP_ENV', 'development');
define('APP_PUBLIC_PATH', 'dist/');

require __DIR__.'/dist/index.php';
