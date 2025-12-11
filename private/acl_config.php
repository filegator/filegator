<?php

/**
 * FileGator Path-Based ACL Configuration
 *
 * This file defines access control rules based on:
 * - User identity (username or group membership)
 * - Source IP address (CIDR notation supported)
 * - File/folder path
 *
 * See: /mnt/ai/filegator/docs/design/ip-folder-acl-design.md
 */

return [
    /**
     * NOTE: This 'enabled' flag is NOT used by the system.
     * PathACL is enabled/disabled via the 'enabled' setting in configuration.php:
     *
     *   'Filegator\Services\PathACL\PathACLInterface' => [
     *       'config' => [
     *           'enabled' => true,  // <-- THIS controls PathACL activation
     *           'acl_config_file' => __DIR__.'/private/acl_config.php',
     *       ],
     *   ],
     */
    'enabled' => true,

    /**
     * Global ACL settings
     */
    'settings' => [
        'evaluation_mode' => 'most_specific_wins',
        'default_inherit' => true,
        'deny_overrides_allow' => true,
        'cache_enabled' => true,
        'cache_ttl' => 300, // 5 minutes
        'trusted_proxies' => ['127.0.0.1'], // For X-Forwarded-For validation
        'fail_mode' => 'deny', // 'deny' or 'allow' on error
    ],

    /**
     * Default permissions granted when no specific rules match
     * Empty array means deny by default (recommended for security)
     */
    'default_permissions' => ['read'],

    /**
     * Group definitions
     * Groups simplify permission management by grouping users together
     * Use @groupname in rules to reference a group
     */
    'groups' => [
        'developers' => ['john', 'jane'],
        'managers' => ['bob'],
        'guests' => ['guest'],
    ],

    /**
     * Path-based ACL rules
     *
     * Each path can have multiple rules that define who can access it
     * and from which IP addresses. Rules are evaluated in priority order.
     *
     * Rule structure:
     * - users: Array of usernames or @groupname or ['*'] for all authenticated users
     * - ip_inclusions: Array of IPs/CIDR blocks to include (allow), or ['*'] for all
     * - ip_exclusions: Array of IPs/CIDR blocks to exclude (deny, takes precedence)
     * - permissions: Array of permissions granted (read, write, upload, download, delete, zip, chmod)
     * - priority: Higher priority rules evaluated first
     * - override_inherited: If true, replace inherited permissions; if false, merge
     */
    'path_rules' => [
        /**
         * Root level - default permissions for all users
         * These permissions cascade to all subdirectories unless overridden
         */
        '/' => [
            'inherit' => false, // No parent to inherit from
            'rules' => [
                // Default rule: All authenticated users can read from any IP
                [
                    'users' => ['*'], // * = all authenticated users
                    'ip_inclusions' => ['*'], // * = all IP addresses
                    'ip_exclusions' => [], // No IPs excluded
                    'permissions' => ['read'], // Read-only by default
                    'priority' => 0, // Lowest priority (default rule)
                    'override_inherited' => false,
                ],
            ],
        ],
    ],
];
