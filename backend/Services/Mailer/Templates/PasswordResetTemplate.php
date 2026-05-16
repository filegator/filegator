<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Mailer\Templates;

class PasswordResetTemplate
{
    public static function render(string $resetUrl, string $username, int $ttlMinutes, string $appName): array
    {
        $safeUser = htmlspecialchars($username, ENT_QUOTES, 'UTF-8');
        $safeUrl  = htmlspecialchars($resetUrl, ENT_QUOTES, 'UTF-8');
        $safeApp  = htmlspecialchars($appName, ENT_QUOTES, 'UTF-8');

        $text = "Hi {$username},\n\n"
            ."We received a request to reset the password for your {$appName} account.\n\n"
            ."Click the link below to set a new password. The link expires in {$ttlMinutes} minutes and can only be used once.\n\n"
            ."{$resetUrl}\n\n"
            ."If you did not request this, you can safely ignore this email.\n";

        $html = "<p>Hi {$safeUser},</p>"
            ."<p>We received a request to reset the password for your {$safeApp} account.</p>"
            ."<p><a href=\"{$safeUrl}\">Reset your password</a></p>"
            ."<p>The link expires in {$ttlMinutes} minutes and can only be used once.</p>"
            ."<p>If you did not request this, you can safely ignore this email.</p>";

        return ['text' => $text, 'html' => $html];
    }
}
