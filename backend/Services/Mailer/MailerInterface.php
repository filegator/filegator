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

    /**
     * Send an email. If $fromEmail is provided it overrides the configured
     * default sender for this one send — useful when one app has multiple
     * mail flows that need different visible senders (e.g. transactional
     * no-reply vs. operational audit alerts).
     */
    public function send(string $to, string $subject, string $textBody, ?string $htmlBody = null, ?string $fromEmail = null, ?string $fromName = null): bool;
}
