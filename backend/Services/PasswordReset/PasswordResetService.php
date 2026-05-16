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
        $this->store = new TokenStore($this->tokenFile);
    }

    public function isSupported(): bool
    {
        return $this->auth instanceof PasswordResettableInterface;
    }

    /**
     * Issue a reset token for the email address if a matching user exists.
     * Always returns success; callers should not differentiate based on the result.
     */
    public function requestReset(string $email, string $ip, ?string $baseUrl): void
    {
        if (! $this->isSupported()) {
            $this->logger->log('Password reset requested but auth adapter does not support it');
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

        $this->store->add($user->getUsername(), $hash, $ttl, $ip);

        $resetUrl = $this->buildResetUrl($baseUrl, $token);
        $appName = (string) ($this->config->get('frontend_config.app_name') ?: 'FileGator');
        $rendered = PasswordResetTemplate::render($resetUrl, $user->getUsername(), (int) ceil($ttl / 60), $appName);

        $ok = $this->mailer->send($email, $this->resetSubject, $rendered['text'], $rendered['html']);

        $this->logger->log(sprintf(
            'Password reset email %s for user=%s token_prefix=%s ip=%s',
            $ok ? 'sent' : 'failed',
            $user->getUsername(),
            substr($token, 0, 6),
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

    protected function checkAndIncrementLimit(string $key, int $max, int $window): bool
    {
        $file = $key.'.lock';
        foreach ($this->tmpfs->findAll($file) as $flock) {
            if (time() - $flock['time'] >= $window) {
                $this->tmpfs->remove($flock['name']);
            }
        }
        if ($this->tmpfs->exists($file) && strlen($this->tmpfs->read($file)) >= $max) {
            return false;
        }
        $this->tmpfs->write($file, 'x', true);
        return true;
    }

    protected function buildResetUrl(?string $baseUrl, string $token): string
    {
        if ($baseUrl) {
            $base = rtrim($baseUrl, '/').'/';
        } else {
            $base = (string) $this->config->get('public_path', '/');
            if ($base === '' || $base[strlen($base) - 1] !== '/') {
                $base .= '/';
            }
        }
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
