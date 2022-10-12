<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Auth\Adapters;

use Dibi\Connection;
use Dibi\Row;
use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Auth\User;
use Filegator\Services\Auth\UsersCollection;
use Filegator\Services\Service;
use Filegator\Services\Session\SessionStorageInterface as Session;
use Filegator\Utils\PasswordHash;

class Database implements Service, AuthInterface
{
    use PasswordHash;

    const SESSION_KEY = 'database_auth';
    const SESSION_HASH = 'database_auth_hash';

    const GUEST_USERNAME = 'guest';

    protected $session;

    protected $conn;

    public function __construct(Session $session)
    {
        $this->session = $session;
    }

    public function init(array $config = [])
    {
        $this->conn = new Connection($config);
    }

    public function getConnection()
    {
        return $this->conn;
    }

    public function user(): ?User
    {
        if (! $this->session) return null;

        $user = $this->session->get(self::SESSION_KEY, null);
        $hash = $this->session->get(self::SESSION_HASH, null);

        if (! $user) return null;

        $ret = $this->getConnection()
            ->fetch('SELECT * FROM users WHERE username = ?', $user->getUsername())
        ;

        if ($ret && $hash == $ret->password.$ret->permissions.$ret->homedir.$ret->role) {
            return $user;
        }

        return null;
    }

    public function authenticate($username, $password): bool
    {
        $ret = $this->getConnection()
            ->fetch('SELECT * FROM users WHERE username = ?', $username)
        ;

        if ($ret && $this->verifyPassword($password, $ret->password)) {
            $user = $this->mapToUserObject($ret);
            $this->store($user);
            $this->session->set(self::SESSION_HASH, $ret->password.$ret->permissions.$ret->homedir.$ret->role);

            return true;
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
        if (! $this->find($username)) {
            throw new \Exception('User not found');
        }

        if ($username != $user->getUsername() && $this->find($user->getUsername())) {
            throw new \Exception('Username already taken');
        }

        $this->getConnection()->query('UPDATE users SET', [
            'username' => $user->getUsername(),
            'name' => $user->getName(),
            'homedir' => $user->getHomeDir(),
            'permissions' => $user->getPermissions(true),
            'role' => $user->getRole(),
        ], 'WHERE username = ?', $username);

        if ($password) {
            $this->getConnection()->query('UPDATE users SET', [
                'password' => $this->hashPassword($password),
            ], 'WHERE username = ?', $username);
        }

        return $this->find($user->getUsername()) ?: $user;
    }

    public function add(User $user, $password): User
    {
        if ($this->find($user->getUsername())) {
            throw new \Exception('Username already taken');
        }

        $this->getConnection()->query('INSERT INTO users', [
            'username' => $user->getUsername(),
            'name' => $user->getName(),
            'role' => $user->getRole(),
            'homedir' => $user->getHomeDir(),
            'permissions' => $user->getPermissions(true),
            'password' => $this->hashPassword($password),
        ]);

        return $this->find($user->getUsername()) ?: $user;
    }

    public function delete(User $user)
    {
        if (! $this->find($user->getUsername())) {
            throw new \Exception('User not found');
        }

        $this->getConnection()->query('DELETE FROM users WHERE username = ?', $user->getUsername());

        return true;
    }

    public function find($username): ?User
    {
        $row = $this->getConnection()
            ->fetch('SELECT * FROM users WHERE username = ?', $username)
        ;

        if ($row) {
            return $this->mapToUserObject($row);
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

        $rows = $this->getConnection()
            ->fetchAll('SELECT * FROM users')
        ;

        foreach ($rows as $user) {
            $users->addUser($this->mapToUserObject($user));
        }

        return $users;
    }

    protected function mapToUserObject(Row $user): User
    {
        $new = new User();

        $new->setRole(isset($user->role) ? $user->role : 'guest');
        $new->setHomedir(isset($user->homedir) ? $user->homedir : '/');
        $new->setPermissions(isset($user->permissions) ? $user->permissions : '', true);
        $new->setUsername(isset($user->username) ? $user->username : '');
        $new->setName(isset($user->name) ? $user->name : 'Guest');

        return $new;
    }
}
