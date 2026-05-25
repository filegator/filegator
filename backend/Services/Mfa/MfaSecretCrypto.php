<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Mfa;

use Filegator\Services\Service;

/**
 * Symmetric encryption for TOTP secrets at rest. Uses libsodium's
 * crypto_secretbox (XSalsa20 + Poly1305) with a 32-byte key persisted
 * outside users.json so a disk leak of the user DB alone cannot recover
 * working TOTP seeds.
 *
 * Output format: `v1$` + base64(nonce || ciphertext). The version prefix
 * lets us rotate the algorithm later without ambiguity, and is also the
 * lazy-migration sentinel — anything stored without that prefix is
 * treated as legacy plaintext and re-encrypted on next successful TOTP
 * verify (see [MfaService::verifyTotp]).
 *
 * Key recovery: lose the keyfile and every enrolled user's TOTP seed
 * becomes undecipherable. The user can still log in via backup codes
 * (hashed independently and unaffected). For the single-admin case
 * where the admin loses their key AND their backup codes, the documented
 * recovery is to edit users.json to `mfa_enabled=false, mfa_secret=null`
 * for the admin row and re-enroll.
 */
class MfaSecretCrypto implements Service
{
    const VERSION_PREFIX = 'v1$';

    /** Path to the key file. Set in init() from service config. */
    protected $keyPath;

    /** @var string|null Cached key bytes; loaded once per process. */
    protected $key = null;

    public function init(array $config = [])
    {
        if (empty($config['key_path'])) {
            throw new \RuntimeException('MfaSecretCrypto requires a `key_path` config entry');
        }
        $this->keyPath = (string) $config['key_path'];
    }

    public function encrypt(string $plaintext): string
    {
        $key = $this->loadOrCreateKey();
        $nonce = random_bytes(SODIUM_CRYPTO_SECRETBOX_NONCEBYTES);
        $ct = sodium_crypto_secretbox($plaintext, $nonce, $key);
        return self::VERSION_PREFIX . base64_encode($nonce . $ct);
    }

    /**
     * Decrypt a stored ciphertext. Returns null on any failure (MAC
     * mismatch, malformed input, unknown version) so callers can treat
     * the secret as unrecoverable without distinguishing reasons.
     */
    public function decrypt(string $stored): ?string
    {
        if (substr($stored, 0, strlen(self::VERSION_PREFIX)) !== self::VERSION_PREFIX) {
            return null;
        }
        $blob = base64_decode(substr($stored, strlen(self::VERSION_PREFIX)), true);
        if ($blob === false || strlen($blob) < SODIUM_CRYPTO_SECRETBOX_NONCEBYTES) {
            return null;
        }
        $nonce = substr($blob, 0, SODIUM_CRYPTO_SECRETBOX_NONCEBYTES);
        $ct = substr($blob, SODIUM_CRYPTO_SECRETBOX_NONCEBYTES);
        try {
            $key = $this->loadOrCreateKey();
        } catch (\Throwable $e) {
            return null;
        }
        $pt = sodium_crypto_secretbox_open($ct, $nonce, $key);
        return $pt === false ? null : $pt;
    }

    public function isEncrypted(string $stored): bool
    {
        return substr($stored, 0, strlen(self::VERSION_PREFIX)) === self::VERSION_PREFIX;
    }

    /**
     * Return the keyfile's bytes. Creates it atomically (O_EXCL) on
     * first call so two concurrent FPM workers can't both observe
     * absence and both write — exactly one process wins the create;
     * losers fall through to the read path with the same key. Cached
     * in-memory so subsequent encrypt/decrypt calls don't hit disk.
     *
     * Perms hardening: wraps fopen in umask(0077) so the file is
     * created with 0600 from the first byte. The previous
     * fopen-then-chmod pattern left a TOCTOU window where a
     * tight-looping peer process on multi-tenant hosts could open the
     * keyfile during the millisecond between create and chmod.
     */
    protected function loadOrCreateKey(): string
    {
        if ($this->key !== null) {
            return $this->key;
        }

        // Try to atomically create the keyfile. fopen('xb') is the single
        // O_EXCL primitive that distinguishes "created" from "exists":
        //   - success → this process owns first-create, write the key
        //   - false   → file exists (overwhelmingly common after first boot)
        //                OR a different error (perms, etc.) — both routes
        //                converge on reading the now-present file
        // No file_exists pre-check: it would just add a syscall and a TOCTOU
        // window between the stat and the fopen.
        //
        // umask(0077) ensures the create lands at 0600 immediately, closing
        // the perms TOCTOU window from the older chmod-after pattern.
        $previousUmask = umask(0077);
        try {
            $fh = @fopen($this->keyPath, 'xb');
            if ($fh === false) {
                $this->key = $this->readKeyFromDisk();
                return $this->key;
            }

            try {
                $generated = random_bytes(SODIUM_CRYPTO_SECRETBOX_KEYBYTES);
                fwrite($fh, $generated);
                fflush($fh);
            } finally {
                fclose($fh);
            }
            // Defensive chmod in case the umask was already restrictive enough
            // that 0077 effectively no-op'd, OR in case the filesystem ignored
            // it (some network filesystems do). Idempotent on the happy path.
            @chmod($this->keyPath, 0600);

            $this->key = $generated;
            return $this->key;
        } finally {
            umask($previousUmask);
        }
    }

    /**
     * Read the keyfile bytes, retrying briefly if the file is present
     * but empty/short. Race window: process A wins fopen('xb'); process
     * B's fopen('xb') fails and falls into this method; B may arrive
     * before A's fwrite+fflush completes and see a 0-byte file. A
     * small backoff loop (capped at ~31ms total) gives the winner time
     * to finish before we declare the keyfile malformed.
     */
    protected function readKeyFromDisk(): string
    {
        $attempts = 5;
        for ($i = 0; $i < $attempts; $i++) {
            $bytes = @file_get_contents($this->keyPath);
            if ($bytes !== false && strlen($bytes) === SODIUM_CRYPTO_SECRETBOX_KEYBYTES) {
                return $bytes;
            }
            // 1ms, 2ms, 4ms, 8ms, 16ms — cumulative ~31ms.
            usleep(1000 * (1 << $i));
        }
        throw new \RuntimeException("MFA encryption key file at {$this->keyPath} is missing or malformed");
    }
}
