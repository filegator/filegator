<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\PasswordReset;

use Filegator\Config\Config;
use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Auth\PasswordResettableInterface;
use Filegator\Services\Logger\LoggerInterface;
use Filegator\Services\Mailer\MailerInterface;
use Filegator\Services\Mailer\Templates\PasswordResetTemplate;
use Filegator\Services\Service;
use Filegator\Services\Tmpfs\TmpfsInterface;

class PasswordResetService implements Service
{
    protected $auth;

    protected $mailer;

    protected $tmpfs;

    protected $logger;

    protected $config;

    protected $store;

    protected $tokenFile;

    protected $resetSubject = 'Reset your password';

    protected $resetUrlBase = null;

    public function __construct(AuthInterface $auth, MailerInterface $mailer, TmpfsInterface $tmpfs, LoggerInterface $logger, Config $config)
    {
        $this->auth = $auth;
        $this->mailer = $mailer;
        $this->tmpfs = $tmpfs;
        $this->logger = $logger;
        $this->config = $config;
    }

    public function init(array $config = [])
    {
        $this->tokenFile = $config['token_file'] ?? null;
        if (! $this->tokenFile) {
            throw new \Exception('PasswordResetService requires a token_file config value');
        }
        if (! empty($config['reset_subject'])) {
            $this->resetSubject = (string) $config['reset_subject'];
        }
        if (! empty($config['reset_url_base'])) {
            $this->resetUrlBase = (string) $config['reset_url_base'];
        }
        $this->store = new TokenStore($this->tokenFile);
    }

    public function isSupported(): bool
    {
        return ($this->auth instanceof PasswordResettableInterface) && $this->resetUrlBase !== null;
    }

    public function isConfigured(): bool
    {
        return $this->resetUrlBase !== null;
    }

    /**
     * Issue a reset token for the email address if a matching user exists.
     * Always returns success; callers should not differentiate based on the result.
     *
     * The reset link host is taken from the configured reset_url_base only —
     * never from the request Host header, to prevent host-header injection of
     * attacker-controlled links into the victim's mailbox.
     */
    public function requestReset(string $email, string $ip): void
    {
        if (! $this->isSupported()) {
            $this->logger->log('Password reset requested but feature is not configured (auth adapter or reset_url_base missing)');
            return;
        }

        $resettable = $this->resettable();
        $user = $resettable->findByEmail($email);

        if (! $user) {
            // Pad timing slightly to flatten user-existence signal.
            usleep(random_int(50000, 150000));
            $this->logger->log('Password reset requested for unknown email (ip='.$ip.')');
            return;
        }

        $token = bin2hex(random_bytes(32));
        $hash = hash('sha256', $token);
        $ttl = (int) $this->config->get('password_reset_token_ttl', 3600);

        try {
            $this->store->add($user->getUsername(), $hash, $ttl, $ip);
        } catch (\Throwable $e) {
            $this->logger->log('Password reset token persist failed: '.$e->getMessage());
            return;
        }

        $resetUrl = $this->buildResetUrl($token);
        $appName = (string) ($this->config->get('frontend_config.app_name') ?: 'FileGator');
        $rendered = PasswordResetTemplate::render($resetUrl, $user->getUsername(), (int) ceil($ttl / 60), $appName);

        $ok = $this->mailer->send($email, $this->resetSubject, $rendered['text'], $rendered['html']);

        $this->logger->log(sprintf(
            'Password reset email %s for user=%s token_hash_prefix=%s ip=%s',
            $ok ? 'sent' : 'failed',
            $user->getUsername(),
            substr($hash, 0, 8),
            $ip
        ));
    }

    public function validateToken(string $token): ?array
    {
        if (! $this->store) return null;
        $hash = hash('sha256', $token);
        return $this->store->find($hash);
    }

    public function confirmReset(string $token, string $newPassword): bool
    {
        if (! $this->isSupported()) return false;
        $hash = hash('sha256', $token);
        $row = $this->store->find($hash);
        if (! $row) return false;
        if (! $this->store->markUsed($hash)) return false;

        $this->resettable()->setPasswordDirect($row['username'], $newPassword);

        $this->logger->log(sprintf(
            'Password reset completed for user=%s token_prefix=%s',
            $row['username'],
            substr($token, 0, 6)
        ));

        return true;
    }

    public function rateLimitIp(string $ip): bool
    {
        $max = (int) $this->config->get('password_reset_max_per_hour_per_ip', 3);
        return $this->checkAndIncrementLimit('reset_ip_'.md5($ip), $max, 3600);
    }

    public function rateLimitEmail(string $email): bool
    {
        $max = (int) $this->config->get('password_reset_max_per_day_per_email', 3);
        return $this->checkAndIncrementLimit('reset_em_'.md5(strtolower(trim($email))), $max, 86400);
    }

    /**
     * Atomic counter increment via Tmpfs::incrementCounterIfBelow (LOCK_EX
     * across read+append) so two concurrent requests can't both observe a
     * sub-max value and both append, bypassing the limit.
     */
    protected function checkAndIncrementLimit(string $key, int $max, int $window): bool
    {
        $file = $key.'.lock';
        foreach ($this->tmpfs->findAll($file) as $flock) {
            if (time() - $flock['time'] >= $window) {
                $this->tmpfs->remove($flock['name']);
            }
        }
        return $this->tmpfs->incrementCounterIfBelow($file, $max) !== -1;
    }

    protected function buildResetUrl(string $token): string
    {
        // resetUrlBase is required (checked in isSupported) — built only from
        // operator-trusted config, never from request headers.
        $base = rtrim((string) $this->resetUrlBase, '/').'/';
        return $base.'#/reset-password?token='.$token;
    }

    protected function resettable(): PasswordResettableInterface
    {
        if (! $this->auth instanceof PasswordResettableInterface) {
            throw new \RuntimeException('Configured auth adapter does not support password reset');
        }
        return $this->auth;
    }
}
