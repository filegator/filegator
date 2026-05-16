<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit;

use Filegator\Services\Logger\Adapters\MonoLogger;
use Filegator\Services\Mailer\Adapters\SymfonyMailer;
use Monolog\Handler\NullHandler;
use PHPUnit\Framework\TestCase;

/**
 * Ensures the SymfonyMailer adapter enforces an SMTP timeout even when
 * the operator forgot to set one in the DSN. Without this guard, an
 * unreachable SMTP server pins a PHP-FPM worker for PHP's 60s default
 * socket timeout.
 *
 * @internal
 */
class SymfonyMailerTimeoutTest extends TestCase
{
    private function build(array $config): SymfonyMailer
    {
        $logger = new MonoLogger();
        $logger->init(['monolog_handlers' => [function () { return new NullHandler(); }]]);
        $mailer = new SymfonyMailer($logger);
        $mailer->init($config);
        return $mailer;
    }

    private function dsnFor(SymfonyMailer $mailer): string
    {
        $ref = new \ReflectionObject($mailer);
        $prop = $ref->getProperty('config');
        $prop->setAccessible(true);
        return (string) ($prop->getValue($mailer)['dsn'] ?? '');
    }

    public function testTimeoutAppendedWhenMissing()
    {
        $m = $this->build(['dsn' => 'smtp://user:pass@smtp.example.com:587?encryption=tls', 'from_email' => 'x@y']);
        $dsn = $this->dsnFor($m);
        $this->assertStringContainsString('timeout=5', $dsn);
        $this->assertStringContainsString('encryption=tls', $dsn);
    }

    public function testOperatorSuppliedTimeoutPreserved()
    {
        $m = $this->build(['dsn' => 'smtp://smtp.example.com:587?timeout=20', 'from_email' => 'x@y']);
        $dsn = $this->dsnFor($m);
        $this->assertStringContainsString('timeout=20', $dsn);
        $this->assertStringNotContainsString('timeout=5', $dsn);
    }

    public function testCustomDefaultTimeoutApplied()
    {
        $m = $this->build(['dsn' => 'smtp://smtp.example.com:587', 'from_email' => 'x@y', 'timeout' => 8]);
        $dsn = $this->dsnFor($m);
        $this->assertStringContainsString('timeout=8', $dsn);
    }

    public function testNullDsnLeftUntouched()
    {
        $m = $this->build(['dsn' => 'null://null', 'from_email' => 'x@y']);
        $this->assertSame('null://null', $this->dsnFor($m));
    }
}
