<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Auth\Adapters;

use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Auth\MfaCapableInterface;
use Filegator\Services\Auth\PasswordResettableInterface;
use Filegator\Services\Auth\User;
use Filegator\Services\Auth\UsersCollection;
use Filegator\Services\Service;
use Filegator\Services\Session\SessionStorageInterface as Session;
use Filegator\Utils\PasswordHash;

class JsonFile implements Service, AuthInterface, MfaCapableInterface, PasswordResettableInterface
{
    use PasswordHash;

    const SESSION_KEY = 'json_auth';
    const SESSION_HASH = 'json_auth_hash';

    const GUEST_USERNAME = 'guest';

    protected $session;

    protected $file;

    public function __construct(Session $session)
    {
        $this->session = $session;
    }

    public function init(array $config = [])
    {
        if (! file_exists($config['file'])) {
            copy($config['file'].'.blank', $config['file']);
        }

        $this->file = $config['file'];
    }

    public function user(): ?User
    {
        if (! $this->session) return null;

        $user = $this->session->get(self::SESSION_KEY, null);
        $hash = $this->session->get(self::SESSION_HASH, null);

        if ($user) {
            foreach ($this->getUsers() as $u) {
                if ($u['username'] == $user->getUsername() && hash_equals($this->buildSessionHash($u), (string) $hash)) {
                    return $user;
                }
            }
        }

        return null;
    }

    public function authenticate($username, $password): bool
    {
        $all_users = $this->getUsers();

        foreach ($all_users as &$u) {
            if ($u['username'] == $username && $this->verifyPassword($password, $u['password'])) {
                $user = $this->mapToUserObject($u);
                $this->store($user);
                $this->session->set(self::SESSION_HASH, $this->buildSessionHash($u));
                $this->session->migrate(true);

                return true;
            }
        }

        return false;
    }

    public function verifyPasswordOnly(string $username, string $password): bool
    {
        foreach ($this->getUsers() as $u) {
            if ($u['username'] == $username && $this->verifyPassword($password, $u['password'])) {
                return true;
            }
        }

        return false;
    }

    public function establishSessionFor(string $username): bool
    {
        foreach ($this->getUsers() as $u) {
            if ($u['username'] == $username) {
                $user = $this->mapToUserObject($u);
                $this->store($user);
                $this->session->set(self::SESSION_HASH, $this->buildSessionHash($u));
                $this->session->migrate(true);
                return true;
            }
        }
        return false;
    }

    public function forget()
    {
        return $this->session->invalidate();
    }

    public function store(User $user)
    {
        return $this->session->set(self::SESSION_KEY, $user);
    }

    public function update($username, User $user, $password = ''): User
    {
        $all_users = $this->getUsers();

        if ($username != $user->getUsername() && $this->find($user->getUsername())) {
            throw new \Exception('Username already taken');
        }

        foreach ($all_users as &$u) {
            if ($u['username'] == $username) {
                $u['username'] = $user->getUsername();
                $u['name'] = $user->getName();
                $u['role'] = $user->getRole();
                $u['homedir'] = $user->getHomeDir();
                $u['permissions'] = $user->getPermissions(true);

                if ($password) {
                    $u['password'] = $this->hashPassword($password);
                }

                $this->saveUsers($all_users);

                return $this->find($user->getUsername()) ?: $user;
            }
        }

        throw new \Exception('User not found');
    }

    public function add(User $user, $password): User
    {
        if ($this->find($user->getUsername())) {
            throw new \Exception('Username already taken');
        }

        $all_users = $this->getUsers();

        $all_users[] = [
            'username' => $user->getUsername(),
            'name' => $user->getName(),
            'role' => $user->getRole(),
            'homedir' => $user->getHomeDir(),
            'permissions' => $user->getPermissions(true),
            'password' => $this->hashPassword($password),
            'email' => null,
            'mfa_enabled' => false,
            'mfa_secret' => null,
            'mfa_backup_codes' => null,
            'mfa_enrolled_at' => null,
        ];

        $this->saveUsers($all_users);

        return $this->find($user->getUsername()) ?: $user;
    }

    public function delete(User $user)
    {
        $all_users = $this->getUsers();

        foreach ($all_users as $key => $u) {
            if ($u['username'] == $user->getUsername()) {
                unset($all_users[$key]);
                $this->saveUsers($all_users);

                return true;
            }
        }

        throw new \Exception('User not found');
    }

    public function find($username): ?User
    {
        foreach ($this->getUsers() as $user) {
            if ($user['username'] == $username) {
                return $this->mapToUserObject($user);
            }
        }

        return null;
    }

    public function findByEmail(string $email): ?User
    {
        $needle = strtolower(trim($email));
        if ($needle === '') return null;

        foreach ($this->getUsers() as $user) {
            $stored = isset($user['email']) ? strtolower(trim((string) $user['email'])) : '';
            if ($stored !== '' && hash_equals($stored, $needle)) {
                return $this->mapToUserObject($user);
            }
        }

        return null;
    }

    public function getGuest(): User
    {
        $guest = $this->find(self::GUEST_USERNAME);

        if (! $guest || ! $guest->isGuest()) {
            throw new \Exception('No guest account');
        }

        return $guest;
    }

    public function allUsers(): UsersCollection
    {
        $users = new UsersCollection();

        foreach ($this->getUsers() as $user) {
            $users->addUser($this->mapToUserObject($user));
        }

        return $users;
    }

    /**
     * Adapter-specific batch accessor for callers (admin /listusers) that need
     * MFA + email metadata for every user. Returns one row per user via a
     * single file read, avoiding the N+1 of getMfaState/getEmail per user.
     *
     * @return array<string, array{enabled: bool, backup_codes_remaining: int, email: string|null}>
     */
    public function allUsersMeta(): array
    {
        $meta = [];
        foreach ($this->getUsers() as $u) {
            $meta[$u['username']] = [
                'enabled' => (bool) ($u['mfa_enabled'] ?? false),
                'backup_codes_remaining' => is_array($u['mfa_backup_codes'] ?? null) ? count($u['mfa_backup_codes']) : 0,
                'email' => $u['email'] ?? null,
            ];
        }
        return $meta;
    }

    public function getMfaState(string $username): array
    {
        foreach ($this->getUsers() as $u) {
            if ($u['username'] == $username) {
                return [
                    'enabled' => (bool) ($u['mfa_enabled'] ?? false),
                    'secret' => $u['mfa_secret'] ?? null,
                    'backup_codes_remaining' => is_array($u['mfa_backup_codes'] ?? null) ? count($u['mfa_backup_codes']) : 0,
                    'enrolled_at' => $u['mfa_enrolled_at'] ?? null,
                    'has_email' => ! empty($u['email']),
                    'email' => $u['email'] ?? null,
                ];
            }
        }

        throw new \Exception('User not found');
    }

    public function setMfaSecret(string $username, string $secret): void
    {
        $this->mutateUser($username, function (array &$u) use ($secret) {
            $u['mfa_secret'] = $secret;
        });
    }

    public function enableMfa(string $username, array $backupCodeHashes): void
    {
        $this->mutateUser($username, function (array &$u) use ($backupCodeHashes) {
            $u['mfa_enabled'] = true;
            $u['mfa_backup_codes'] = array_values($backupCodeHashes);
            $u['mfa_enrolled_at'] = time();
        });
    }

    public function disableMfa(string $username): void
    {
        $this->mutateUser($username, function (array &$u) {
            $u['mfa_enabled'] = false;
            $u['mfa_secret'] = null;
            $u['mfa_backup_codes'] = null;
            $u['mfa_enrolled_at'] = null;
        });
    }

    public function consumeBackupCode(string $username, string $code): bool
    {
        $consumed = false;

        $this->mutateUser($username, function (array &$u) use ($code, &$consumed) {
            if (empty($u['mfa_backup_codes']) || ! is_array($u['mfa_backup_codes'])) {
                return;
            }
            $remaining = [];
            foreach ($u['mfa_backup_codes'] as $hash) {
                if (! $consumed && $this->verifyPassword($code, $hash)) {
                    $consumed = true;
                    continue;
                }
                $remaining[] = $hash;
            }
            if ($consumed) {
                $u['mfa_backup_codes'] = array_values($remaining);
            }
        });

        return $consumed;
    }

    public function regenerateBackupCodes(string $username, array $backupCodeHashes): void
    {
        $this->mutateUser($username, function (array &$u) use ($backupCodeHashes) {
            $u['mfa_backup_codes'] = array_values($backupCodeHashes);
        });
    }

    public function getEmail(string $username): ?string
    {
        foreach ($this->getUsers() as $u) {
            if ($u['username'] == $username) {
                return $u['email'] ?? null;
            }
        }
        return null;
    }

    public function setEmail(string $username, ?string $email): void
    {
        $normalized = $email === null ? null : strtolower(trim($email));
        if ($normalized !== null && $normalized !== '') {
            foreach ($this->getUsers() as $u) {
                if ($u['username'] != $username && isset($u['email']) && strtolower((string) $u['email']) === $normalized) {
                    throw new \Exception('Email already in use');
                }
            }
        }
        $this->mutateUser($username, function (array &$u) use ($normalized) {
            $u['email'] = ($normalized === '' ? null : $normalized);
        });
    }

    public function setPasswordDirect(string $username, string $newPassword): void
    {
        $this->mutateUser($username, function (array &$u) use ($newPassword) {
            $u['password'] = $this->hashPassword($newPassword);
        });
    }

    /**
     * Atomic read-modify-write on a single user row.
     *
     * Holds LOCK_EX on the underlying users.json file fd for the entire
     * window so two concurrent FPM workers cannot read the same snapshot,
     * each mutate one row, and silently overwrite each other on save.
     *
     * Most security-critical for consumeBackupCode (single-use enforcement)
     * and setMfaSecret / enableMfa / disableMfa, where a lost write would
     * undo a security-relevant state change.
     */
    protected function mutateUser(string $username, callable $mutator): void
    {
        $fh = $this->openLocked();
        try {
            $all_users = $this->readLocked($fh);
            $found = false;

            foreach ($all_users as &$u) {
                if ($u['username'] == $username) {
                    $mutator($u);
                    $found = true;
                    break;
                }
            }
            unset($u);

            if (! $found) {
                throw new \Exception('User not found');
            }

            $this->writeLocked($fh, $all_users);
        } finally {
            $this->closeLocked($fh);
        }
    }

    /** @return resource */
    protected function openLocked()
    {
        $fh = @fopen($this->file, 'c+');
        if ($fh === false) {
            throw new \RuntimeException("Could not open users file for locked mutation: {$this->file}");
        }
        if (! flock($fh, LOCK_EX)) {
            fclose($fh);
            throw new \RuntimeException("Could not acquire exclusive lock on users file: {$this->file}");
        }
        return $fh;
    }

    /** @param resource $fh */
    protected function readLocked($fh): array
    {
        rewind($fh);
        $contents = stream_get_contents($fh);
        if ($contents === false || $contents === '') return [];
        $users = json_decode($contents, true);
        return is_array($users) ? $users : [];
    }

    /** @param resource $fh */
    protected function writeLocked($fh, array $users): void
    {
        rewind($fh);
        ftruncate($fh, 0);
        fwrite($fh, json_encode($users));
        fflush($fh);
    }

    /** @param resource $fh */
    protected function closeLocked($fh): void
    {
        @flock($fh, LOCK_UN);
        @fclose($fh);
    }

    protected function buildSessionHash(array $u): string
    {
        return hash('sha256', implode('|', [
            $u['password'] ?? '',
            $u['permissions'] ?? '',
            $u['homedir'] ?? '',
            $u['role'] ?? '',
            ($u['mfa_enabled'] ?? false) ? '1' : '0',
            (string) ($u['email'] ?? ''),
        ]));
    }

    protected function mapToUserObject(array $user): User
    {
        $new = new User();

        $new->setUsername($user['username']);
        $new->setName($user['name']);
        $new->setRole($user['role']);
        $new->setHomedir($user['homedir']);
        $new->setPermissions($user['permissions'], true);

        return $new;
    }

    protected function getUsers(): array
    {
        $users = json_decode((string) file_get_contents($this->file), true);

        return is_array($users) ? $users : [];
    }

    protected function saveUsers(array $users)
    {
        return file_put_contents($this->file, json_encode($users), LOCK_EX);
    }
}
