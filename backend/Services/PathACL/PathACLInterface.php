<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\PathACL;

use Filegator\Services\Auth\User;
use Filegator\Services\Service;

/**
 * PathACL Interface - Three-dimensional access control (User + IP + Path)
 *
 * This interface defines the contract for path-based access control that
 * combines user identity, source IP address, and folder paths to determine
 * effective permissions.
 */
interface PathACLInterface extends Service
{
    /**
     * Check if user can perform permission on path from IP address.
     *
     * This is the primary method for permission checks. It evaluates all
     * applicable ACL rules considering user identity, client IP, and path
     * inheritance to determine if the requested permission is granted.
     *
     * @param User $user Current authenticated user
     * @param string $clientIp Client IP address (IPv4 or IPv6)
     * @param string $path File/folder path (relative to repository root)
     * @param string $permission Permission to check (read, write, upload, delete, etc.)
     * @return bool True if allowed, false otherwise
     */
    public function checkPermission(User $user, string $clientIp, string $path, string $permission): bool;

    /**
     * Get effective permissions for user on path from IP.
     *
     * Returns the complete set of permissions granted to the user for the
     * specified path and IP address. This is useful for determining all
     * capabilities at once rather than checking individual permissions.
     *
     * @param User $user Current authenticated user
     * @param string $clientIp Client IP address
     * @param string $path File/folder path
     * @return array Array of granted permission strings (e.g., ['read', 'write', 'upload'])
     */
    public function getEffectivePermissions(User $user, string $clientIp, string $path): array;

    /**
     * Check if path-based ACL system is enabled.
     *
     * When disabled, the system falls back to FileGator's global permission
     * system (user permissions from users.json).
     *
     * @return bool True if ACL evaluation is enabled
     */
    public function isEnabled(): bool;

    /**
     * Clear permission cache (call after ACL config changes).
     *
     * Invalidates all cached permission evaluations. Should be called
     * whenever the ACL configuration is modified to ensure users see
     * updated permissions immediately.
     *
     * @return void
     */
    public function clearCache(): void;

    /**
     * Get detailed information about permission decision (for debugging).
     *
     * Returns a comprehensive explanation of why a permission was granted
     * or denied, including matched rules, effective permissions, and the
     * evaluation reasoning. Useful for troubleshooting ACL configurations.
     *
     * @param User $user Current authenticated user
     * @param string $clientIp Client IP address
     * @param string $path File/folder path
     * @param string $permission Permission to check
     * @return array Decision details including:
     *               - 'allowed': bool
     *               - 'reason': string explanation
     *               - 'matched_rules': array of rules that matched
     *               - 'effective_permissions': array of granted permissions
     *               - 'requested_permission': string
     *               - 'user_ip_check': bool result of user-level IP check
     *               - 'evaluation_path': array of paths traversed
     */
    public function explainPermission(User $user, string $clientIp, string $path, string $permission): array;
}
