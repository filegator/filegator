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
    /** Run the replay-marker GC sweep on roughly 1 in N verifyTotp() calls. */
    const REPLAY_GC_SAMPLE_RATE = 10;

    /** TTL for individual mfa_used_*.lock replay markers, in seconds. */
    const REPLAY_MARKER_TTL = 90;

    protected $auth;

    protected $tmpfs;

    protected $config;

    protected $crypto;

    protected $issuer = 'FileGator';

    public function __construct(AuthInterface $auth, TmpfsInterface $tmpfs, Config $config, MfaSecretCrypto $crypto)
    {
        $this->auth = $auth;
        $this->tmpfs = $tmpfs;
        $this->config = $config;
        $this->crypto = $crypto;
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
     *
     * Refuses to overwrite an existing enrollment so a hijacked session cannot replace the
     * victim's TOTP secret with the attacker's, and so an accidental UI re-click cannot
     * silently break the user's authenticator. Callers must disable() first (which already
     * requires password + current TOTP) to re-enroll.
     *
     * @throws \RuntimeException if MFA is already enabled for this user.
     */
    public function beginEnrollment(string $username): array
    {
        $state = $this->capable()->getMfaState($username);
        if (! empty($state['enabled'])) {
            throw new \RuntimeException('MFA is already enabled; disable it before re-enrolling');
        }

        $totp = TOTP::create();
        $totp->setLabel($username);
        $totp->setIssuer($this->issuer);

        // Encrypt at rest so a users.json leak doesn't yield working seeds.
        $this->capable()->setMfaSecret($username, $this->crypto->encrypt($totp->getSecret()));

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
        $stored = $state['secret'] ?? null;
        if (! $stored) return null;

        // beginEnrollment writes encrypted; legacy/test rows may still be
        // plaintext. Same shape as verifyTotp's decrypt-or-passthrough.
        if ($this->crypto->isEncrypted($stored)) {
            $secret = $this->crypto->decrypt($stored);
            if ($secret === null) return null;
        } else {
            $secret = $stored;
        }

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
        $stored = $state['secret'] ?? null;
        if (! $stored) return false;

        $code = preg_replace('/\s+/', '', $code) ?? '';

        // Decrypt stored secret (or fall through with the plaintext value
        // if it's a legacy un-encrypted row — lazy migration runs below).
        if ($this->crypto->isEncrypted($stored)) {
            $secret = $this->crypto->decrypt($stored);
            if ($secret === null) return false;
        } else {
            $secret = $stored;
        }

        if (! $this->verifyTotpAgainstSecret($secret, $code)) {
            return false;
        }

        // Atomic SETNX-style replay claim. If two parallel /login/mfa requests
        // submit the same TOTP code, only ONE will successfully create the
        // marker file (O_EXCL semantics inside Tmpfs::addIfAbsent). The loser
        // gets false here and the login fails. This is the only correct order:
        // verify-then-atomically-claim. Verifying after the claim would let
        // an attacker burn replay slots with garbage codes.
        $this->gcExpiredReplayMarkers();
        if (! $this->tmpfs->addIfAbsent($this->replayFile($username, $code), '1')) {
            return false;
        }

        // Lazy migration: upgrade plaintext secrets to encrypted form on
        // first successful verify. Wrapped in try/catch so a write failure
        // never blocks the login itself.
        if (! $this->crypto->isEncrypted($stored)) {
            try {
                $this->capable()->setMfaSecret($username, $this->crypto->encrypt($secret));
            } catch (\Throwable $e) {
                // Best-effort; swallow.
            }
        }

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
        try {
            $totp = TOTP::createFromSecret($secret);
            // window = 1 step (~30s) drift tolerance
            return $totp->verify($code, null, 1);
        } catch (\Throwable $e) {
            // A corrupted/invalid mfa_secret is treated as a verification failure
            // rather than a fatal error, so the user can still try a backup code.
            return false;
        }
    }

    /**
     * Opportunistically GC expired mfa_used_*.lock replay markers (~90s TTL)
     * so they do not accumulate until the tmpfs-wide GC runs (every 2 days).
     * Scoped to our own namespace so other lockfiles (IP throttles) are unaffected.
     */
    protected function gcExpiredReplayMarkers(): void
    {
        if (random_int(1, self::REPLAY_GC_SAMPLE_RATE) !== 1) return;
        foreach ($this->tmpfs->findAll('mfa_used_*.lock') as $entry) {
            if (time() - $entry['time'] >= self::REPLAY_MARKER_TTL) {
                $this->tmpfs->remove($entry['name']);
            }
        }
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
