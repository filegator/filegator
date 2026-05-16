<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\PasswordReset;

class TokenStore
{
    protected $file;

    public function __construct(string $file)
    {
        $this->file = $file;
    }

    /**
     * Append a new token row, invalidating any prior unused tokens for the
     * same user. Holds LOCK_EX across the entire read-modify-write window so
     * concurrent /password/forgot calls cannot lose a token or partially
     * invalidate the predecessor.
     */
    public function add(string $username, string $tokenHash, int $ttl, string $ip): void
    {
        $this->withWriteLock(function (array $rows) use ($username, $tokenHash, $ttl, $ip) {
            $now = time();
            foreach ($rows as &$row) {
                if (! empty($row['username']) && $row['username'] === $username && empty($row['used'])) {
                    $row['used'] = true;
                }
            }
            unset($row);

            $rows = $this->gc($rows, $now);

            $rows[] = [
                'token_hash' => $tokenHash,
                'username' => $username,
                'created' => $now,
                'expires' => $now + $ttl,
                'used' => false,
                'ip' => $ip,
            ];

            return $rows;
        });
    }

    /**
     * find() is intentionally read-only and called from /password/reset/validate
     * (a probe endpoint). markUsed() — called from confirmReset — is the
     * authoritative single-use barrier and runs inside a LOCK_EX RMW so a
     * concurrent double-submit of the same token cannot both pass.
     */
    public function find(string $tokenHash): ?array
    {
        $now = time();
        foreach ($this->all() as $row) {
            if (! isset($row['token_hash']) || ! hash_equals((string) $row['token_hash'], $tokenHash)) continue;
            if (! empty($row['used'])) return null;
            if (($row['expires'] ?? 0) < $now) return null;
            return $row;
        }
        return null;
    }

    /**
     * Atomically mark a token as used. Returns true only for the first caller
     * that observes an unused, non-expired matching row; concurrent callers
     * with the same token get false and must reject the reset.
     */
    public function markUsed(string $tokenHash): bool
    {
        $hit = false;
        $now = time();
        $this->withWriteLock(function (array $rows) use ($tokenHash, &$hit, $now) {
            foreach ($rows as &$row) {
                if (! isset($row['token_hash'])) continue;
                if (! hash_equals((string) $row['token_hash'], $tokenHash)) continue;
                // Re-check used/expires inside the locked window so two parallel
                // confirms cannot both observe "unused" before either writes.
                if (! empty($row['used'])) continue;
                if (($row['expires'] ?? 0) < $now) continue;
                $row['used'] = true;
                $hit = true;
                break;
            }
            unset($row);
            return $rows;
        });
        return $hit;
    }

    public function all(): array
    {
        if (! file_exists($this->file)) return [];
        $raw = (string) file_get_contents($this->file);
        $decoded = json_decode($raw, true);
        return is_array($decoded) ? $decoded : [];
    }

    protected function gc(array $rows, int $now): array
    {
        $cutoff = $now - 86400;
        return array_values(array_filter($rows, function ($row) use ($cutoff) {
            return ($row['expires'] ?? 0) >= $cutoff;
        }));
    }

    /**
     * Read-modify-write helper. $mutator receives the current rows array and
     * returns the updated array. The whole read/mutate/write happens while
     * holding LOCK_EX on the file fd, so concurrent FPM workers serialise.
     */
    protected function withWriteLock(callable $mutator): void
    {
        $this->ensureDir();
        $fh = @fopen($this->file, 'c+');
        if ($fh === false) {
            throw new \RuntimeException("Could not open password-reset token file: {$this->file}");
        }
        if (! flock($fh, LOCK_EX)) {
            fclose($fh);
            throw new \RuntimeException("Could not acquire lock on password-reset token file: {$this->file}");
        }
        try {
            rewind($fh);
            $contents = stream_get_contents($fh);
            $rows = ($contents !== false && $contents !== '') ? json_decode($contents, true) : [];
            if (! is_array($rows)) $rows = [];

            $rows = $mutator($rows);
            $rows = array_values($rows);

            rewind($fh);
            ftruncate($fh, 0);
            $written = fwrite($fh, json_encode($rows));
            if ($written === false) {
                throw new \RuntimeException("Could not write password-reset token file: {$this->file}");
            }
            fflush($fh);
        } finally {
            @flock($fh, LOCK_UN);
            @fclose($fh);
        }
    }

    protected function ensureDir(): void
    {
        $dir = dirname($this->file);
        if (! is_dir($dir) && ! mkdir($dir, 0775, true) && ! is_dir($dir)) {
            throw new \RuntimeException("Could not create password-reset token directory: {$dir}");
        }
    }
}
