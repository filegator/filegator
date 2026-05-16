<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Mailer;

interface MailerInterface
{
    public function isConfigured(): bool;

    public function send(string $to, string $subject, string $textBody, ?string $htmlBody = null): bool;
}
