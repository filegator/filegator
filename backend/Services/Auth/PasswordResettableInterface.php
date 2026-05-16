<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Auth;

interface PasswordResettableInterface
{
    public function findByEmail(string $email): ?User;

    public function setPasswordDirect(string $username, string $newPassword): void;
}
