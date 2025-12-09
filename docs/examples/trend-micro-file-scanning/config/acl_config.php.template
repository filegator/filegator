<?php

/**
 * Path-Based ACL Configuration for Trend Micro File Scanning Example
 *
 * This configuration implements a three-zone security architecture:
 * - Gateway IP: Can upload and monitor files in staging area
 * - Internal Network: Can download scanned files
 * - File scanning pipeline: Automated via hooks
 *
 * User: john
 * Gateway IP: {{GATEWAY_IP}} (replaced by installer)
 * Internal Network: All other local IPs
 *
 * Permission Matrix:
 * - / (home):     Gateway=Read, Internal=Read
 * - /upload:      Gateway=Upload+Read+Download, Internal=Hidden
 * - /scanned:     Gateway=Hidden, Internal=Read+Download
 * - /download:    Gateway=Hidden, Internal=Read+Upload
 *
 * See: /mnt/ai/filegator/docs/examples/trend-micro-file-scanning/DESIGN.md
 */

return [
    /**
     * Enable Path-Based ACL system
     */
    'enabled' => true,

    /**
     * Global ACL settings
     */
    'settings' => [
        /**
         * Evaluation mode: most specific path rules override parent rules
         */
        'evaluation_mode' => 'most_specific_wins',

        /**
         * Default inheritance behavior
         */
        'default_inherit' => true,

        /**
         * Security: deny always overrides allow
         */
        'deny_overrides_allow' => true,

        /**
         * Enable caching for performance
         */
        'cache_enabled' => true,

        /**
         * Cache TTL in seconds
         */
        'cache_ttl' => 300, // 5 minutes

        /**
         * Trusted proxies for X-Forwarded-For header validation
         * Add your reverse proxy/gateway IP here
         */
        'trusted_proxies' => [
            '127.0.0.1',
            '::1',
            '{{GATEWAY_IP}}', // Gateway server
        ],

        /**
         * Fail secure: deny access on configuration errors
         */
        'fail_mode' => 'deny',
    ],

    /**
     * Default permissions when no rules match
     * Empty = most secure (deny all)
     */
    'default_permissions' => [],

    /**
     * Group definitions
     * Groups are not used in this example (IP-based only)
     */
    'groups' => [
        'gateway' => [],      // Checked via IP rules
        'internal' => [],     // Checked via IP rules
    ],

    /**
     * Path-based ACL rules
     *
     * Rules are evaluated from most specific path to least specific.
     * Higher priority rules are evaluated first within the same path.
     */
    'path_rules' => [

        /**
         * ROOT LEVEL (/)
         * Read-only access for all authenticated users
         */
        '/' => [
            'inherit' => false, // Root has no parent
            'rules' => [
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['*'], // All IPs
                    'ip_denylist' => [],
                    'permissions' => ['read'],
                    'priority' => 10,
                    'description' => 'Default read access to home directory',
                ],
            ],
        ],

        /**
         * UPLOAD FOLDER (/upload)
         * Visible ONLY to gateway IP
         * Gateway can upload, read, and download files in staging area
         * Internal network cannot see this folder at all
         */
        '/upload' => [
            'inherit' => false,
            'rules' => [
                // Gateway IP: Full access to upload folder
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['{{GATEWAY_IP}}'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'upload', 'download'],
                    'priority' => 50,
                    'override_inherited' => true,
                    'description' => 'Gateway can upload and monitor files in staging area',
                ],
                // Internal network: Explicitly denied (folder hidden)
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => ['{{GATEWAY_IP}}'],
                    'permissions' => [], // No permissions = folder is hidden
                    'priority' => 40,
                    'override_inherited' => true,
                    'description' => 'Internal network cannot see upload folder',
                ],
            ],
        ],

        /**
         * SCANNED FOLDER (/scanned)
         * Visible ONLY to internal network
         * Contains clean files that passed Trend Micro scanning
         * Gateway cannot see this folder
         */
        '/scanned' => [
            'inherit' => false,
            'rules' => [
                // Internal network: Read and download clean files
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => ['{{GATEWAY_IP}}'],
                    'permissions' => ['read', 'download'],
                    'priority' => 50,
                    'override_inherited' => true,
                    'description' => 'Internal users can download scanned files',
                ],
                // Gateway: Explicitly denied (folder hidden)
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['{{GATEWAY_IP}}'],
                    'ip_denylist' => [],
                    'permissions' => [], // No permissions = folder is hidden
                    'priority' => 40,
                    'override_inherited' => true,
                    'description' => 'Gateway cannot see scanned folder',
                ],
            ],
        ],

        /**
         * DOWNLOAD FOLDER (/download)
         * Upload destination for INTERNAL NETWORK ONLY
         * Files uploaded here are automatically moved to /upload by hooks
         * Gateway cannot see this folder - they upload directly to /upload
         */
        '/download' => [
            'inherit' => false,
            'rules' => [
                // Internal network: Read and upload
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => ['{{GATEWAY_IP}}'],
                    'permissions' => ['read', 'upload'],
                    'priority' => 50,
                    'override_inherited' => true,
                    'description' => 'Internal users upload here for outbound file transfer',
                ],
                // Gateway: Explicitly denied (folder hidden)
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['{{GATEWAY_IP}}'],
                    'ip_denylist' => [],
                    'permissions' => [], // No permissions = folder is hidden
                    'priority' => 40,
                    'override_inherited' => true,
                    'description' => 'Gateway cannot see download folder',
                ],
            ],
        ],
    ],

    /**
     * WORKFLOW SUMMARY:
     *
     * External Upload Flow:
     * 1. Gateway uploads to /download
     * 2. Hook moves file to /upload
     * 3. Hook scans with Trend Micro
     * 4. Clean: moved to /scanned
     * 5. Malware: deleted and admin notified
     *
     * Internal Upload Flow:
     * 1. Internal user uploads to /download
     * 2. Hook moves file to /upload
     * 3. Hook scans with Trend Micro
     * 4. Clean: moved to /scanned (user can download)
     * 5. Malware: deleted and admin notified
     *
     * SECURITY NOTES:
     * - Gateway cannot download from /scanned (data exfiltration prevention)
     * - Internal users cannot see /upload (staging area isolation)
     * - All uploads are scanned before becoming available
     * - Malware is deleted immediately and logged
     */
];
