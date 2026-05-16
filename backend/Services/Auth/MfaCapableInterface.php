<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Auth;

interface MfaCapableInterface
{
    public function getMfaState(string $username): array;

    public function setMfaSecret(string $username, string $secret): void;

    public function enableMfa(string $username, array $backupCodeHashes): void;

    public function disableMfa(string $username): void;

    public function consumeBackupCode(string $username, string $code): bool;

    public function regenerateBackupCodes(string $username, array $backupCodeHashes): void;

    public function getEmail(string $username): ?string;

    public function setEmail(string $username, ?string $email): void;

    public function findByEmail(string $email): ?User;

    public function verifyPasswordOnly(string $username, string $password): bool;

    /**
     * Establish a fully-authenticated session for the given username, bypassing
     * the password check. Used after a successful MFA verification step.
     */
    public function establishSessionFor(string $username): bool;
}
