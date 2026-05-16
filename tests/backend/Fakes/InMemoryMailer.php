<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Fakes;

use Filegator\Services\Mailer\MailerInterface;
use Filegator\Services\Service;

class InMemoryMailer implements Service, MailerInterface
{
    public static $messages = [];

    public static $configured = true;

    public function init(array $config = [])
    {
        if (array_key_exists('configured', $config)) {
            self::$configured = (bool) $config['configured'];
        }
    }

    public function isConfigured(): bool
    {
        return self::$configured;
    }

    public function send(string $to, string $subject, string $textBody, ?string $htmlBody = null): bool
    {
        self::$messages[] = [
            'to' => $to,
            'subject' => $subject,
            'text' => $textBody,
            'html' => $htmlBody,
        ];
        return true;
    }

    public static function reset(): void
    {
        self::$messages = [];
        self::$configured = true;
    }

    public static function last(): ?array
    {
        return self::$messages ? end(self::$messages) : null;
    }
}
