<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Auth;

class User implements \JsonSerializable
{
    protected $role = 'guest';

    protected $permissions = [];

    protected $username = '';

    protected $homedir = '';

    protected $name = '';

    protected $available_roles = ['guest', 'user', 'admin'];

    protected $available_permissions = ['read', 'write', 'upload', 'download', 'batchdownload', 'zip', 'chmod'];

    public function __construct()
    {
    }

    public function isGuest(): bool
    {
        return 'guest' == $this->role;
    }

    public function isUser(): bool
    {
        return 'user' == $this->role;
    }

    public function isAdmin(): bool
    {
        return 'admin' == $this->role;
    }

    public function hasRole($check): bool
    {
        if (is_array($check)) {
            return in_array($this->getRole(), $check);
        }

        return $this->getRole() == $check;
    }

    public function getRole(): string
    {
        return $this->role;
    }

    public function hasPermissions($check): bool
    {
        if (empty($check)) {
            return true;
        }

        if (is_array($check)) {
            return count(array_intersect($check, $this->getPermissions())) == count($check);
        }

        return in_array($check, $this->getPermissions());
    }

    public function setName(string $name): void
    {
        $this->name = $name;
    }

    public function getName(): string
    {
        return $this->name;
    }

    public function setUsername(string $username)
    {
        $this->username = $username;
    }

    public function getUsername(): string
    {
        return $this->username;
    }

    public function setHomedir(string $homedir)
    {
        $this->homedir = $homedir;
    }

    public function getHomeDir(): string
    {
        return $this->homedir;
    }

    public function setRole(string $role)
    {
        $this->checkValidRole($role);

        $this->role = $role;
    }

    public function setPermissions($permissions, $encoded = false)
    {
        if ($encoded) {
            $permissions = explode('|', $permissions);
        }

        $this->checkValidPermissions($permissions);

        $this->permissions = $permissions;
    }

    public function getPermissions($encoded = false)
    {
        return $encoded ? implode('|', $this->permissions) : $this->permissions;
    }

    public function jsonSerialize()
    {
        return [
            'role' => $this->getRole(),
            'permissions' => $this->getPermissions(),
            'homedir' => $this->getHomeDir(),
            'username' => $this->getUsername(),
            'name' => $this->getName(),
        ];
    }

    protected function checkValidRole($role)
    {
        if (! in_array($role, $this->available_roles)) {
            throw new \Exception("User role {$role} does not exists.");
        }

        return true;
    }

    protected function checkValidPermissions(array $permissions)
    {
        foreach ($permissions as $permission) {
            if ($permission && ! in_array($permission, $this->available_permissions)) {
                throw new \Exception("Permission {$permission} does not exists.");
            }
        }

        return true;
    }
}
