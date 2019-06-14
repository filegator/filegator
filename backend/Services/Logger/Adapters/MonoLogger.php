<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Logger\Adapters;

use Filegator\Services\Logger\LoggerInterface;
use Filegator\Services\Service;
use Monolog\ErrorHandler;
use Monolog\Logger;

class MonoLogger implements Service, LoggerInterface
{
    protected $logger;

    public function init(array $config = [])
    {
        $this->logger = new Logger('default');

        foreach ($config['monolog_handlers'] as $handler) {
            $this->logger->pushHandler($handler());
        }

        $handler = new ErrorHandler($this->logger);
        $handler->registerErrorHandler([], true);
        $handler->registerFatalHandler();
    }

    public function log(string $message, int $level = Logger::INFO)
    {
        $this->logger->log($level, $message);
    }
}
