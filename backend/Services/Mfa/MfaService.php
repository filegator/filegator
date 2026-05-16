<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Mfa;

use Filegator\Config\Config;
use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Auth\MfaCapableInterface;
use Filegator\Services\Service;
use Filegator\Services\Tmpfs\TmpfsInterface;
use OTPHP\TOTP;

class MfaService implements Service
{
    protected $auth;

    protected $tmpfs;

    protected $config;

    protected $issuer = 'FileGator';

    public function __construct(AuthInterface $auth, TmpfsInterface $tmpfs, Config $config)
    {
        $this->auth = $auth;
        $this->tmpfs = $tmpfs;
        $this->config = $config;
    }

    public function init(array $config = [])
    {
        if (! empty($config['issuer'])) {
            $this->issuer = (string) $config['issuer'];
        }
    }

    public function isSupported(): bool
    {
        return $this->auth instanceof MfaCapableInterface;
    }

    public function isEnabled(string $username): bool
    {
        if (! $this->isSupported()) return false;
        $state = $this->capable()->getMfaState($username);
        return (bool) $state['enabled'];
    }

    public function isRequiredForUser(string $username, string $role): bool
    {
        if ($role === 'admin' && (bool) $this->config->get('mfa_required_for_admins', true)) {
            return true;
        }
        return false;
    }

    /**
     * Begin enrollment: generate a fresh secret (NOT yet persisted as enabled) and return
     * the secret + provisioning URI. The secret is persisted on the user record so a refresh
     * of the page does not invalidate it; mfa_enabled stays false until confirm.
     */
    public function beginEnrollment(string $username): array
    {
        $totp = TOTP::create();
        $totp->setLabel($username);
        $totp->setIssuer($this->issuer);

        $this->capable()->setMfaSecret($username, $totp->getSecret());

        return [
            'secret' => $totp->getSecret(),
            'otpauth_uri' => $totp->getProvisioningUri(),
            'issuer' => $this->issuer,
            'label' => $username,
        ];
    }

    /**
     * Confirm enrollment: verify a TOTP code against the pending secret, then mark enabled
     * and return one-time backup codes (plaintext) for display to the user.
     *
     * @return string[]|null Plaintext backup codes, or null if the code was invalid.
     */
    public function confirmEnrollment(string $username, string $code): ?array
    {
        $state = $this->capable()->getMfaState($username);
        $secret = $state['secret'] ?? null;
        if (! $secret) return null;
        if (! $this->verifyTotpAgainstSecret($secret, $code)) return null;

        $plain = BackupCodeGenerator::generate();
        $hashes = BackupCodeGenerator::hashAll($plain);
        $this->capable()->enableMfa($username, $hashes);

        return $plain;
    }

    public function regenerateBackupCodes(string $username): array
    {
        $plain = BackupCodeGenerator::generate();
        $hashes = BackupCodeGenerator::hashAll($plain);
        $this->capable()->regenerateBackupCodes($username, $hashes);
        return $plain;
    }

    public function disable(string $username): void
    {
        $this->capable()->disableMfa($username);
    }

    /**
     * Verify a TOTP code (6-digit) against the user's stored secret, with replay protection.
     */
    public function verifyTotp(string $username, string $code): bool
    {
        $state = $this->capable()->getMfaState($username);
        $secret = $state['secret'] ?? null;
        if (! $secret) return false;

        $code = preg_replace('/\s+/', '', $code) ?? '';

        if ($this->isReplayed($username, $code)) return false;

        if (! $this->verifyTotpAgainstSecret($secret, $code)) {
            return false;
        }

        $this->markUsed($username, $code);
        return true;
    }

    public function consumeBackupCode(string $username, string $code): bool
    {
        $normalized = BackupCodeGenerator::normalize($code);
        if ($normalized === '') return false;
        return $this->capable()->consumeBackupCode($username, $normalized);
    }

    protected function verifyTotpAgainstSecret(string $secret, string $code): bool
    {
        $totp = TOTP::createFromSecret($secret);
        // window = 1 step (~30s) drift tolerance
        return $totp->verify($code, null, 1);
    }

    protected function isReplayed(string $username, string $code): bool
    {
        $file = $this->replayFile($username, $code);
        if ($this->tmpfs->exists($file)) return true;
        return false;
    }

    protected function markUsed(string $username, string $code): void
    {
        $this->tmpfs->write($this->replayFile($username, $code), '1', false);
    }

    protected function replayFile(string $username, string $code): string
    {
        return 'mfa_used_'.hash('sha256', $username.'|'.$code).'.lock';
    }

    protected function capable(): MfaCapableInterface
    {
        if (! $this->auth instanceof MfaCapableInterface) {
            throw new \RuntimeException('Configured auth adapter does not support MFA');
        }
        return $this->auth;
    }
}
