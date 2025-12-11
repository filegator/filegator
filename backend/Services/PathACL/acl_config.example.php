<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

/**
 * Example PathACL Configuration
 *
 * This file demonstrates the PathACL configuration structure.
 * Copy this file to /private/acl_config.php and customize for your needs.
 *
 * Three-dimensional access control: User + IP + Path → Permissions
 */

return [
    // Note: The 'enabled' setting in configuration.php controls whether PathACL is active.
    // This 'enabled' flag here is for reference only and is not used by the system.
    // Make sure configuration.php has: 'enabled' => true in the PathACL service config.
    'enabled' => true,

    // Global ACL settings
    'settings' => [
        'evaluation_mode' => 'most_specific_wins',  // Algorithm choice
        'default_inherit' => true,                   // Default inheritance behavior
        'deny_overrides_allow' => true,             // Deny always wins
        'cache_enabled' => true,                     // Enable permission caching
        'cache_ttl' => 300,                          // Cache lifetime (seconds)
        'trusted_proxies' => ['127.0.0.1'],         // For X-Forwarded-For validation
        'fail_mode' => 'deny',                       // 'deny', 'allow', or 'fallback' on error
    ],

    // Group definitions (users can belong to groups)
    'groups' => [
        'developers' => ['john', 'jane', 'bob'],
        'contractors' => ['alice', 'charlie'],
        'hr-staff' => ['susan', 'tom'],
        'admins' => ['admin'],
    ],

    // Path-based ACL rules
    'path_rules' => [

        // Root folder (applies to all paths by default)
        '/' => [
            'inherit' => false,  // No parent to inherit from
            'rules' => [
                // Rule 1: Default read-only for all authenticated users
                [
                    'users' => ['*'],  // * = all authenticated users
                    'ip_inclusions' => ['*'],  // * = all IPs
                    'ip_exclusions' => [],
                    'permissions' => ['read'],
                    'priority' => 0,  // Lowest priority
                ],
                // Rule 2: Admins have full access from anywhere
                [
                    'users' => ['@admins'],  // @ prefix for groups
                    'ip_inclusions' => ['*'],
                    'ip_exclusions' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete', 'zip', 'chmod'],
                    'priority' => 100,  // High priority
                ],
            ],
        ],

        // Public folder: everyone can read/download
        '/public' => [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['*'],
                    'ip_inclusions' => ['*'],
                    'ip_exclusions' => [],
                    'permissions' => ['read', 'download'],
                    'priority' => 50,
                    'override_inherited' => false,  // Merge with inherited permissions
                ],
            ],
        ],

        // Projects folder: developers only from office network
        '/projects' => [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['@developers'],
                    'ip_inclusions' => ['192.168.1.0/24', '10.8.0.0/24'],  // Office + VPN
                    'ip_exclusions' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 60,
                    'override_inherited' => true,  // Replace inherited permissions
                ],
            ],
        ],

        // Specific project: limited team members
        '/projects/project-alpha' => [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['john', 'jane'],  // Only specific users
                    'ip_inclusions' => ['*'],
                    'ip_exclusions' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 75,
                    'override_inherited' => true,
                ],
                // Contractors get read-only from VPN only
                [
                    'users' => ['@contractors'],
                    'ip_inclusions' => ['10.8.0.0/24'],
                    'ip_exclusions' => [],
                    'permissions' => ['read', 'download'],
                    'priority' => 70,
                    'override_inherited' => true,
                ],
            ],
        ],

        // HR confidential: strict access
        '/hr/confidential' => [
            'inherit' => false,  // Don't inherit from /hr or /
            'rules' => [
                [
                    'users' => ['@hr-staff', '@admins'],
                    'ip_inclusions' => ['192.168.1.0/24'],  // Office network only
                    'ip_exclusions' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 100,
                ],
            ],
        ],

        // Uploads folder: write-only from internal network
        '/uploads' => [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['*'],
                    'ip_inclusions' => ['192.168.0.0/16', '10.0.0.0/8'],
                    'ip_exclusions' => [],
                    'permissions' => ['upload'],  // Can upload but not read
                    'priority' => 50,
                    'override_inherited' => false,
                ],
            ],
        ],
    ],
];
