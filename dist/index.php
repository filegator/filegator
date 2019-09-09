<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE
 * file that was distributed with this source code.
 */

if (version_compare(PHP_VERSION, '7.1.3', '<')) {
    echo 'Minimum requirement is PHP 7.1.3. You are using: '.PHP_VERSION."\n";
    die;
}

if (! is_writable(__DIR__.'/../private/logs/')) {
    echo 'Folder not writable: /private/logs/'."\n";
    die;
}

if (! is_writable(__DIR__.'/../repository/')) {
    echo 'Folder not writable: /repository/'."\n";
    die;
}

require __DIR__.'/../vendor/autoload.php';

if (! defined('APP_ENV')) {
    define('APP_ENV', 'production');
}

if (! defined('APP_PUBLIC_PATH')) {
    define('APP_PUBLIC_PATH', '');
}

define('APP_PUBLIC_DIR', __DIR__);
define('APP_VERSION', '7.1.1');

use Filegator\App;
use Filegator\Config\Config;
use Filegator\Container\Container;
use Filegator\Kernel\Request;
use Filegator\Kernel\Response;
use Filegator\Kernel\StreamedResponse;

$config = require __DIR__.'/../configuration.php';

new App(new Config($config), Request::createFromGlobals(), new Response(), new StreamedResponse(), new Container());
