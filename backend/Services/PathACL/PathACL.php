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

/**
 * PathACL Service - Three-dimensional access control (User + IP + Path)
 *
 * Implements path-based access control that combines:
 * - User identity (username, group membership)
 * - Source IP address (with CIDR support)
 * - Folder path (with inheritance)
 *
 * To determine effective permissions for file operations.
 *
 * Features:
 * - Cascading inheritance from parent folders
 * - Priority-based rule evaluation
 * - Group-based permissions
 * - IP inclusions/exclusions support
 * - Permission caching for performance
 * - Detailed permission explanation for debugging
 */
class PathACL implements PathACLInterface
{
    /**
     * @var array ACL configuration
     */
    protected $config = [];

    /**
     * @var bool Whether PathACL system is enabled
     */
    protected $enabled = false;

    /**
     * @var array Path-based ACL rules
     */
    protected $pathRules = [];

    /**
     * @var array Group definitions (groupname => [usernames])
     */
    protected $groups = [];

    /**
     * @var array User to groups mapping (username => [groupnames])
     */
    protected $userGroups = [];

    /**
     * @var IpMatcher IP matching utility
     */
    protected $ipMatcher;

    /**
     * @var PathMatcher Path matching utility
     */
    protected $pathMatcher;

    /**
     * @var array Permission cache (cache_key => bool)
     */
    protected $cache = [];

    /**
     * @var bool Whether caching is enabled
     */
    protected $cacheEnabled = true;

    /**
     * @var int Cache TTL in seconds
     */
    protected $cacheTtl = 300;

    /**
     * @var string Fail mode: 'deny', 'allow', or 'fallback'
     */
    protected $failMode = 'deny';

    /**
     * Initialize PathACL service.
     *
     * @param array $config Configuration array
     * @return void
     */
    public function init(array $config = [])
    {
        $this->config = $config;
        $this->ipMatcher = new IpMatcher();
        $this->pathMatcher = new PathMatcher();

        // Load ACL configuration
        $this->loadConfig($config);
    }

    /**
     * Load ACL configuration from file or array.
     *
     * @param array $config Configuration array
     * @return void
     */
    protected function loadConfig(array $config)
    {
        try {
            // Check if enabled
            $this->enabled = $config['enabled'] ?? false;

            if (!$this->enabled) {
                return;
            }

            // Load from external file if specified
            $aclConfigFile = $config['acl_config_file'] ?? null;

            if ($aclConfigFile !== null) {
                // Check if file exists
                if (!file_exists($aclConfigFile)) {
                    error_log('PathACL: Configuration file does not exist: ' . $aclConfigFile);
                    $this->handleConfigError(new \Exception('ACL config file not found: ' . $aclConfigFile));
                    return;
                }

                // Check if file is readable (prevents fatal error from require)
                if (!is_readable($aclConfigFile)) {
                    error_log('PathACL: Configuration file is not readable: ' . $aclConfigFile);
                    error_log('PathACL: Please check file permissions. The web server (www-data) must be able to read this file.');
                    error_log('PathACL: Try: sudo chown www-data:www-data ' . $aclConfigFile . ' && sudo chmod 600 ' . $aclConfigFile);
                    $this->handleConfigError(new \Exception('ACL config file not readable: ' . $aclConfigFile . '. Check file permissions.'));
                    return;
                }

                $aclConfig = require $aclConfigFile;
            } else {
                $aclConfig = $config;
            }

            // Load settings
            $settings = $aclConfig['settings'] ?? [];
            $this->cacheEnabled = $settings['cache_enabled'] ?? true;
            $this->cacheTtl = $settings['cache_ttl'] ?? 300;
            $this->failMode = $settings['fail_mode'] ?? 'deny';

            // Load groups
            $this->groups = $aclConfig['groups'] ?? [];
            $this->userGroups = $this->buildUserGroupMap($this->groups);

            // Load path rules
            $this->pathRules = $aclConfig['path_rules'] ?? [];

            // Validate and pre-process rules
            $this->preprocessRules();

        } catch (\Exception $e) {
            // Handle configuration errors based on fail mode
            $this->handleConfigError($e);
        }
    }

    /**
     * Build reverse mapping from username to groups.
     *
     * @param array $groups Group definitions (groupname => [usernames])
     * @return array User to groups mapping (username => [groupnames])
     */
    protected function buildUserGroupMap(array $groups): array
    {
        $userGroups = [];

        foreach ($groups as $groupName => $members) {
            foreach ($members as $username) {
                if (!isset($userGroups[$username])) {
                    $userGroups[$username] = [];
                }
                $userGroups[$username][] = $groupName;
            }
        }

        return $userGroups;
    }

    /**
     * Pre-process and validate ACL rules.
     *
     * @return void
     */
    protected function preprocessRules()
    {
        foreach ($this->pathRules as $path => &$acl) {
            // Ensure inherit flag is set
            if (!isset($acl['inherit'])) {
                $acl['inherit'] = true;
            }

            // Validate rules array
            if (!isset($acl['rules']) || !is_array($acl['rules'])) {
                $acl['rules'] = [];
            }

            // Add rule order for sorting
            foreach ($acl['rules'] as $index => &$rule) {
                $rule['_order'] = $index;

                // Set default priority if not specified
                if (!isset($rule['priority'])) {
                    $rule['priority'] = 0;
                }

                // Set default override_inherited if not specified
                if (!isset($rule['override_inherited'])) {
                    $rule['override_inherited'] = false;
                }

                // Validate IP lists
                $rule['ip_inclusions'] = $rule['ip_inclusions'] ?? [];
                $rule['ip_exclusions'] = $rule['ip_exclusions'] ?? [];
            }
        }
    }

    /**
     * Handle configuration loading errors.
     *
     * @param \Exception $e Exception that occurred
     * @return void
     */
    protected function handleConfigError(\Exception $e)
    {
        // Log error (in production, use proper logger)
        error_log('PathACL configuration error: ' . $e->getMessage());

        // Disable ACL system on error
        $this->enabled = false;
    }

    /**
     * Check if user can perform permission on path from IP address.
     *
     * @param User $user Current authenticated user
     * @param string $clientIp Client IP address
     * @param string $path File/folder path
     * @param string $permission Permission to check
     * @return bool True if allowed, false otherwise
     */
    public function checkPermission(User $user, string $clientIp, string $path, string $permission): bool
    {
        // Debug: Log every permission check
        error_log("[PathACL DEBUG] PathACL::checkPermission - user: " . $user->getUsername() . ", IP: " . $clientIp . ", path: " . $path . ", permission: " . $permission);

        // If ACL is disabled, return true (fall back to global permissions)
        if (!$this->enabled) {
            error_log("[PathACL DEBUG] PathACL::checkPermission - DISABLED, returning TRUE (allow all)");
            return true;
        }

        try {
            // Check cache first
            if ($this->cacheEnabled) {
                $cacheKey = $this->getCacheKey($user, $clientIp, $path, $permission);
                if (isset($this->cache[$cacheKey])) {
                    $cachedResult = $this->cache[$cacheKey];
                    error_log("[PathACL DEBUG] PathACL::checkPermission - Cache hit, returning: " . ($cachedResult ? 'ALLOW' : 'DENY'));
                    return $cachedResult;
                }
            }

            // Step 1: User-level IP check
            if (!$this->checkUserIpAccess($user, $clientIp)) {
                error_log("[PathACL DEBUG] PathACL::checkPermission - User-level IP check FAILED, returning DENY");
                return $this->cacheResult($user, $clientIp, $path, $permission, false);
            }

            // Step 2: Get effective permissions for this path
            $effectivePerms = $this->getEffectivePermissions($user, $clientIp, $path);
            error_log("[PathACL DEBUG] PathACL::checkPermission - Effective permissions: " . json_encode($effectivePerms));

            // Step 3: Check if requested permission is granted
            $allowed = in_array($permission, $effectivePerms);
            error_log("[PathACL DEBUG] PathACL::checkPermission - Final decision: " . ($allowed ? 'ALLOW' : 'DENY'));

            return $this->cacheResult($user, $clientIp, $path, $permission, $allowed);

        } catch (\Exception $e) {
            // Handle errors based on fail mode
            error_log('PathACL error: ' . $e->getMessage());

            if ($this->failMode === 'allow') {
                return true;
            } elseif ($this->failMode === 'fallback') {
                return true; // Fallback to global permissions
            } else {
                return false; // Default deny
            }
        }
    }

    /**
     * Get effective permissions for user on path from IP.
     *
     * @param User $user Current authenticated user
     * @param string $clientIp Client IP address
     * @param string $path File/folder path
     * @return array Array of granted permissions
     */
    public function getEffectivePermissions(User $user, string $clientIp, string $path): array
    {
        try {
            // Normalize path
            $normalizedPath = $this->pathMatcher->normalizePath($path);

            // Step 1: Find all matching rules
            $matchingRules = $this->findMatchingRules($user, $clientIp, $normalizedPath);

            // Step 2: Sort rules by specificity and priority
            $sortedRules = $this->sortRules($matchingRules);

            // Step 3: Merge permissions
            $effectivePermissions = $this->mergePermissions($sortedRules);

            return $effectivePermissions;

        } catch (\Exception $e) {
            error_log('PathACL error in getEffectivePermissions: ' . $e->getMessage());
            return [];
        }
    }

    /**
     * Check user-level IP access (from user profile).
     *
     * @param User $user User to check
     * @param string $clientIp Client IP address
     * @return bool True if IP is allowed for this user
     */
    protected function checkUserIpAccess(User $user, string $clientIp): bool
    {
        $inclusions = $user->getIpInclusions();
        $exclusions = $user->getIpExclusions();

        // If no IP restrictions are set at user level, allow
        if (empty($inclusions) && empty($exclusions)) {
            return true;
        }

        // Use IpMatcher to evaluate user-level IP restrictions
        return $this->ipMatcher->isAllowed($clientIp, $inclusions, $exclusions);
    }

    /**
     * Find all ACL rules that match user, IP, and path.
     *
     * @param User $user User to match
     * @param string $clientIp Client IP address
     * @param string $path Normalized path
     * @return array Array of matching rules with metadata
     */
    protected function findMatchingRules(User $user, string $clientIp, string $path): array
    {
        $matchingRules = [];
        $ancestors = $this->pathMatcher->getPathAncestors($path);

        foreach ($ancestors as $ancestorPath) {
            // Check if this path has ACL rules
            if (!isset($this->pathRules[$ancestorPath])) {
                continue;
            }

            $pathAcl = $this->pathRules[$ancestorPath];

            // Evaluate each rule for this path
            foreach ($pathAcl['rules'] as $rule) {
                // Check if user matches
                if (!$this->userMatchesRule($user, $rule['users'])) {
                    continue;
                }

                // Check if IP matches
                if (!$this->ipMatchesRule($clientIp, $rule['ip_inclusions'], $rule['ip_exclusions'])) {
                    continue;
                }

                // Rule matches - add with metadata
                $matchingRules[] = [
                    'rule' => $rule,
                    'path' => $ancestorPath,
                    'specificity' => $this->pathMatcher->getPathDepth($ancestorPath),
                ];
            }

            // Stop traversal if inheritance is disabled at this path
            if (!$pathAcl['inherit']) {
                break;
            }
        }

        return $matchingRules;
    }

    /**
     * Check if user matches rule's user specification.
     *
     * @param User $user User to check
     * @param array $ruleUsers Rule's user specification
     * @return bool True if user matches
     */
    protected function userMatchesRule(User $user, array $ruleUsers): bool
    {
        // Wildcard matches all authenticated users
        if (in_array('*', $ruleUsers, true)) {
            return true;
        }

        $username = $user->getUsername();

        // Check direct username match
        if (in_array($username, $ruleUsers, true)) {
            return true;
        }

        // Check group membership
        foreach ($ruleUsers as $ruleUser) {
            if (str_starts_with($ruleUser, '@')) {
                $groupName = substr($ruleUser, 1);
                if ($this->userInGroup($username, $groupName)) {
                    return true;
                }
            }
        }

        return false;
    }

    /**
     * Check if user is member of group.
     *
     * @param string $username Username to check
     * @param string $groupName Group name
     * @return bool True if user is in group
     */
    protected function userInGroup(string $username, string $groupName): bool
    {
        return isset($this->userGroups[$username]) &&
               in_array($groupName, $this->userGroups[$username], true);
    }

    /**
     * Check if IP matches rule's IP restrictions.
     *
     * @param string $clientIp Client IP address
     * @param array $inclusions IP inclusions (IPs to include/allow)
     * @param array $exclusions IP exclusions (IPs to exclude/deny)
     * @return bool True if IP is allowed
     */
    protected function ipMatchesRule(string $clientIp, array $inclusions, array $exclusions): bool
    {
        return $this->ipMatcher->isAllowed($clientIp, $inclusions, $exclusions);
    }

    /**
     * Sort rules by specificity, priority, and order.
     *
     * @param array $rules Rules to sort
     * @return array Sorted rules
     */
    protected function sortRules(array $rules): array
    {
        usort($rules, function ($a, $b) {
            // Primary: More specific paths first (higher depth)
            if ($a['specificity'] !== $b['specificity']) {
                return $b['specificity'] <=> $a['specificity'];
            }

            // Secondary: Higher priority first
            if ($a['rule']['priority'] !== $b['rule']['priority']) {
                return $b['rule']['priority'] <=> $a['rule']['priority'];
            }

            // Tertiary: Earlier rules first (lower order)
            return $a['rule']['_order'] <=> $b['rule']['_order'];
        });

        return $rules;
    }

    /**
     * Merge permissions from sorted rules.
     *
     * @param array $sortedRules Sorted matching rules
     * @return array Effective permissions
     */
    protected function mergePermissions(array $sortedRules): array
    {
        $effectivePermissions = [];

        foreach ($sortedRules as $match) {
            $rule = $match['rule'];
            $rulePermissions = $rule['permissions'] ?? [];

            if ($rule['override_inherited']) {
                // Replace all permissions and stop
                $effectivePermissions = $rulePermissions;
                break;
            } else {
                // Merge permissions (union)
                $effectivePermissions = array_unique(array_merge($effectivePermissions, $rulePermissions));
            }
        }

        return $effectivePermissions;
    }

    /**
     * Check if path-based ACL system is enabled.
     *
     * @return bool True if enabled
     */
    public function isEnabled(): bool
    {
        return $this->enabled;
    }

    /**
     * Clear permission cache.
     *
     * @return void
     */
    public function clearCache(): void
    {
        $this->cache = [];
    }

    /**
     * Get detailed information about permission decision.
     *
     * @param User $user Current authenticated user
     * @param string $clientIp Client IP address
     * @param string $path File/folder path
     * @param string $permission Permission to check
     * @return array Decision details
     */
    public function explainPermission(User $user, string $clientIp, string $path, string $permission): array
    {
        try {
            $normalizedPath = $this->pathMatcher->normalizePath($path);

            // User-level IP check
            $userIpCheck = $this->checkUserIpAccess($user, $clientIp);

            if (!$userIpCheck) {
                return [
                    'allowed' => false,
                    'reason' => 'User IP access denied at user level',
                    'user_ip_check' => false,
                    'matched_rules' => [],
                    'effective_permissions' => [],
                    'requested_permission' => $permission,
                    'evaluation_path' => [],
                ];
            }

            // Find matching rules
            $matchingRules = $this->findMatchingRules($user, $clientIp, $normalizedPath);
            $sortedRules = $this->sortRules($matchingRules);
            $effectivePermissions = $this->mergePermissions($sortedRules);

            // Check if permission is granted
            $allowed = in_array($permission, $effectivePermissions);

            $reason = $allowed
                ? "Permission '{$permission}' granted by ACL rules"
                : (empty($effectivePermissions)
                    ? 'No matching ACL rules found'
                    : "Permission '{$permission}' not in effective permissions");

            return [
                'allowed' => $allowed,
                'reason' => $reason,
                'user_ip_check' => true,
                'matched_rules' => $matchingRules,
                'effective_permissions' => $effectivePermissions,
                'requested_permission' => $permission,
                'evaluation_path' => $this->pathMatcher->getPathAncestors($normalizedPath),
            ];

        } catch (\Exception $e) {
            return [
                'allowed' => false,
                'reason' => 'Error during evaluation: ' . $e->getMessage(),
                'user_ip_check' => false,
                'matched_rules' => [],
                'effective_permissions' => [],
                'requested_permission' => $permission,
                'evaluation_path' => [],
            ];
        }
    }

    /**
     * Generate cache key for permission check.
     *
     * @param User $user User
     * @param string $clientIp Client IP
     * @param string $path Path
     * @param string $permission Permission
     * @return string Cache key
     */
    protected function getCacheKey(User $user, string $clientIp, string $path, string $permission): string
    {
        return md5(sprintf(
            'acl:%s:%s:%s:%s',
            $user->getUsername(),
            $clientIp,
            $path,
            $permission
        ));
    }

    /**
     * Cache permission check result.
     *
     * @param User $user User
     * @param string $clientIp Client IP
     * @param string $path Path
     * @param string $permission Permission
     * @param bool $result Result to cache
     * @return bool The result (for chaining)
     */
    protected function cacheResult(User $user, string $clientIp, string $path, string $permission, bool $result): bool
    {
        if ($this->cacheEnabled) {
            $cacheKey = $this->getCacheKey($user, $clientIp, $path, $permission);
            $this->cache[$cacheKey] = $result;
        }

        return $result;
    }
}
