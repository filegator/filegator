<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Mfa;

use Filegator\Utils\PasswordHash;

class BackupCodeGenerator
{
    use PasswordHash;

    const ALPHABET = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'; // excludes 0,O,1,I

    /**
     * Generate $count plaintext codes of $length characters, formatted as XXXXX-XXXXX.
     *
     * @return string[]
     */
    public static function generate(int $count = 10, int $length = 10): array
    {
        $codes = [];
        for ($i = 0; $i < $count; $i++) {
            $raw = '';
            for ($j = 0; $j < $length; $j++) {
                $raw .= self::ALPHABET[random_int(0, strlen(self::ALPHABET) - 1)];
            }
            $codes[] = substr($raw, 0, 5).'-'.substr($raw, 5);
        }
        return $codes;
    }

    /**
     * Hash a list of plaintext backup codes for storage.
     *
     * @param  string[] $codes
     * @return string[]
     */
    public static function hashAll(array $codes): array
    {
        return array_map(static fn ($c) => self::hashPassword(self::normalize($c)), $codes);
    }

    public static function normalize(string $code): string
    {
        return strtoupper(preg_replace('/[^A-Z0-9]/i', '', $code) ?? '');
    }
}
