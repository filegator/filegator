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

    /** @var string[] List of folder paths this user can access. Single-folder
     *  users have a 1-element list; multi-folder users have more. The legacy
     *  scalar setHomedir/getHomeDir methods are kept as deprecated shims over
     *  setHomedirs/getHomeDirs so callers that haven't migrated still work. */
    protected $homedirs = [];

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

    /**
     * Replace the user's full folder list. Each entry is trimmed and the
     * array is re-indexed; blank/non-string entries are dropped.
     */
    public function setHomedirs(array $homedirs): void
    {
        $clean = [];
        foreach ($homedirs as $h) {
            if (! is_string($h)) continue;
            $t = trim($h);
            if ($t === '') continue;
            $clean[] = $t;
        }
        $this->homedirs = array_values($clean);
    }

    /** @return string[] */
    public function getHomeDirs(): array
    {
        return $this->homedirs;
    }

    /**
     * Legacy single-folder setter. Shim over setHomedirs.
     * @deprecated Use setHomedirs() — kept so LDAP/WPAuth adapters and any
     *             single-folder call sites keep working without edits.
     */
    public function setHomedir(string $homedir)
    {
        $this->setHomedirs([$homedir]);
    }

    /**
     * Legacy single-folder getter. Returns the first homedir, or '' if none.
     * @deprecated Use getHomeDirs() — kept for back-compat through the
     *             rollout. Phase 10 removes it.
     */
    public function getHomeDir(): string
    {
        return $this->homedirs[0] ?? '';
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
        // `homedir` (singular) is preserved for one release so existing
        // frontend code that reads the scalar keeps working. `homedirs`
        // (plural) is the new authoritative key. Phase 10 drops `homedir`.
        return [
            'role' => $this->getRole(),
            'permissions' => $this->getPermissions(),
            'homedir' => $this->getHomeDir(),
            'homedirs' => $this->getHomeDirs(),
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
