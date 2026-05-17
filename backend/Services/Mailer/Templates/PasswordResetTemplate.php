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
    /**
     * Render a password-reset email as a (text, html) pair.
     *
     * @param array $branding Optional per-deployment branding:
     *   - app_label:      string  Friendly product name used in subject + body
     *                             (default: $appName fallback)
     *   - logo_url:       string  Absolute URL of the header logo. Omitted if blank.
     *   - primary_color:  string  CSS color for the CTA button + header band
     *                             (default: #2c7a7b — neutral teal)
     *   - background:     string  Page background behind the email card
     *                             (default: #f4f4f5 — light grey)
     *   - support_email:  string  Shown in the footer for "didn't request this"
     *                             (omitted if blank)
     */
    public static function render(string $resetUrl, string $username, int $ttlMinutes, string $appName, array $branding = []): array
    {
        $label = (string) ($branding['app_label'] ?? $appName);
        $logo = (string) ($branding['logo_url'] ?? '');
        $primary = (string) ($branding['primary_color'] ?? '#2c7a7b');
        $bg = (string) ($branding['background'] ?? '#f4f4f5');
        $support = (string) ($branding['support_email'] ?? '');

        $safeUser = htmlspecialchars($username, ENT_QUOTES, 'UTF-8');
        $safeUrl = htmlspecialchars($resetUrl, ENT_QUOTES, 'UTF-8');
        $safeLabel = htmlspecialchars($label, ENT_QUOTES, 'UTF-8');
        $safeLogo = htmlspecialchars($logo, ENT_QUOTES, 'UTF-8');
        $safePrimary = htmlspecialchars($primary, ENT_QUOTES, 'UTF-8');
        $safeBg = htmlspecialchars($bg, ENT_QUOTES, 'UTF-8');
        $safeSupport = htmlspecialchars($support, ENT_QUOTES, 'UTF-8');

        // Plain text fallback for mail clients that don't render HTML.
        $text = "Hi {$username},\n\n"
            ."We received a request to reset the password for your {$label} account.\n\n"
            ."Open the link below to choose a new password. The link expires in {$ttlMinutes} minutes and can only be used once.\n\n"
            ."{$resetUrl}\n\n"
            ."If you did not request this, you can safely ignore this email — your password will not change.\n";

        if ($support !== '') {
            $text .= "\nQuestions? Contact {$support}\n";
        }

        // HTML: table-based layout (still the most compatible across mail
        // clients in 2026 — Outlook desktop in particular), inline styles
        // (no <style> support in most clients), centred card on a tinted bg.
        $logoBlock = $logo === ''
            ? ''
            : "<tr><td align=\"center\" style=\"padding: 24px 0 8px;\">"
              ."<img src=\"{$safeLogo}\" alt=\"{$safeLabel}\" style=\"max-height: 80px; max-width: 240px; display: block;\" />"
              ."</td></tr>";

        $supportFooter = $support === ''
            ? ''
            : "<p style=\"margin: 16px 0 0; color: #6b7280; font-size: 12px; text-align: center;\">"
              ."Questions? Contact <a href=\"mailto:{$safeSupport}\" style=\"color: {$safePrimary}; text-decoration: none;\">{$safeSupport}</a>"
              ."</p>";

        $html = <<<HTML
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Reset your password</title>
</head>
<body style="margin: 0; padding: 0; background: {$safeBg}; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #1f2937;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background: {$safeBg}; padding: 32px 16px;">
  <tr>
    <td align="center">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="max-width: 560px; background: #ffffff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); overflow: hidden;">
        <tr><td style="height: 6px; background: {$safePrimary};"></td></tr>
        {$logoBlock}
        <tr>
          <td style="padding: 24px 32px 32px;">
            <h1 style="margin: 0 0 16px; font-size: 20px; font-weight: 600; color: #111827;">Reset your password</h1>
            <p style="margin: 0 0 16px; line-height: 1.5;">Hi {$safeUser},</p>
            <p style="margin: 0 0 16px; line-height: 1.5;">We received a request to reset the password for your <strong>{$safeLabel}</strong> account.</p>
            <p style="margin: 24px 0; text-align: center;">
              <a href="{$safeUrl}" style="display: inline-block; background: {$safePrimary}; color: #ffffff; text-decoration: none; padding: 12px 24px; border-radius: 6px; font-weight: 600;">Choose a new password</a>
            </p>
            <p style="margin: 0 0 16px; line-height: 1.5; color: #4b5563; font-size: 14px;">This link expires in {$ttlMinutes} minutes and can only be used once.</p>
            <p style="margin: 0 0 16px; line-height: 1.5; color: #4b5563; font-size: 14px;">If the button doesn't work, copy and paste this URL into your browser:</p>
            <p style="margin: 0 0 24px; word-break: break-all; color: #6b7280; font-size: 12px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;">{$safeUrl}</p>
            <hr style="margin: 24px 0; border: 0; border-top: 1px solid #e5e7eb;">
            <p style="margin: 0; color: #6b7280; font-size: 12px; line-height: 1.5;">If you did not request a password reset, you can safely ignore this email — your password will not change.</p>
            {$supportFooter}
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>
</body>
</html>
HTML;

        return ['text' => $text, 'html' => $html];
    }
}
