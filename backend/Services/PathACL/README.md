# PathACL Service

Three-dimensional access control for FileGator combining **User Identity + Source IP Address + Folder Path** to determine effective permissions.

## Overview

The PathACL service provides enterprise-grade access control capabilities by evaluating permissions based on three dimensions:

1. **User Identity** - Username, group membership
2. **Source IP Address** - CIDR ranges, inclusions, exclusions
3. **Folder Path** - Path-specific rules with inheritance

## Features

- **Cascading Inheritance** - Permissions flow from parent to child folders
- **Explicit Override** - Specific rules can override inherited permissions
- **Group-based Permissions** - Organize users into groups for easier management
- **IP Restrictions** - Support for CIDR notation, IPv4/IPv6, wildcards
- **Priority-based Evaluation** - Rules are evaluated by specificity and priority
- **Performance Caching** - In-memory cache for evaluated permissions
- **Security-first** - Deny overrides allow, fail-secure by default
- **Backward Compatible** - Existing users continue to work without changes

## Files

### Core Implementation

- **PathACLInterface.php** - Service interface definition
- **PathACL.php** - Main service implementation
- **IpMatcher.php** - IP address matching utility (wraps Symfony IpUtils)
- **PathMatcher.php** - Path normalization and traversal utility

### Configuration

- **acl_config.example.php** - Example configuration file

## Quick Start

### 1. Enable the Service

Add to your `/configuration.php`:

```php
'services' => [
    // ... existing services ...

    'Filegator\Services\PathACL\PathACLInterface' => [
        'handler' => '\Filegator\Services\PathACL\PathACL',
        'config' => [
            'enabled' => true,  // REQUIRED: Must be true to enable PathACL
            'acl_config_file' => __DIR__.'/private/acl_config.php',
        ],
    ],
]
```

**Important:** The `'enabled' => true` setting in `configuration.php` is **required** to activate the PathACL system. Without it, the service will be registered but will not filter any paths.

### 2. Create Configuration File

Copy the example configuration:

```bash
cp backend/Services/PathACL/acl_config.example.php private/acl_config.php
```

Edit `/private/acl_config.php` to define your rules.

### 3. Integrate with Controllers

The PathACL service will be automatically injected into FileController when you modify it to accept the PathACLInterface dependency.

## Configuration Format

### Basic Structure

```php
return [
    'enabled' => true,

    'settings' => [
        'cache_enabled' => true,
        'cache_ttl' => 300,
        'fail_mode' => 'deny',
    ],

    'groups' => [
        'developers' => ['john', 'jane'],
        'admins' => ['admin'],
    ],

    'path_rules' => [
        '/' => [
            'inherit' => false,
            'rules' => [
                // Rules array
            ],
        ],
    ],
];
```

### Rule Structure

Each rule in `path_rules` consists of:

```php
[
    'users' => ['*'],                      // '*', usernames, or '@groupname'
    'ip_inclusions' => ['192.168.1.0/24'], // CIDR, IPs, or '*' - IPs to include
    'ip_exclusions' => [],                  // IPs to exclude (takes precedence)
    'permissions' => ['read', 'write'],    // Permission array
    'priority' => 50,                      // Higher priority first
    'override_inherited' => false,         // Replace vs merge permissions
]
```

## Usage Examples

### Example 1: Office Network Only

```php
'path_rules' => [
    '/' => [
        'rules' => [
            [
                'users' => ['*'],
                'ip_inclusions' => ['203.0.113.0/24'],  // Office network
                'ip_exclusions' => [],
                'permissions' => ['read', 'write', 'upload', 'download'],
                'priority' => 50,
            ],
        ],
    ],
]
```

### Example 2: VPN Read-Only, Office Full Access

```php
'path_rules' => [
    '/' => [
        'rules' => [
            // VPN users: read-only
            [
                'users' => ['*'],
                'ip_inclusions' => ['10.8.0.0/24'],
                'permissions' => ['read', 'download'],
                'priority' => 40,
            ],
            // Office users: full access
            [
                'users' => ['*'],
                'ip_inclusions' => ['192.168.1.0/24'],
                'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                'priority' => 50,
            ],
        ],
    ],
]
```

### Example 3: Department Folders

```php
'groups' => [
    'hr-staff' => ['susan', 'tom'],
    'developers' => ['john', 'jane'],
],

'path_rules' => [
    '/hr' => [
        'rules' => [
            [
                'users' => ['@hr-staff'],
                'ip_inclusions' => ['192.168.1.0/24'],
                'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                'priority' => 60,
            ],
        ],
    ],
    '/engineering' => [
        'rules' => [
            [
                'users' => ['@developers'],
                'ip_inclusions' => ['192.168.2.0/24'],
                'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                'priority' => 60,
            ],
        ],
    ],
]
```

## Evaluation Algorithm

The PathACL service evaluates permissions using this algorithm:

1. **User-Level IP Check** - Verify user's IP is allowed (if user-level restrictions exist)
2. **Find Matching Rules** - Traverse from specific path to root, collect matching rules
3. **Sort Rules** - By path specificity (depth), then priority, then order
4. **Merge Permissions** - Combine permissions (or override if `override_inherited` is true)
5. **Check Permission** - Determine if requested permission is in effective set

### Evaluation Order

- **Path Specificity**: `/projects/alpha` is more specific than `/projects`
- **Priority**: Higher priority rules evaluated first
- **Rule Order**: Earlier rules in array evaluated first (tie-breaker)

### Inheritance

- By default, child paths inherit permissions from parents
- Set `'inherit' => false` to stop inheritance at a path
- Use `'override_inherited' => true` to replace (not merge) parent permissions

## IP Matching

The IpMatcher utility supports:

- **Single IPs**: `192.168.1.50`
- **CIDR Notation**: `192.168.1.0/24` (subnet)
- **IPv6**: `2001:db8::/32`
- **Wildcard**: `*` (all IPs)

### IP Evaluation Logic

```
1. Check exclusions first - if match, DENY (exclusions always win)
2. Check inclusions (if non-empty) - if no match, DENY
3. If inclusions is empty, ALLOW (exclusions-only mode)
```

### Examples

```php
// Allow entire subnet except one IP
'ip_inclusions' => ['192.168.1.0/24'],
'ip_exclusions' => ['192.168.1.50'],

// Allow only specific IPs
'ip_inclusions' => ['192.168.1.10', '192.168.1.20'],
'ip_exclusions' => [],

// Block specific IPs (allow all others)
'ip_inclusions' => [],  // Empty = allow all
'ip_exclusions' => ['192.0.2.50', '198.51.100.0/24'],
```

## Path Matching

The PathMatcher utility provides:

- **Path Normalization** - Converts paths to canonical form, prevents traversal
- **Depth Calculation** - Determines path specificity
- **Parent Traversal** - Gets all ancestor paths for inheritance
- **Security** - Blocks directory traversal attempts (`..`, absolute paths)

## Security Considerations

### IP Spoofing Protection

- Configure trusted proxies in settings
- Only trust X-Forwarded-For from known reverse proxies
- Direct connections use socket IP (cannot be spoofed)

### Default Behaviors (Fail-Secure)

| Scenario | Behavior |
|----------|----------|
| No ACL rules match | DENY |
| Empty permission set | DENY |
| Invalid IP format | Treat as non-match |
| ACL config load fails | Controlled by `fail_mode` |
| Cache lookup fails | Evaluate fresh |
| Path normalization fails | DENY |

### Fail Modes

- **deny** (default) - Deny access on errors (most secure)
- **allow** - Allow access on errors (most permissive)
- **fallback** - Use global user permissions on errors (balanced)

## Performance

### Caching

The service implements in-memory permission caching:

- **Cache Key**: `md5(username:ip:path:permission)`
- **TTL**: Configurable (default 300 seconds)
- **Cache Invalidation**: Call `clearCache()` after config changes

### Expected Performance

| Scenario | Expected Time |
|----------|---------------|
| Cache hit | < 1ms |
| Cache miss, simple ACL | 1-5ms |
| Cache miss, complex ACL | 5-15ms |

### Optimization Tips

1. Enable caching (enabled by default)
2. Use higher priority for frequently-matched rules
3. Use `override_inherited => true` to stop rule traversal early
4. Keep group memberships in reasonable size

## Debugging

Use the `explainPermission()` method to troubleshoot:

```php
$explanation = $pathAcl->explainPermission($user, $clientIp, $path, $permission);
print_r($explanation);
```

Returns:

```php
[
    'allowed' => true/false,
    'reason' => 'Explanation string',
    'matched_rules' => [...],
    'effective_permissions' => ['read', 'write', ...],
    'requested_permission' => 'write',
    'user_ip_check' => true/false,
    'evaluation_path' => ['/path', '/parent', '/'],
]
```

## Integration

### With FileController

Modify FileController to inject PathACLInterface and check permissions:

```php
class FileController
{
    private $pathAcl;

    public function __construct(
        Config $config,
        Session $session,
        AuthInterface $auth,
        Filesystem $storage,
        PathACLInterface $pathAcl
    ) {
        $this->pathAcl = $pathAcl;
        // ... existing code ...
    }

    private function checkPathPermission(string $path, string $permission): bool
    {
        if (!$this->pathAcl->isEnabled()) {
            return $this->auth->user()->hasPermissions($permission);
        }

        $clientIp = $this->request->getClientIp();
        return $this->pathAcl->checkPermission(
            $this->auth->user(),
            $clientIp,
            $path,
            $permission
        );
    }
}
```

### Backward Compatibility

When `enabled => false`, the PathACL service:
- Returns `true` from `checkPermission()` (no restrictions)
- Allows FileGator to use existing global permission system
- Zero impact on existing installations

## Dependencies

- **Symfony HttpFoundation** - For IpUtils (already a FileGator dependency)
- **PHP 7.4+** - Required for type hints and property types
- **FileGator User class** - For user identity

## Common Patterns

### Pattern 1: Public Read, Authenticated Write

```php
'/' => [
    'rules' => [
        [
            'users' => ['*'],
            'ip_inclusions' => ['*'],
            'permissions' => ['read', 'download'],
            'priority' => 10,
        ],
        [
            'users' => ['*'],
            'ip_inclusions' => ['192.168.1.0/24'],  // Office only
            'permissions' => ['write', 'upload', 'delete'],
            'priority' => 20,
        ],
    ],
]
```

### Pattern 2: Contractor Temporary Access

```php
'groups' => [
    'contractors' => ['alice', 'bob'],
],

'path_rules' => [
    '/projects/client-project' => [
        'rules' => [
            [
                'users' => ['@contractors'],
                'ip_inclusions' => ['10.8.0.0/24'],  // VPN only
                'permissions' => ['read', 'download'],
                'priority' => 50,
            ],
        ],
    ],
]
```

### Pattern 3: Admin Emergency Access

```php
'/' => [
    'rules' => [
        [
            'users' => ['admin'],
            'ip_inclusions' => ['*'],  // From anywhere
            'permissions' => ['read', 'write', 'upload', 'download', 'delete', 'chmod'],
            'priority' => 100,  // Highest priority
        ],
    ],
]
```

## Testing

To test your ACL configuration:

1. **Start Simple** - Begin with basic rules and gradually add complexity
2. **Use explainPermission()** - Debug why access is granted/denied
3. **Test Edge Cases** - Verify inheritance, overrides, IP matching
4. **Monitor Logs** - Check error logs for configuration issues
5. **Disable if Needed** - Set `enabled => false` to quickly rollback

## Troubleshooting

### Issue: User can't access files

**Check:**
1. Is PathACL enabled? (`$pathAcl->isEnabled()`)
2. Does user match any rules? (use `explainPermission()`)
3. Is user's IP in inclusions list?
4. Are there conflicting rules with higher priority?

### Issue: Everyone has access to restricted folder

**Check:**
1. Is there a wildcard rule (`'users' => ['*']`) with high priority?
2. Is `override_inherited` set correctly?
3. Are exclusions configured properly?

### Issue: Performance degradation

**Check:**
1. Is caching enabled? (`'cache_enabled' => true`)
2. Are there too many rules? (consider consolidation)
3. Is cache TTL too low?

## License

This file is part of the FileGator package.

(c) Milos Stojanovic <alcalbg@gmail.com>

For the full copyright and license information, please view the LICENSE file.
