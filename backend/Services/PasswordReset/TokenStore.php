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

    public function add(string $username, string $tokenHash, int $ttl, string $ip): void
    {
        $now = time();
        $rows = $this->all();

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

        $this->save($rows);
    }

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

    public function markUsed(string $tokenHash): bool
    {
        $rows = $this->all();
        $hit = false;
        foreach ($rows as &$row) {
            if (isset($row['token_hash']) && hash_equals((string) $row['token_hash'], $tokenHash)) {
                $row['used'] = true;
                $hit = true;
            }
        }
        unset($row);
        if ($hit) $this->save($rows);
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

    protected function save(array $rows): void
    {
        $dir = dirname($this->file);
        if (! is_dir($dir)) {
            @mkdir($dir, 0775, true);
        }
        file_put_contents($this->file, json_encode(array_values($rows)), LOCK_EX);
    }
}
