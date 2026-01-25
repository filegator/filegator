# Path ACL Configuration Reference

This document provides a complete reference for configuring FileGator's Path-Based ACL system.

## Configuration Files

Path ACL uses two configuration files:

1. **`configuration.php`** - Enables the PathACL service
2. **`private/acl_config.php`** - Contains all ACL rules and settings

## Service Registration

Enable PathACL in your `configuration.php`:

```php
'services' => [
    // ... other services ...

    'Filegator\Services\PathACL\PathACLInterface' => [
        'handler' => '\Filegator\Services\PathACL\PathACL',
        'config' => [
            'acl_config_file' => __DIR__.'/private/acl_config.php',
            'enabled' => true,
        ],
    ],
],
```

### Service Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `acl_config_file` | string | `''` | Path to ACL configuration file |
| `enabled` | bool | `true` | Enable/disable PathACL system |

## ACL Configuration Structure

The `/private/acl_config.php` file contains the complete ACL configuration:

```php
<?php
return [
    'enabled' => true,
    'settings' => [ /* ... */ ],
    'groups' => [ /* ... */ ],
    'path_rules' => [ /* ... */ ],
];
```

## Global Settings

The `settings` section controls how the ACL system operates:

```php
'settings' => [
    'evaluation_mode' => 'most_specific_wins',
    'default_inherit' => true,
    'deny_overrides_allow' => true,
    'cache_enabled' => true,
    'cache_ttl' => 300,
    'trusted_proxies' => ['127.0.0.1'],
    'fail_mode' => 'deny',
],
```

### Settings Reference

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `evaluation_mode` | string | `'most_specific_wins'` | Algorithm for rule evaluation |
| `default_inherit` | bool | `true` | Default inheritance behavior |
| `deny_overrides_allow` | bool | `true` | Whether deny always wins over allow |
| `cache_enabled` | bool | `true` | Enable permission result caching |
| `cache_ttl` | int | `300` | Cache lifetime in seconds |
| `trusted_proxies` | array | `['127.0.0.1']` | Trusted proxy IPs for X-Forwarded-For |
| `fail_mode` | string | `'deny'` | Behavior on errors: `'deny'`, `'allow'`, or `'fallback'` |

### Evaluation Modes

- **`most_specific_wins`**: Rules from deeper paths override parent rules (recommended)
- **`priority_based`**: Priority value determines rule order regardless of path depth
- **`deny_priority`**: Deny rules always processed first

### Fail Modes

- **`deny`**: Deny all access when configuration errors occur (most secure)
- **`allow`**: Allow access when errors occur (most permissive)
- **`fallback`**: Fall back to global user permissions (balanced)

## Group Definitions

Groups allow you to manage permissions for multiple users at once:

```php
'groups' => [
    'developers' => ['john', 'jane', 'bob'],
    'contractors' => ['alice', 'charlie'],
    'hr-staff' => ['susan', 'tom', 'mary'],
    'admins' => ['admin', 'root'],
],
```

### Using Groups in Rules

Reference groups with the `@` prefix:

```php
'users' => ['@developers']  // All users in the developers group
```

Users can belong to multiple groups, and permissions are merged (union).

## Path Rules

The `path_rules` section defines permissions for each path:

```php
'path_rules' => [
    '/' => [
        'inherit' => false,
        'rules' => [ /* ... */ ],
    ],
    '/projects' => [
        'inherit' => true,
        'rules' => [ /* ... */ ],
    ],
],
```

### Path Rule Structure

Each path has:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `inherit` | bool | No | Whether to inherit permissions from parent (default: `true`) |
| `rules` | array | Yes | Array of permission rules for this path |

### Rule Structure

Each rule within a path contains:

```php
[
    'users' => ['john', '@developers', '*'],
    'ip_allowlist' => ['192.168.1.0/24', '10.8.0.0/24'],
    'ip_denylist' => ['192.168.1.99'],
    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
    'priority' => 50,
    'override_inherited' => true,
]
```

### Rule Properties

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `users` | array | Yes | List of usernames, groups (`@group`), or `['*']` for all |
| `ip_allowlist` | array | Yes | IPs/subnets to allow, or `['*']` for all |
| `ip_denylist` | array | No | IPs/subnets to explicitly deny (overrides allow) |
| `permissions` | array | Yes | List of granted permissions |
| `priority` | int | No | Rule priority (higher = processed first), default: `0` |
| `override_inherited` | bool | No | Replace inherited permissions (vs merge), default: `false` |

## IP Specification Formats

The ACL system supports multiple IP address formats:

### Single IP Addresses

```php
'ip_allowlist' => ['192.168.1.50']  // Exact IP match
```

### CIDR Notation (IPv4)

```php
'ip_allowlist' => [
    '192.168.1.0/24',    // 192.168.1.0 - 192.168.1.255 (256 IPs)
    '10.0.0.0/8',        // 10.0.0.0 - 10.255.255.255 (16M IPs)
]
```

### IPv6 Support

```php
'ip_allowlist' => [
    '2001:db8::1',       // Single IPv6 address
    '2001:db8::/32',     // IPv6 subnet
]
```

### Wildcard (All IPs)

```php
'ip_allowlist' => ['*']  // Matches all IP addresses
```

### Allow/Deny List Behavior

**Evaluation Order**:
1. Check IP against `ip_denylist` - if match, DENY immediately
2. If `ip_allowlist` is non-empty, check IP against it
3. If no match in allowlist, DENY
4. If `ip_allowlist` is empty (or `['*']`), ALLOW

**Examples**:

```php
// Allow subnet, deny specific IP
'ip_allowlist' => ['192.168.1.0/24'],
'ip_denylist' => ['192.168.1.99'],
// Result: 192.168.1.1-98 and 192.168.1.100-255 allowed, .99 denied

// Public access with blacklist
'ip_allowlist' => ['*'],
'ip_denylist' => ['198.51.100.0/24', '203.0.113.50'],
// Result: All IPs allowed except the denied subnet and IP
```

## Permission Types

Available permissions in FileGator:

| Permission | Description |
|------------|-------------|
| `read` | View files, list directories |
| `write` | Create and modify files |
| `upload` | Upload files via HTTP |
| `download` | Download files via HTTP |
| `batchdownload` | Download multiple files as ZIP |
| `delete` | Delete files and folders |
| `zip` | Create ZIP archives |
| `chmod` | Change file permissions (Unix) |

### Permission Examples

```php
// Read-only access
'permissions' => ['read', 'download']

// Full access
'permissions' => ['read', 'write', 'upload', 'download', 'delete', 'zip', 'chmod']

// Upload-only (no read)
'permissions' => ['upload']
```

## Inheritance Behavior

Inheritance determines if permissions cascade from parent to child folders.

### Inherit Enabled

```php
'/' => [
    'rules' => [
        ['users' => ['*'], 'permissions' => ['read']]
    ]
],
'/projects' => [
    'inherit' => true,  // Inherits 'read' from root
    'rules' => [
        ['users' => ['@developers'], 'permissions' => ['write']]
    ]
]
// Developers have: read (inherited) + write = read, write
```

### Inherit Disabled

```php
'/' => [
    'rules' => [
        ['users' => ['*'], 'permissions' => ['read', 'write', 'delete']]
    ]
],
'/restricted' => [
    'inherit' => false,  // Don't inherit anything from root
    'rules' => [
        ['users' => ['admin'], 'permissions' => ['read']]
    ]
]
// Admin has: read (only from local rule)
// Others have: no access (inheritance cut off)
```

### Override Inherited

```php
// Parent rule
'/' => [
    'rules' => [
        ['users' => ['*'], 'permissions' => ['read', 'write', 'delete']]
    ]
],
// Child rule
'/public' => [
    'inherit' => true,
    'rules' => [
        [
            'users' => ['*'],
            'permissions' => ['read'],
            'override_inherited' => true  // Replace, don't merge
        ]
    ]
]
// Result: Users have only 'read' in /public (write, delete removed)
```

## Priority Resolution

When multiple rules match, priority determines processing order:

```php
'path_rules' => [
    '/files' => [
        'rules' => [
            // Admins: highest priority
            [
                'users' => ['@admins'],
                'permissions' => ['read', 'write', 'delete'],
                'priority' => 100,
            ],
            // VPN users: medium priority
            [
                'users' => ['*'],
                'ip_allowlist' => ['10.8.0.0/24'],
                'permissions' => ['read', 'download'],
                'priority' => 50,
            ],
            // Default: lowest priority
            [
                'users' => ['*'],
                'permissions' => ['read'],
                'priority' => 0,
            ],
        ],
    ],
]
```

**Sorting Order**:
1. Path specificity (deeper paths first)
2. Priority value (higher first)
3. Rule order in array (earlier first)

## Trusted Proxies

When FileGator is behind a reverse proxy (nginx, CloudFlare, etc.), configure trusted proxies:

```php
'settings' => [
    'trusted_proxies' => [
        '127.0.0.1',           // localhost
        '10.0.0.1',            // internal nginx
        '172.17.0.0/16',       // Docker network
        // CloudFlare IPs (example)
        '173.245.48.0/20',
        '103.21.244.0/22',
    ],
]
```

Without trusted proxies, X-Forwarded-For headers are ignored (preventing spoofing).

## Complete Configuration Example

```php
<?php
return [
    'enabled' => true,

    'settings' => [
        'evaluation_mode' => 'most_specific_wins',
        'default_inherit' => true,
        'deny_overrides_allow' => true,
        'cache_enabled' => true,
        'cache_ttl' => 300,
        'trusted_proxies' => ['127.0.0.1', '10.0.0.1'],
        'fail_mode' => 'deny',
    ],

    'groups' => [
        'developers' => ['john', 'jane', 'bob'],
        'contractors' => ['alice', 'charlie'],
        'admins' => ['admin'],
    ],

    'path_rules' => [
        // Root: Default read for all
        '/' => [
            'inherit' => false,
            'rules' => [
                [
                    'users' => ['*'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read'],
                    'priority' => 0,
                ],
                // Admins: full access
                [
                    'users' => ['@admins'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete', 'zip', 'chmod'],
                    'priority' => 100,
                ],
            ],
        ],

        // Public: everyone can download
        '/public' => [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['*'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'download'],
                    'priority' => 50,
                    'override_inherited' => false,
                ],
            ],
        ],

        // Projects: developers only from office/VPN
        '/projects' => [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['@developers'],
                    'ip_allowlist' => ['192.168.1.0/24', '10.8.0.0/24'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 60,
                    'override_inherited' => true,
                ],
            ],
        ],

        // Restricted: no inheritance, strict rules
        '/restricted' => [
            'inherit' => false,
            'rules' => [
                [
                    'users' => ['@admins'],
                    'ip_allowlist' => ['192.168.1.0/24'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 100,
                ],
            ],
        ],
    ],
];
```

## Default Values

Properties with default values:

```php
// Path rules
'inherit' => true  // Inherits from parent by default

// Rule properties
'ip_denylist' => []  // No IPs denied by default
'priority' => 0  // Lowest priority by default
'override_inherited' => false  // Merge by default
```

## Configuration Validation

The ACL service validates configuration on load:

- All required properties must be present
- IP addresses must be valid format
- Permission names must be recognized
- Priority values must be integers
- Groups must exist when referenced

Invalid configurations trigger errors based on `fail_mode` setting.

## Caching Behavior

When `cache_enabled => true`:

```php
'cache_ttl' => 300  // Cache results for 5 minutes
```

**Cache Key**: `acl:{username}:{ip}:{path}:{permission}`

**Clear Cache**: Call `$pathAcl->clearCache()` after configuration changes.

## Environment Variables

For security, store sensitive data in environment variables:

```php
// In acl_config.php
'trusted_proxies' => [
    getenv('TRUSTED_PROXY_1') ?: '127.0.0.1',
    getenv('TRUSTED_PROXY_2') ?: '10.0.0.1',
]
```

Set in your server configuration:

### Apache (.htaccess)
```apache
SetEnv TRUSTED_PROXY_1 "10.0.0.1"
```

### Nginx (fastcgi_params)
```nginx
fastcgi_param TRUSTED_PROXY_1 "10.0.0.1";
```

### PHP-FPM (pool.d/www.conf)
```ini
env[TRUSTED_PROXY_1] = "10.0.0.1"
```

## Migration from Standard Permissions

To migrate existing FileGator installations:

1. **Keep existing configuration** - Don't modify `users.json`
2. **Enable service with disabled ACL**:
```php
'config' => [
    'acl_config_file' => __DIR__.'/private/acl_config.php',
    'enabled' => false,  // Start disabled
]
```
3. **Create initial ACL config** replicating current permissions
4. **Test with enabled ACL** - verify all users can access files
5. **Add IP restrictions gradually** - test after each change
6. **Deploy per-folder rules** as needed

See [Examples](./examples.md) for complete migration scenarios.
