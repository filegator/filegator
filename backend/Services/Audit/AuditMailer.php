<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Audit;

use Filegator\Services\Logger\LoggerInterface;
use Filegator\Services\Mailer\MailerInterface;
use Filegator\Services\Service;

/**
 * Sends operational audit emails whenever an admin mutates a user
 * (create / update / delete / reset MFA) or a user disables their own MFA.
 *
 * Audit alerts are deliberately separate from transactional email (password
 * reset etc.) so they can have their own visible From: address, recipient,
 * and on/off switch without touching unrelated mail flows.
 */
class AuditMailer implements Service
{
    /**
     * Update-event subject lines are picked from the highest-priority field
     * that changed. Order: most consequential first.
     */
    const UPDATE_PRIORITY = ['role', 'homedir', 'permissions', 'password', 'email', 'username'];

    protected $mailer;

    protected $logger;

    protected $config = [];

    public function __construct(MailerInterface $mailer, LoggerInterface $logger)
    {
        $this->mailer = $mailer;
        $this->logger = $logger;
    }

    public function init(array $config = [])
    {
        $this->config = $config;
    }

    public function isConfigured(): bool
    {
        return ! empty($this->config['recipient']) && ! empty($this->config['from_email']);
    }

    public function userCreated(string $adminUsername, array $userSnapshot, ?string $email): void
    {
        if (! $this->shouldSend()) return;

        $subject = sprintf('New user created: %s', $userSnapshot['username'] ?? '(unknown)');
        $body = $this->joinLines([
            sprintf('A new user was created in the %s.', $this->appLabel()),
            '',
            sprintf('Admin: %s', $adminUsername),
            sprintf('Username: %s', $userSnapshot['username'] ?? ''),
            sprintf('Name: %s', $userSnapshot['name'] ?? ''),
            sprintf('Folder: %s', $userSnapshot['homedir'] ?? ''),
            sprintf('Role: %s', $userSnapshot['role'] ?? ''),
            sprintf('Permissions: %s', $this->formatPermissions($userSnapshot['permissions'] ?? [])),
            sprintf('Email: %s', $email !== null && $email !== '' ? $email : '(none)'),
            '',
            $this->timestampLine(),
        ]);

        $this->dispatch($subject, $body);
    }

    public function userDeleted(string $adminUsername, array $userSnapshot, ?string $email): void
    {
        if (! $this->shouldSend()) return;

        $username = $userSnapshot['username'] ?? '(unknown)';
        $subject = sprintf('User deleted: %s', $username);
        $body = $this->joinLines([
            sprintf('A user was deleted from the %s.', $this->appLabel()),
            '',
            sprintf('Admin: %s', $adminUsername),
            sprintf('Username: %s', $username),
            sprintf('Name: %s', $userSnapshot['name'] ?? ''),
            sprintf('Folder at time of deletion: %s', $userSnapshot['homedir'] ?? ''),
            sprintf('Role: %s', $userSnapshot['role'] ?? ''),
            sprintf('Permissions: %s', $this->formatPermissions($userSnapshot['permissions'] ?? [])),
            sprintf('Email: %s', $email !== null && $email !== '' ? $email : '(none)'),
            '',
            $this->timestampLine(),
        ]);

        $this->dispatch($subject, $body);
    }

    /**
     * Compute the field-by-field diff for an update event and send an alert
     * if something meaningful changed. Returns silently for no-op saves and
     * for saves where the only change is the cosmetic 'name' field.
     */
    public function userUpdated(
        string $adminUsername,
        string $originalUsername,
        array $before,
        array $after,
        ?string $beforeEmail,
        ?string $afterEmail,
        bool $passwordChanged
    ): void {
        if (! $this->shouldSend()) return;

        $diffs = $this->computeDiffs($before, $after, $beforeEmail, $afterEmail, $passwordChanged);
        if (empty($diffs)) return;
        if (count($diffs) === 1 && isset($diffs['name'])) return; // cosmetic-only

        $subject = $this->subjectForUpdate($originalUsername, $diffs);

        $lines = [
            sprintf('A user was updated in the %s.', $this->appLabel()),
            '',
            sprintf('Admin: %s', $adminUsername),
            sprintf('Username: %s', $originalUsername),
            '',
            'Changes:',
        ];
        foreach ($diffs as $field => $change) {
            $lines[] = '- '.$this->formatDiffLine($field, $change);
        }
        $lines[] = '';
        $lines[] = $this->timestampLine();

        $this->dispatch($subject, $this->joinLines($lines));
    }

    public function mfaResetByAdmin(string $adminUsername, string $targetUsername): void
    {
        if (! $this->shouldSend()) return;

        $subject = sprintf('MFA reset by admin for %s', $targetUsername);
        $body = $this->joinLines([
            sprintf('An admin reset another user\'s two-factor authentication in the %s.', $this->appLabel()),
            '',
            sprintf('Admin: %s', $adminUsername),
            sprintf('Target user: %s', $targetUsername),
            '',
            $this->timestampLine(),
        ]);

        $this->dispatch($subject, $body);
    }

    public function userSelfDisabledMfa(string $username, string $role): void
    {
        if (! $this->shouldSend()) return;

        $subject = sprintf('MFA disabled by user: %s', $username);
        $body = $this->joinLines([
            sprintf('A user disabled their own two-factor authentication in the %s.', $this->appLabel()),
            '',
            sprintf('Username: %s', $username),
            sprintf('Role: %s', $role),
            '',
            $this->timestampLine(),
        ]);

        $this->dispatch($subject, $body);
    }

    protected function shouldSend(): bool
    {
        if (! ($this->config['enabled'] ?? true)) return false;
        if (! $this->isConfigured()) {
            $this->logger->log('Audit alert skipped: AuditMailer not configured (recipient or from_email missing)');
            return false;
        }
        return true;
    }

    protected function dispatch(string $subject, string $textBody): void
    {
        $ok = $this->mailer->send(
            (string) $this->config['recipient'],
            $subject,
            $textBody,
            null,
            (string) $this->config['from_email'],
            isset($this->config['from_name']) ? (string) $this->config['from_name'] : null
        );

        $this->logger->log(sprintf(
            'Audit email %s: %s',
            $ok ? 'sent' : 'failed',
            $subject
        ));
    }

    protected function computeDiffs(array $before, array $after, ?string $beforeEmail, ?string $afterEmail, bool $passwordChanged): array
    {
        $diffs = [];
        foreach (['role', 'homedir', 'permissions', 'username', 'name'] as $field) {
            $b = $before[$field] ?? null;
            $a = $after[$field] ?? null;
            if ($this->normalize($b) !== $this->normalize($a)) {
                $diffs[$field] = ['before' => $b, 'after' => $a];
            }
        }
        if (($beforeEmail ?? null) !== ($afterEmail ?? null)) {
            $diffs['email'] = ['before' => $beforeEmail, 'after' => $afterEmail];
        }
        if ($passwordChanged) {
            $diffs['password'] = ['before' => null, 'after' => null];
        }
        return $diffs;
    }

    /**
     * Make sortable arrays comparable: permissions like ['read', 'write'] and
     * ['write', 'read'] should be treated as equivalent for diff purposes.
     */
    protected function normalize($value)
    {
        if (is_array($value)) {
            $copy = $value;
            sort($copy);
            return $copy;
        }
        return $value;
    }

    protected function subjectForUpdate(string $username, array $diffs): string
    {
        foreach (self::UPDATE_PRIORITY as $field) {
            if (! isset($diffs[$field])) continue;
            switch ($field) {
                case 'role':
                    return sprintf('User %s role changed: %s → %s', $username, $diffs[$field]['before'], $diffs[$field]['after']);
                case 'homedir':
                    return sprintf('Folder changed for %s: %s → %s', $username, $diffs[$field]['before'], $diffs[$field]['after']);
                case 'permissions':
                    return sprintf('Permissions changed for %s', $username);
                case 'password':
                    return sprintf('Password reset by admin for %s', $username);
                case 'email':
                    return sprintf('Email changed for %s', $username);
                case 'username':
                    return sprintf('Username changed: %s → %s', $diffs[$field]['before'], $diffs[$field]['after']);
            }
        }
        return sprintf('Profile updated for %s', $username);
    }

    protected function formatDiffLine(string $field, array $change): string
    {
        switch ($field) {
            case 'password':
                return 'Password: (reset by admin)';
            case 'permissions':
                return sprintf(
                    'Permissions: %s → %s',
                    $this->formatPermissions($change['before'] ?? []),
                    $this->formatPermissions($change['after'] ?? [])
                );
            case 'email':
                return sprintf(
                    'Email: %s → %s',
                    ($change['before'] ?? '') !== '' && $change['before'] !== null ? $change['before'] : '(none)',
                    ($change['after'] ?? '') !== '' && $change['after'] !== null ? $change['after'] : '(none)'
                );
            default:
                return sprintf(
                    '%s: %s → %s',
                    ucfirst($field),
                    $this->scalar($change['before'] ?? ''),
                    $this->scalar($change['after'] ?? '')
                );
        }
    }

    protected function formatPermissions($perms): string
    {
        if (is_string($perms)) $perms = explode('|', $perms);
        if (! is_array($perms) || empty($perms)) return '(none)';
        $perms = array_filter($perms, function ($p) { return $p !== ''; });
        return $perms ? implode(', ', $perms) : '(none)';
    }

    protected function scalar($v): string
    {
        if (is_array($v)) return implode(',', $v);
        if ($v === null) return '(none)';
        return (string) $v;
    }

    protected function appLabel(): string
    {
        return (string) ($this->config['app_label'] ?? 'FileGator portal');
    }

    protected function timestampLine(): string
    {
        return 'Timestamp: '.gmdate('Y-m-d H:i:s').' UTC';
    }

    protected function joinLines(array $lines): string
    {
        return implode("\n", $lines)."\n";
    }
}
