<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Utils;

/**
 * @codeCoverageIgnore
 */
trait PasswordHash
{
    public static function hashPassword($value)
    {
        $hash = password_hash($value, PASSWORD_BCRYPT);

        if ($hash === false) {
            throw new \Exception('Bcrypt hashing not supported.');
        }

        return $hash;
    }

    public static function verifyPassword($value, $hash)
    {
        return password_verify($value, $hash);
    }
}
