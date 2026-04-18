# Path-Based Access Control (ACL) System

FileGator's Path-Based ACL system provides enterprise-grade access control by combining three dimensions: User Identity, Source IP Address, and Folder Path. This enables fine-grained permissions that can vary based on where users are connecting from and which folders they're accessing.

## Table of Contents

- [Overview](#overview)
- [Key Features](#key-features)
- [Quick Start](#quick-start)
- [How It Works](#how-it-works)
- [When to Use Path ACL](#when-to-use-path-acl)
- [Documentation](#documentation)

## Overview

Traditional file management systems typically use a simple two-dimensional model: User + Permissions. FileGator's Path ACL system extends this with three dimensions:

**User Identity + Source IP + Folder Path = Effective Permissions**

This means the same user can have:
- Full access to `/projects` from the office network
- Read-only access to `/projects` from VPN
- No access to `/restricted` from any IP
- Upload-only access to `/uploads` from external IPs

## Key Features

### Three-Dimensional Access Control
Permissions are determined by the combination of who you are, where you're connecting from, and what you're accessing.

### IP-Based Restrictions
Support for:
- Single IP addresses: `192.168.1.50`
- CIDR notation: `192.168.1.0/24`
- IPv6 addresses and ranges: `2001:db8::/32`
- IP exclusions using deny lists
- Wildcards for public access

### Path Inheritance
Permissions cascade from parent folders to child folders, with options to override at any level.

### Group-Based Access
Define groups of users and apply permissions to entire groups:
```php
'groups' => [
    'developers' => ['john', 'jane', 'bob'],
    'contractors' => ['alice', 'charlie'],
]
```

### Priority and Override Control
Fine-tune permission resolution with priority levels and explicit override flags.

### Flexible Configuration
Configure using intuitive PHP arrays with support for:
- Multiple rules per path
- Rule priorities
- Inheritance control
- Allow/deny lists

### Performance Optimized
Built-in caching ensures minimal overhead (< 1ms on cache hits).

### Backward Compatible
Existing users continue working without changes. The ACL system can be enabled incrementally.

## Quick Start

### 1. Enable Path ACL Service

Add to your `configuration.php`:

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

### 2. Create ACL Configuration

Create `/private/acl_config.php`:

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
    ],

    'groups' => [
        'developers' => ['john', 'jane'],
        'admins' => ['admin'],
    ],

    'path_rules' => [
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
            ],
        ],

        '/projects' => [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['@developers'],
                    'ip_allowlist' => ['192.168.1.0/24'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 50,
                    'override_inherited' => true,
                ],
            ],
        ],
    ],
];
```

### 3. Test Configuration

Use the explain method to verify permissions:

```php
$result = $pathAcl->explainPermission($user, $clientIp, '/projects/file.txt', 'write');
print_r($result);
```

## How It Works

### Three-Dimensional Evaluation

When a user requests access to a file or folder, FileGator evaluates permissions in this order:

```
1. User-Level IP Check
   └─> Check user's IP allowlist/denylist from user profile
       └─> DENY if IP is blocked at user level

2. Find Matching Path Rules
   └─> Start at requested path, traverse to root
   └─> Collect rules where user AND IP match
   └─> Stop if inheritance is disabled

3. Sort Rules by Specificity
   └─> More specific paths first
   └─> Higher priority values first
   └─> Earlier rules first

4. Merge Permissions
   └─> Apply rules in sorted order
   └─> Override or merge based on settings

5. Check Requested Permission
   └─> ALLOW if permission is granted
   └─> DENY otherwise
```

### Permission Inheritance Example

```
/ (root)
├─ Default: Everyone can read
│
├─ /projects
│  ├─ Inherit from root
│  └─ Add: Developers can write from office IP
│
└─ /projects/secret
   ├─ Don't inherit (override)
   └─ Only john can access from specific IP
```

### IP Matching

IP restrictions use industry-standard formats:

```php
'ip_allowlist' => [
    '192.168.1.0/24',     // Office subnet
    '10.8.0.0/24',        // VPN subnet
    '203.0.113.50',       // Specific admin IP
],
'ip_denylist' => [
    '192.168.1.99',       // Blocked device
]
```

**Evaluation Order**: Deny always wins. If an IP is in both lists, access is denied.

## When to Use Path ACL

### Use Path ACL When You Need:

- **Location-based restrictions**: Different permissions for office vs remote access
- **Per-folder security**: Sensitive folders require stricter access control
- **Contractor/temporary access**: Limited permissions from specific networks only
- **Compliance requirements**: Audit trails showing who accessed what from where
- **Department isolation**: Each team has access only to their folders
- **Upload-only folders**: Public uploads without download permission
- **Geographic restrictions**: Restrict access by IP region

### Use Standard Permissions When:

- All users have the same permissions everywhere
- No need for IP-based restrictions
- Simple setup with minimal folders
- Single-office environment with trusted network

### Comparison: Standard vs Path ACL

| Feature | Standard Permissions | Path ACL |
|---------|---------------------|----------|
| **User-based access** | Yes | Yes |
| **IP restrictions** | Global only | Per-path |
| **Folder-specific permissions** | No | Yes |
| **Location-based access** | No | Yes |
| **Group management** | No | Yes |
| **Inheritance** | N/A | Yes |
| **Configuration complexity** | Low | Medium |
| **Performance overhead** | None | Minimal (< 1ms) |

## Documentation

### Configuration Reference
Complete guide to all configuration options, settings, and syntax.

[View Configuration Documentation](./configuration.md)

### Practical Examples
Real-world scenarios with complete configuration examples.

[View Examples](./examples.md)

### Troubleshooting
Common issues, debugging techniques, and performance optimization.

[View Troubleshooting Guide](./troubleshooting.md)

## Security Considerations

### IP Spoofing Protection

Configure trusted proxies to prevent X-Forwarded-For header spoofing:

```php
'settings' => [
    'trusted_proxies' => ['127.0.0.1', '10.0.0.1'],
]
```

### Fail-Safe Defaults

The system defaults to denying access when:
- No rules match the request
- Configuration cannot be loaded
- Path normalization fails
- Invalid IP formats are detected

### Path Traversal Prevention

All paths are normalized to prevent directory traversal attacks:
- `..` components are resolved
- Duplicate slashes are removed
- Symlinks are handled securely

## Best Practices

1. **Start Simple**: Begin with a basic root rule, then add per-folder restrictions as needed
2. **Use Groups**: Define groups for easier management of team permissions
3. **Test First**: Use `explainPermission()` to verify rules before deploying
4. **Enable Caching**: Keep cache enabled for production (5-minute TTL recommended)
5. **Monitor Access**: Use audit logging to track denied access attempts
6. **Document Rules**: Add comments to your ACL config explaining each rule
7. **Regular Review**: Periodically review and clean up unused rules

## Migration from Standard Permissions

Existing FileGator installations can adopt Path ACL gradually:

1. Enable PathACL service with `enabled => false`
2. Create ACL config replicating current permissions
3. Test thoroughly with `enabled => true`
4. Add IP restrictions incrementally
5. Deploy per-folder rules as needed

See the [Configuration Documentation](./configuration.md) for detailed migration steps.

## Support

For questions, issues, or feature requests:
- Documentation: https://docs.filegator.io/
- GitHub Issues: https://github.com/filegator/filegator/issues
- Community Forum: https://github.com/filegator/filegator/discussions
