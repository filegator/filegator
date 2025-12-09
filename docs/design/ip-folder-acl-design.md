# FileGator IP-Based Folder Permission System - Design Document

**Version:** 1.0
**Date:** 2025-12-09
**Status:** Design Complete - Ready for Implementation
**Target:** FileGator 7.13.0+

---

## Executive Summary

This document defines a three-dimensional access control system for FileGator that combines **User Identity + Source IP Address + Folder Path** to determine effective permissions. The design prioritizes simplicity and practicality while providing enterprise-grade access control capabilities.

### Key Features

1. **Three-Dimensional Access Control**: User + IP + Path → Permissions
2. **Flexible IP Specification**: Single IPs, CIDR blocks, IP ranges, and negation
3. **Cascading Inheritance**: Permissions flow from parent to child folders
4. **Explicit Override**: Specific rules override inherited permissions
5. **PHP Array Configuration**: Intuitive configuration familiar to administrators
6. **Backward Compatible**: Existing users continue to work without changes

### Design Principles

- **Security First**: Deny overrides allow, explicit over implicit
- **Simplicity**: Easy to understand and configure
- **Performance**: Efficient evaluation with caching
- **Extensibility**: Foundation for future enhancements

---

## 1. IP Specification Syntax

### 1.1 Supported Formats

The system supports industry-standard IP address formats using CIDR notation and Symfony's IpUtils library:

| Format | Example | Description |
|--------|---------|-------------|
| **Single IPv4** | `192.168.1.50` | Exact IP match |
| **CIDR IPv4** | `192.168.1.0/24` | Subnet (192.168.1.0-255) |
| **CIDR IPv4 Large** | `10.0.0.0/8` | Large network (10.0.0.0-10.255.255.255) |
| **Single IPv6** | `2001:db8::1` | Exact IPv6 match |
| **CIDR IPv6** | `2001:db8::/32` | IPv6 subnet |
| **Wildcard** | `*` | All IP addresses |

### 1.2 Negation via Deny Lists

Negation (exclusion) is handled through explicit deny lists rather than negation operators:

```php
// Allow entire subnet except one IP
'ip_allowlist' => ['192.168.1.0/24'],
'ip_denylist' => ['192.168.1.50'],  // This IP is blocked
```

### 1.3 Evaluation Order

**Recommendation: Additive Allow + Subtractive Deny**

```
Step 1: Check IP against denylist
  → If match found → DENY (immediate rejection)

Step 2: Check IP against allowlist (if allowlist is non-empty)
  → If match found → Continue to permission evaluation
  → If no match → DENY (implicit default-deny in allowlist mode)

Step 3: If allowlist is empty → Continue (implicit default-allow in denylist mode)
```

**Rationale**:
- Deny always wins (security-first)
- Empty allowlist = allow all except denylist (public mode)
- Non-empty allowlist = only these IPs allowed (restricted mode)
- Simple and predictable

### 1.4 IP Specification Examples

```php
// Example 1: Office network only
'ip_allowlist' => ['203.0.113.0/24', '198.51.100.10'],
'ip_denylist' => [],

// Example 2: Public access with blocked IPs
'ip_allowlist' => [],  // Empty = allow all
'ip_denylist' => ['192.0.2.50', '198.51.100.0/24'],

// Example 3: VPN users except one problematic IP
'ip_allowlist' => ['10.8.0.0/24'],
'ip_denylist' => ['10.8.0.99'],

// Example 4: IPv6 support
'ip_allowlist' => ['2001:db8::/32', '2001:db8:1234:5678::1'],
'ip_denylist' => [],

// Example 5: Localhost only
'ip_allowlist' => ['127.0.0.1', '::1'],
'ip_denylist' => [],
```

---

## 2. Configuration Schema

### 2.1 File Structure

The configuration uses PHP arrays stored in `/private/acl_config.php` (or embedded in `configuration.php`):

```
/private/
  ├── users.json              # User credentials and basic info
  ├── acl_config.php          # Path-based ACL rules (NEW)
  └── groups.json             # Group definitions (NEW, optional)
```

### 2.2 Complete Configuration Format

```php
<?php
// /private/acl_config.php

return [
    // Enable/disable path-based ACL system
    'enabled' => true,

    // Global ACL settings
    'settings' => [
        'evaluation_mode' => 'most_specific_wins',  // Algorithm choice
        'default_inherit' => true,                   // Default inheritance behavior
        'deny_overrides_allow' => true,             // Deny always wins
        'cache_enabled' => true,                     // Enable permission caching
        'cache_ttl' => 300,                          // Cache lifetime (seconds)
        'trusted_proxies' => ['127.0.0.1'],         // For X-Forwarded-For validation
        'fail_mode' => 'deny',                       // 'deny' or 'allow' on error
    ],

    // Group definitions (users can belong to groups)
    'groups' => [
        'developers' => ['john', 'jane', 'bob'],
        'contractors' => ['alice', 'charlie'],
        'hr-staff' => ['susan', 'tom'],
        'admins' => ['admin', 'root'],
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
                    'ip_allowlist' => ['*'],  // * = all IPs
                    'ip_denylist' => [],
                    'permissions' => ['read'],
                    'priority' => 0,  // Lowest priority
                ],
                // Rule 2: Admins have full access from anywhere
                [
                    'users' => ['@admins'],  // @ prefix for groups
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
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
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
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
                    'ip_allowlist' => ['192.168.1.0/24', '10.8.0.0/24'],  // Office + VPN
                    'ip_denylist' => [],
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
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 75,
                    'override_inherited' => true,
                ],
                // Contractors get read-only from VPN only
                [
                    'users' => ['@contractors'],
                    'ip_allowlist' => ['10.8.0.0/24'],
                    'ip_denylist' => [],
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
                    'ip_allowlist' => ['192.168.1.0/24'],  // Office network only
                    'ip_denylist' => [],
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
                    'ip_allowlist' => ['192.168.0.0/16', '10.0.0.0/8'],
                    'ip_denylist' => [],
                    'permissions' => ['upload'],  // Can upload but not read
                    'priority' => 50,
                    'override_inherited' => false,
                ],
            ],
        ],
    ],
];
```

### 2.3 User Configuration Format

Extend existing `/private/users.json` with IP restrictions (optional):

```json
{
  "1": {
    "username": "admin",
    "name": "Administrator",
    "role": "admin",
    "homedir": "/",
    "permissions": "read|write|upload|download|batchdownload|zip|chmod",
    "password": "$2y$10$...",
    "ip_allowlist": ["*"],
    "ip_denylist": []
  },
  "2": {
    "username": "john",
    "name": "John Doe",
    "role": "user",
    "homedir": "/",
    "permissions": "read|upload|download",
    "password": "$2y$10$...",
    "ip_allowlist": ["192.168.1.0/24", "10.8.0.0/24"],
    "ip_denylist": ["192.168.1.99"]
  }
}
```

**Note**: User-level IP restrictions are checked BEFORE path-based ACL evaluation. If a user's IP is denied at the user level, they cannot access any paths.

### 2.4 Configuration Location

**Option 1: Separate ACL Config File** (Recommended)
```php
// configuration.php
'services' => [
    'Filegator\Services\PathACL\PathACLInterface' => [
        'handler' => '\Filegator\Services\PathACL\PathACL',
        'config' => [
            'acl_config_file' => __DIR__.'/private/acl_config.php',
        ],
    ],
]
```

**Option 2: Embedded in configuration.php**
```php
// configuration.php
'services' => [
    'Filegator\Services\PathACL\PathACLInterface' => [
        'handler' => '\Filegator\Services\PathACL\PathACL',
        'config' => require __DIR__.'/private/acl_config.php',
    ],
]
```

**Option 3: JSON Format** (for consistency with users.json)
```php
// Load from JSON
'acl_config_file' => __DIR__.'/private/acl_config.json',
```

**Recommendation**: Use Option 1 (separate PHP file) for better maintainability and PHP array benefits (comments, multi-line).

---

## 3. Permission Evaluation Algorithm

### 3.1 High-Level Flow

```
User requests access to: /projects/alpha/file.txt with permission 'write'

Step 1: User-Level IP Check
  → Get user's IP allowlist/denylist from user profile
  → If IP in user's denylist → DENY (403 Forbidden)
  → If user has allowlist AND IP not in allowlist → DENY
  → Otherwise continue

Step 2: Find Matching Path Rules
  → Start at requested path: /projects/alpha
  → Traverse to root: /projects → /
  → Collect all rules where:
      - User matches (username or group membership)
      - IP matches (in allowlist AND not in denylist)
  → Stop traversal if 'inherit' = false

Step 3: Sort Rules by Specificity and Priority
  → Primary sort: Path specificity (deeper paths first)
  → Secondary sort: Priority value (higher first)
  → Tertiary sort: Rule order (earlier first)

Step 4: Merge Permissions
  → Start with empty permission set
  → For each rule in sorted order:
      - If 'override_inherited' = true → Replace permission set
      - If 'override_inherited' = false → Merge (union) permissions
  → Result: Effective permissions for this user+IP+path

Step 5: Check Requested Permission
  → If requested permission ('write') in effective permissions → ALLOW
  → Otherwise → DENY (403 Forbidden)
```

### 3.2 Detailed Algorithm Pseudocode

```python
def check_permission(username, client_ip, requested_path, requested_permission):
    """
    Determines if user can perform requested_permission on requested_path from client_ip.

    Returns: (allowed: bool, reason: string)
    """

    # Step 1: User-level IP check
    user = get_user(username)
    if not check_user_ip_access(user, client_ip):
        return (False, "User IP access denied")

    # Step 2: Find all matching path rules
    matching_rules = []
    current_path = normalize_path(requested_path)

    while current_path is not None:
        if path_rules_exist(current_path):
            path_acl = get_path_acl(current_path)

            for rule in path_acl['rules']:
                # Check if user matches
                if not user_matches_rule(username, rule['users']):
                    continue

                # Check if IP matches
                if not ip_matches_rule(client_ip, rule['ip_allowlist'], rule['ip_denylist']):
                    continue

                # Rule matches - add with metadata
                matching_rules.append({
                    'rule': rule,
                    'path': current_path,
                    'specificity': path_depth(current_path),
                })

            # Stop if inheritance disabled
            if not path_acl.get('inherit', True):
                break

        # Move to parent path
        current_path = parent_path(current_path)

    # If no rules matched, check global default
    if not matching_rules:
        return (False, "No matching ACL rules found")

    # Step 3: Sort rules
    matching_rules = sorted(
        matching_rules,
        key=lambda r: (
            r['specificity'],          # More specific paths first
            r['rule']['priority'],     # Higher priority first
            -r['rule'].get('order', 0) # Earlier rules first (negative for reverse)
        ),
        reverse=True
    )

    # Step 4: Merge permissions
    effective_permissions = set()

    for match in matching_rules:
        rule = match['rule']
        rule_permissions = set(rule['permissions'])

        if rule.get('override_inherited', False):
            # Replace all permissions
            effective_permissions = rule_permissions
            break  # Stop processing, this rule overrides everything
        else:
            # Merge permissions (union)
            effective_permissions.update(rule_permissions)

    # Step 5: Check requested permission
    if requested_permission in effective_permissions:
        return (True, f"Allowed by ACL rules (effective permissions: {effective_permissions})")
    else:
        return (False, f"Permission '{requested_permission}' not granted (have: {effective_permissions})")


def check_user_ip_access(user, client_ip):
    """Check user-level IP allowlist/denylist"""
    ip_denylist = user.get('ip_denylist', [])
    ip_allowlist = user.get('ip_allowlist', [])

    # Check denylist first
    if ip_denylist and IpUtils.checkIp(client_ip, ip_denylist):
        return False

    # Check allowlist (if defined)
    if ip_allowlist and ip_allowlist != ['*']:
        if not IpUtils.checkIp(client_ip, ip_allowlist):
            return False

    return True


def user_matches_rule(username, rule_users):
    """Check if username matches rule's user specification"""
    if '*' in rule_users:
        return True  # Wildcard matches all

    if username in rule_users:
        return True  # Direct username match

    # Check group membership
    for rule_user in rule_users:
        if rule_user.startswith('@'):
            group_name = rule_user[1:]  # Remove @ prefix
            if username_in_group(username, group_name):
                return True

    return False


def ip_matches_rule(client_ip, allowlist, denylist):
    """Check if client_ip matches rule's IP restrictions"""
    # Deny takes precedence
    if denylist and IpUtils.checkIp(client_ip, denylist):
        return False

    # Check allowlist
    if allowlist and allowlist != ['*']:
        if not IpUtils.checkIp(client_ip, allowlist):
            return False

    return True


def normalize_path(path):
    """
    Normalize path to canonical form.
    - Remove trailing slashes
    - Remove duplicate slashes
    - Resolve . and .. (security critical)
    """
    # Security: Prevent directory traversal
    path = os.path.realpath(path)
    path = path.replace('\\', '/')  # Normalize slashes
    path = re.sub(r'/+', '/', path)  # Remove duplicates
    if path != '/' and path.endswith('/'):
        path = path[:-1]  # Remove trailing slash
    return path


def path_depth(path):
    """Calculate path depth (number of segments)"""
    if path == '/':
        return 0
    return path.count('/')


def parent_path(path):
    """Get parent path, or None if at root"""
    if path == '/':
        return None
    parent = os.path.dirname(path)
    return parent if parent else '/'
```

### 3.3 Evaluation Examples

#### Example 1: Simple Inheritance

```
Configuration:
  / → [Rule: users=['*'], permissions=['read']]
  /projects → [Rule: users=['@developers'], permissions=['read','write'], override=false]

User: john (member of 'developers')
Path: /projects/alpha/file.txt
Request: 'write'

Evaluation:
  1. Find rules for /projects/alpha → None
  2. Find rules for /projects → Rule matches (john in @developers)
  3. Find rules for / → Rule matches (john is authenticated)
  4. Sort: /projects (specificity=1) > / (specificity=0)
  5. Merge:
     - Start with /projects: {read, write} (override=false, so merge)
     - Merge with /: {read, write} ∪ {read} = {read, write}
  6. Check: 'write' in {read, write} → ALLOW
```

#### Example 2: Explicit Override

```
Configuration:
  / → [Rule: users=['*'], permissions=['read','write','delete']]
  /public → [Rule: users=['*'], permissions=['read'], override=true]

User: john
Path: /public/file.txt
Request: 'delete'

Evaluation:
  1. Find rules for /public → Rule matches
  2. Find rules for / → Rule matches
  3. Sort: /public (specificity=1) > / (specificity=0)
  4. Merge:
     - Start with /public: {read} (override=true, STOP merging)
  5. Check: 'delete' in {read} → DENY
```

#### Example 3: IP Restriction

```
Configuration:
  / → [Rule: users=['*'], ip_allow=['*'], permissions=['read']]
  /admin → [Rule: users=['admin'], ip_allow=['192.168.1.0/24'], permissions=['read','write']]

User: admin
IP: 10.0.0.50
Path: /admin/config.php
Request: 'write'

Evaluation:
  1. Check user IP: admin has no user-level IP restrictions → OK
  2. Find rules for /admin → IP 10.0.0.50 not in 192.168.1.0/24 → Rule DOES NOT MATCH
  3. Find rules for / → Rule matches
  4. Effective permissions: {read} (only from / rule)
  5. Check: 'write' in {read} → DENY
```

#### Example 4: Group Membership

```
Configuration:
  Groups: developers=['john','jane']
  / → [Rule: users=['*'], permissions=['read']]
  /code → [Rule: users=['@developers'], permissions=['read','write']]

User: john
Path: /code/main.py
Request: 'write'

Evaluation:
  1. Find rules for /code → john in @developers → Rule matches
  2. Find rules for / → Rule matches
  3. Merge: {read,write} ∪ {read} = {read,write}
  4. Check: 'write' in {read,write} → ALLOW
```

### 3.4 Edge Cases

| Scenario | Behavior |
|----------|----------|
| **No rules match** | DENY (fail secure) |
| **Empty permission set** | DENY (no permissions granted) |
| **Multiple rules same path** | Sort by priority, higher priority first |
| **IP in both allow and deny** | Deny wins (security first) |
| **User in multiple groups** | All group permissions apply (union) |
| **inherit=false at middle path** | Stop traversal, don't check parent paths |
| **Requested path has no ACL** | Use nearest parent ACL if inherit=true |
| **Invalid IP format** | Log error, treat as non-match |
| **Cache miss** | Evaluate normally, populate cache |

---

## 4. Integration Points

### 4.1 File Modification Summary

| File | Changes Required | Complexity |
|------|------------------|------------|
| `/backend/Services/PathACL/PathACLInterface.php` | **NEW** Interface definition | Low |
| `/backend/Services/PathACL/PathACL.php` | **NEW** Core ACL service | High |
| `/backend/Services/PathACL/IpMatcher.php` | **NEW** IP matching utility | Medium |
| `/backend/Services/PathACL/PathMatcher.php` | **NEW** Path matching utility | Medium |
| `/backend/Services/PathACL/PermissionCache.php` | **NEW** Caching layer | Medium |
| `/backend/Services/Auth/User.php` | **MODIFY** Add IP allowlist/denylist properties | Low |
| `/backend/Controllers/FileController.php` | **MODIFY** Add ACL checks before file operations | Medium |
| `/backend/Services/Storage/Filesystem.php` | **OPTIONAL** Add ACL hooks in critical methods | Low |
| `/configuration.php` | **MODIFY** Register PathACL service | Low |
| `/private/acl_config.php` | **NEW** ACL configuration file | N/A |

### 4.2 New Service: PathACLInterface

```php
<?php
// backend/Services/PathACL/PathACLInterface.php

namespace Filegator\Services\PathACL;

use Filegator\Services\Auth\User;
use Filegator\Services\Service;

interface PathACLInterface extends Service
{
    /**
     * Check if user can perform permission on path from IP address.
     *
     * @param User $user Current user
     * @param string $clientIp Client IP address
     * @param string $path File/folder path (relative to repository root)
     * @param string $permission Permission to check (read, write, upload, delete, etc.)
     * @return bool True if allowed, false otherwise
     */
    public function checkPermission(User $user, string $clientIp, string $path, string $permission): bool;

    /**
     * Get effective permissions for user on path from IP.
     *
     * @param User $user Current user
     * @param string $clientIp Client IP address
     * @param string $path File/folder path
     * @return array Array of granted permissions
     */
    public function getEffectivePermissions(User $user, string $clientIp, string $path): array;

    /**
     * Check if path-based ACL system is enabled.
     *
     * @return bool True if enabled
     */
    public function isEnabled(): bool;

    /**
     * Clear permission cache (call after ACL config changes).
     *
     * @return void
     */
    public function clearCache(): void;

    /**
     * Get detailed information about permission decision (for debugging).
     *
     * @param User $user Current user
     * @param string $clientIp Client IP address
     * @param string $path File/folder path
     * @param string $permission Permission to check
     * @return array Decision details including matched rules and reasoning
     */
    public function explainPermission(User $user, string $clientIp, string $path, string $permission): array;
}
```

### 4.3 Integration with FileController

**Location**: `/backend/Controllers/FileController.php`

**Modification Points**:

```php
class FileController
{
    private $pathAcl; // NEW: PathACL service

    public function __construct(
        Config $config,
        Session $session,
        AuthInterface $auth,
        Filesystem $storage,
        PathACLInterface $pathAcl  // NEW: Inject PathACL service
    ) {
        // ... existing code ...
        $this->pathAcl = $pathAcl;
    }

    public function getDirectory(Request $request, Response $response)
    {
        $path = $request->input('path');

        // NEW: ACL check
        if (!$this->checkPathPermission($path, 'read')) {
            return $response->json('Access Denied', 403);
        }

        // ... existing code ...
    }

    public function createDir(Request $request, Response $response)
    {
        $path = $request->input('path');

        // NEW: ACL check
        if (!$this->checkPathPermission($path, 'write')) {
            return $response->json('Access Denied', 403);
        }

        // ... existing code ...
    }

    public function uploadFile(Request $request, Response $response)
    {
        $path = $request->input('path');

        // NEW: ACL check
        if (!$this->checkPathPermission($path, 'upload')) {
            return $response->json('Access Denied', 403);
        }

        // ... existing code ...
    }

    public function deleteItems(Request $request, Response $response)
    {
        $items = $request->input('items');

        // NEW: Check each item
        foreach ($items as $item) {
            if (!$this->checkPathPermission($item, 'delete')) {
                return $response->json('Access Denied for: '.$item, 403);
            }
        }

        // ... existing code ...
    }

    // NEW: Helper method
    private function checkPathPermission(string $path, string $permission): bool
    {
        // If PathACL disabled, fall back to global user permissions
        if (!$this->pathAcl->isEnabled()) {
            return $this->auth->user()->hasPermissions($permission);
        }

        // Path-based ACL check
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

### 4.4 Integration with Existing Auth Adapters

**All auth adapters continue to work unchanged**. The PathACL service is a separate layer that:

1. Receives authenticated user from AuthInterface
2. Applies additional IP + path restrictions
3. Falls back to global user permissions if disabled

**No changes required to**:
- `/backend/Services/Auth/Adapters/JsonFile.php`
- `/backend/Services/Auth/Adapters/Database.php`
- `/backend/Services/Auth/Adapters/LDAP.php`
- `/backend/Services/Auth/Adapters/WPAuth.php`

### 4.5 Service Registration

**Location**: `/configuration.php`

```php
return [
    'services' => [
        // ... existing services ...

        // NEW: PathACL service
        'Filegator\Services\PathACL\PathACLInterface' => [
            'handler' => '\Filegator\Services\PathACL\PathACL',
            'config' => [
                'acl_config_file' => __DIR__.'/private/acl_config.php',
                'enabled' => true,  // Enable/disable path-based ACL
            ],
        ],
    ],
];
```

---

## 5. Security Considerations

### 5.1 IP Spoofing Protection

**Threat**: Attacker modifies X-Forwarded-For header to bypass IP restrictions.

**Mitigation**:

1. **Trusted Proxy Configuration**
   ```php
   'settings' => [
       'trusted_proxies' => ['127.0.0.1', '10.0.0.1'],  // Only trust these proxies
   ]
   ```

2. **Symfony Request IP Detection**
   - FileGator already uses `$request->getClientIp()` from Symfony HttpFoundation
   - Symfony validates X-Forwarded-For only from trusted proxies
   - Direct connections use socket IP (cannot be spoofed)

3. **Header Validation**
   ```php
   // In PathACL service initialization
   if (!empty($config['trusted_proxies'])) {
       Request::setTrustedProxies(
           $config['trusted_proxies'],
           Request::HEADER_X_FORWARDED_ALL
       );
   }
   ```

**Best Practice**: Only use X-Forwarded-For if behind a trusted reverse proxy (nginx, CloudFlare, etc.).

### 5.2 X-Forwarded-For Handling

**Scenario 1: Direct Connection (No Proxy)**
```
User (1.2.3.4) → FileGator
  getClientIp() returns: 1.2.3.4 (socket IP)
```

**Scenario 2: Trusted Reverse Proxy**
```
User (1.2.3.4) → Nginx (10.0.0.1) → FileGator
  X-Forwarded-For: 1.2.3.4
  Socket IP: 10.0.0.1
  getClientIp() returns: 1.2.3.4 (from X-Forwarded-For, because 10.0.0.1 is trusted)
```

**Scenario 3: Untrusted Proxy (Attack Attempt)**
```
User (1.2.3.4) → FileGator
  User sends: X-Forwarded-For: 192.168.1.1 (spoofed)
  Socket IP: 1.2.3.4
  getClientIp() returns: 1.2.3.4 (ignores X-Forwarded-For, because connection is not from trusted proxy)
```

**Configuration**:
```php
// In configuration.php or acl_config.php
'settings' => [
    // Only trust these proxy IPs
    'trusted_proxies' => [
        '127.0.0.1',      // localhost
        '10.0.0.1',       // internal nginx
        '172.17.0.0/16',  // Docker network
    ],
]
```

### 5.3 Default Behaviors

**Fail-Secure Principles**:

| Scenario | Behavior | Rationale |
|----------|----------|-----------|
| **No ACL rules match** | DENY | Explicit permission required |
| **Empty permission set** | DENY | No permissions = no access |
| **Invalid IP format** | Treat as non-match | Don't allow malformed input |
| **ACL config load fails** | DENY or ALLOW (configurable) | `fail_mode` setting controls |
| **Cache lookup fails** | Evaluate fresh | Performance vs security trade-off |
| **User not authenticated** | DENY | Must be logged in |
| **Path normalization fails** | DENY | Potential directory traversal |

**Fail Mode Configuration**:
```php
'settings' => [
    'fail_mode' => 'deny',  // Options: 'deny', 'allow', 'fallback'
]

// 'deny': Default deny on errors (most secure)
// 'allow': Default allow on errors (most permissive)
// 'fallback': Fall back to global user permissions (balanced)
```

**Recommendation**: Use `'fail_mode' => 'deny'` for production.

### 5.4 Path Traversal Prevention

**Threat**: User requests `/projects/../admin/config.php` to bypass ACL rules.

**Mitigation**:

1. **Path Normalization**
   ```php
   function normalize_path($path) {
       // Security: Resolve . and .. components
       $path = realpath($path);

       // Prevent directory traversal
       if (strpos($path, '..') !== false) {
           throw new SecurityException('Path traversal detected');
       }

       return $path;
   }
   ```

2. **FileGator's Existing Protection**
   - `Filesystem::applyPathPrefix()` already prevents directory traversal
   - All paths go through `joinPaths()` which sanitizes input
   - User's homedir acts as chroot

3. **Additional ACL Layer Protection**
   ```php
   // In PathACL service
   $normalized_path = $this->normalizePath($requested_path);

   // Ensure path is within allowed repository
   if (!$this->isPathWithinRepository($normalized_path)) {
       return false;  // Deny access
   }
   ```

### 5.5 Permission Escalation Prevention

**Threat**: User modifies ACL config to grant themselves admin permissions.

**Mitigation**:

1. **File Permissions**
   ```bash
   # ACL config files owned by web server user
   chown www-data:www-data /private/acl_config.php
   chmod 640 /private/acl_config.php  # Read-only for web server
   ```

2. **No User-Editable ACL**
   - ACL configuration is admin-only
   - No API endpoint to modify ACL rules
   - Changes require server filesystem access

3. **Immutable Admin Rules**
   ```php
   // In PathACL service
   if ($username === 'admin' && !$user->hasRole('admin')) {
       throw new SecurityException('Cannot modify admin user');
   }
   ```

4. **Audit Logging**
   ```php
   // Log all ACL denials
   $this->logger->log(
       "ACL_DENY",
       [
           'username' => $user->getUsername(),
           'ip' => $clientIp,
           'path' => $path,
           'permission' => $permission,
           'reason' => $reason,
       ]
   );
   ```

### 5.6 Cache Poisoning Prevention

**Threat**: Attacker manipulates cache to bypass ACL checks.

**Mitigation**:

1. **Cache Key Includes All Dimensions**
   ```php
   $cache_key = md5(sprintf(
       'acl:%s:%s:%s:%s',
       $user->getUsername(),
       $clientIp,
       $path,
       $permission
   ));
   ```

2. **TTL-Based Expiration**
   ```php
   'settings' => [
       'cache_ttl' => 300,  // 5 minutes max
   ]
   ```

3. **Cache Invalidation**
   ```php
   // When ACL config changes
   $pathAcl->clearCache();
   ```

4. **Separate Cache Namespace**
   ```php
   // Use separate cache store for ACL data
   $cache = new PermissionCache('path_acl');  // Namespace isolation
   ```

---

## 6. Performance Optimization

### 6.1 Caching Strategy

**Cache Levels**:

1. **Permission Cache** (most granular)
   - Key: `acl:$username:$ip:$path:$permission`
   - Value: `boolean` (allowed/denied)
   - TTL: 5 minutes (configurable)

2. **Effective Permissions Cache** (broader)
   - Key: `acl_perms:$username:$ip:$path`
   - Value: `array` (list of granted permissions)
   - TTL: 5 minutes

3. **Rule Matching Cache** (intermediate)
   - Key: `acl_rules:$username:$ip:$path`
   - Value: `array` (matched rules with metadata)
   - TTL: 10 minutes

**Cache Implementation**:
```php
class PermissionCache
{
    private $cache;  // APCu, Redis, or Memcached

    public function get(string $key): ?bool
    {
        return $this->cache->get('path_acl:' . $key);
    }

    public function set(string $key, bool $value, int $ttl): void
    {
        $this->cache->set('path_acl:' . $key, $value, $ttl);
    }

    public function clear(): void
    {
        $this->cache->deleteByPrefix('path_acl:');
    }
}
```

### 6.2 Configuration Pre-Processing

**At Service Initialization**:

1. **Build Group Lookup Table**
   ```php
   // Convert groups to username → groups mapping
   // From: ['developers' => ['john', 'jane']]
   // To: ['john' => ['developers'], 'jane' => ['developers']]
   $this->userGroups = $this->buildUserGroupMap($config['groups']);
   ```

2. **Pre-Compile IP Ranges**
   ```php
   // Pre-validate all IP/CIDR entries at init
   foreach ($config['path_rules'] as $path => $acl) {
       foreach ($acl['rules'] as &$rule) {
           $rule['_compiled_allowlist'] = $this->compileIpList($rule['ip_allowlist']);
           $rule['_compiled_denylist'] = $this->compileIpList($rule['ip_denylist']);
       }
   }
   ```

3. **Index Rules by Path Depth**
   ```php
   // Group rules by path depth for faster lookup
   $this->rulesByDepth = [
       0 => [/* rules for / */],
       1 => [/* rules for /projects, /public, etc. */],
       2 => [/* rules for /projects/alpha, etc. */],
   ];
   ```

### 6.3 Optimization Techniques

**1. Early Termination**
```php
// Check user-level IP first (fast rejection)
if (!$this->checkUserIpAccess($user, $clientIp)) {
    return false;  // Don't even evaluate path rules
}

// Check cache before expensive evaluation
$cached = $this->cache->get($cacheKey);
if ($cached !== null) {
    return $cached;
}
```

**2. Lazy Rule Matching**
```php
// Don't collect all rules, stop at first 'override=true'
foreach ($this->getPathAncestors($path) as $ancestor) {
    $rules = $this->getMatchingRules($user, $clientIp, $ancestor);
    if ($rules) {
        $effectivePerms = $this->mergePermissions($rules);
        if ($this->hasOverrideRule($rules)) {
            break;  // Stop traversal
        }
    }
}
```

**3. Batch Permission Checks**
```php
// When checking multiple permissions for same path
$effectivePerms = $pathAcl->getEffectivePermissions($user, $ip, $path);
$canRead = in_array('read', $effectivePerms);
$canWrite = in_array('write', $effectivePerms);
$canDelete = in_array('delete', $effectivePerms);
// Single evaluation, multiple checks
```

**4. Symfony IpUtils is Already Fast**
- Uses binary operations for CIDR matching
- O(n) where n = number of IP ranges
- Acceptable for typical lists (< 100 entries)

### 6.4 Performance Benchmarks (Expected)

| Scenario | Expected Time | Notes |
|----------|---------------|-------|
| **Cache hit** | < 1ms | Direct memory lookup |
| **Cache miss, simple ACL (< 10 rules)** | 1-5ms | Full evaluation |
| **Cache miss, complex ACL (100 rules)** | 5-15ms | Multiple path traversals |
| **User IP check only** | < 0.5ms | Very fast (single IpUtils call) |
| **Path normalization** | < 0.5ms | String operations only |

**Optimization Goal**: 99% of requests served from cache (< 1ms overhead).

### 6.5 When to Disable Caching

**Disable caching if**:
- ACL rules change frequently (e.g., dynamic IP-based rules)
- User IPs change mid-session (mobile users, VPN switching)
- Debugging ACL evaluation (need fresh results)

**Configuration**:
```php
'settings' => [
    'cache_enabled' => false,  // Disable caching
]
```

---

## 7. Implementation Roadmap

### Phase 1: Foundation (Week 1-2)

**Goals**: Core ACL evaluation without caching

1. Create directory structure
   ```bash
   mkdir -p backend/Services/PathACL
   ```

2. Implement interfaces and utilities
   - `PathACLInterface.php`
   - `IpMatcher.php` (wrap Symfony IpUtils)
   - `PathMatcher.php` (path normalization, depth calculation)

3. Implement core `PathACL.php` service
   - Configuration loading
   - User-IP-Path matching
   - Permission evaluation algorithm

4. Unit tests
   - Test IP matching (CIDR, ranges, wildcards)
   - Test path matching (normalization, inheritance)
   - Test permission merging logic

**Deliverable**: Working PathACL service (no caching, no integration)

### Phase 2: Integration (Week 3)

**Goals**: Integrate with FileController and existing auth

1. Modify `User.php`
   - Add `ip_allowlist` and `ip_denylist` properties
   - Add getters/setters

2. Update `FileController.php`
   - Inject PathACL service
   - Add ACL checks before file operations
   - Handle 403 Forbidden responses

3. Register service in `configuration.php`

4. Create example `acl_config.php`

5. Integration tests
   - Test with JsonFile auth adapter
   - Test with various user roles
   - Test IP restrictions

**Deliverable**: Fully integrated ACL system

### Phase 3: Caching & Performance (Week 4)

**Goals**: Add caching and optimize

1. Implement `PermissionCache.php`
   - APCu backend (simple, no dependencies)
   - Optional Redis/Memcached support

2. Add cache layer to PathACL service
   - Cache permission check results
   - Cache effective permissions
   - Implement cache invalidation

3. Configuration pre-processing
   - Build lookup tables at init
   - Pre-compile IP ranges

4. Performance tests
   - Benchmark with 1000 rules
   - Cache hit/miss ratio analysis
   - Load testing

**Deliverable**: Production-ready ACL with caching

### Phase 4: Admin UI & Documentation (Week 5)

**Goals**: Make it easy to configure and manage

1. Admin UI for ACL management (optional)
   - Visual ACL rule editor
   - Permission simulator (test access)
   - Inheritance visualization

2. Documentation
   - Administrator guide
   - Configuration examples
   - Migration guide
   - API reference

3. Migration tools
   - Script to convert global permissions to path-based
   - Backup/restore for ACL config

**Deliverable**: Complete ACL system with documentation

### Phase 5: Advanced Features (Future)

**Goals**: Extended capabilities

1. Time-based restrictions
   - Business hours only
   - Expiring permissions

2. Audit logging
   - Log all ACL denials
   - Permission change history

3. Policy templates
   - Pre-defined ACL patterns
   - One-click setup

4. External policy engines
   - Integration with Open Policy Agent (OPA)
   - XACML support

**Deliverable**: Enterprise-grade ACL features

---

## 8. Testing Strategy

### 8.1 Unit Tests

**PathACL Core Logic**:
- `testSimplePermissionCheck()` - Basic allow/deny
- `testIpMatching()` - CIDR, ranges, wildcards
- `testPathInheritance()` - Parent to child cascading
- `testOverridePermissions()` - Explicit override behavior
- `testGroupMembership()` - @group expansion
- `testPriorityOrdering()` - Rule sorting
- `testPathNormalization()` - Security: directory traversal
- `testEmptyRuleSet()` - Default deny behavior

**IpMatcher Utility**:
- `testSingleIpMatch()` - 192.168.1.50
- `testCidrMatch()` - 192.168.1.0/24
- `testIpv6Match()` - 2001:db8::/32
- `testDenyOverridesAllow()` - Conflict resolution
- `testWildcardMatch()` - * matches all
- `testInvalidIpFormat()` - Error handling

**PathMatcher Utility**:
- `testPathDepth()` - /a/b/c = depth 3
- `testParentPath()` - /a/b/c → /a/b → /a → /
- `testPathNormalization()` - Remove .., //, etc.
- `testRootPath()` - Handle / correctly

### 8.2 Integration Tests

**FileController Integration**:
- `testReadAccessAllowed()` - User can read allowed path
- `testWriteAccessDenied()` - User cannot write restricted path
- `testUploadWithIpRestriction()` - Upload only from office network
- `testDeleteWithoutPermission()` - Delete fails gracefully
- `testMultiplePathsAccess()` - Batch operation permission checks

**Auth Adapter Integration**:
- `testJsonFileAuthWithACL()` - Works with JsonFile adapter
- `testDatabaseAuthWithACL()` - Works with Database adapter
- `testLDAPAuthWithACL()` - Works with LDAP adapter
- `testFallbackToGlobalPermissions()` - When ACL disabled

**Caching Tests**:
- `testCacheHit()` - Cached result returned
- `testCacheMiss()` - Fresh evaluation on miss
- `testCacheInvalidation()` - Clear cache works
- `testCacheExpiration()` - TTL expiration works

### 8.3 Security Tests

**Path Traversal**:
- `testDirectoryTraversalBlocked()` - /projects/../admin denied
- `testSymlinkFollowing()` - Symlinks handled securely
- `testAbsolutePathRejection()` - /etc/passwd rejected

**IP Spoofing**:
- `testUntrustedProxyIgnored()` - X-Forwarded-For from untrusted source ignored
- `testTrustedProxyAccepted()` - X-Forwarded-For from trusted proxy used
- `testDirectConnectionIp()` - Socket IP used when no proxy

**Permission Escalation**:
- `testUserCannotModifyACL()` - No API to change ACL
- `testAdminBypassNotAllowed()` - Even admins follow ACL rules
- `testDenyAlwaysWins()` - Deny overrides allow

### 8.4 Performance Tests

**Load Testing**:
- Simulate 1000 concurrent users
- Measure ACL evaluation latency
- Measure cache hit ratio
- Identify bottlenecks

**Benchmarks**:
- Evaluate 10 rules: < 5ms
- Evaluate 100 rules: < 15ms
- Evaluate 1000 rules: < 50ms
- Cache hit: < 1ms

### 8.5 Test Configuration Examples

**Simple Test Config**:
```php
'path_rules' => [
    '/' => [
        'inherit' => false,
        'rules' => [
            ['users' => ['testuser'], 'ip_allowlist' => ['*'], 'permissions' => ['read']],
        ],
    ],
]
```

**Complex Test Config**:
```php
'path_rules' => [
    '/' => [
        'rules' => [
            ['users' => ['*'], 'ip_allowlist' => ['*'], 'permissions' => ['read'], 'priority' => 0],
        ],
    ],
    '/restricted' => [
        'rules' => [
            ['users' => ['admin'], 'ip_allowlist' => ['192.168.1.0/24'], 'permissions' => ['read','write'], 'priority' => 100],
        ],
    ],
    '/restricted/secret' => [
        'inherit' => false,
        'rules' => [
            ['users' => ['admin'], 'ip_allowlist' => ['192.168.1.10'], 'permissions' => ['read'], 'priority' => 100],
        ],
    ],
]
```

---

## 9. Configuration Examples

### 9.1 Example 1: Office Network Only

**Scenario**: Only allow access from corporate office network.

```php
'path_rules' => [
    '/' => [
        'inherit' => false,
        'rules' => [
            [
                'users' => ['*'],
                'ip_allowlist' => ['203.0.113.0/24'],  // Office network
                'ip_denylist' => [],
                'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                'priority' => 50,
            ],
        ],
    ],
]
```

### 9.2 Example 2: VPN Users Read-Only, Office Full Access

**Scenario**: VPN users can read, but only office users can modify.

```php
'path_rules' => [
    '/' => [
        'inherit' => false,
        'rules' => [
            // VPN users: read-only
            [
                'users' => ['*'],
                'ip_allowlist' => ['10.8.0.0/24'],  // VPN network
                'ip_denylist' => [],
                'permissions' => ['read', 'download'],
                'priority' => 40,
            ],
            // Office users: full access
            [
                'users' => ['*'],
                'ip_allowlist' => ['192.168.1.0/24'],  // Office network
                'ip_denylist' => [],
                'permissions' => ['read', 'write', 'upload', 'download', 'delete', 'zip'],
                'priority' => 50,
            ],
        ],
    ],
]
```

### 9.3 Example 3: Department Folders with IP Restrictions

**Scenario**: Each department has a folder, accessible only from department IPs.

```php
'groups' => [
    'hr-staff' => ['susan', 'tom', 'mary'],
    'developers' => ['john', 'jane', 'bob'],
],

'path_rules' => [
    '/' => [
        'rules' => [
            ['users' => ['*'], 'ip_allowlist' => ['*'], 'permissions' => ['read'], 'priority' => 0],
        ],
    ],

    '/hr' => [
        'inherit' => true,
        'rules' => [
            [
                'users' => ['@hr-staff'],
                'ip_allowlist' => ['192.168.1.0/24'],  // HR department subnet
                'ip_denylist' => [],
                'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                'priority' => 60,
                'override_inherited' => true,
            ],
        ],
    ],

    '/engineering' => [
        'inherit' => true,
        'rules' => [
            [
                'users' => ['@developers'],
                'ip_allowlist' => ['192.168.2.0/24'],  // Engineering subnet
                'ip_denylist' => [],
                'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                'priority' => 60,
                'override_inherited' => true,
            ],
        ],
    ],
]
```

### 9.4 Example 4: Public Upload, Admin Review

**Scenario**: Public users can upload from anywhere, but only admins can view/download.

```php
'path_rules' => [
    '/' => [
        'rules' => [
            ['users' => ['admin'], 'ip_allowlist' => ['*'], 'permissions' => ['read','write','upload','download','delete'], 'priority' => 100],
        ],
    ],

    '/uploads' => [
        'inherit' => true,
        'rules' => [
            // Anyone can upload (write-only)
            [
                'users' => ['*'],
                'ip_allowlist' => ['*'],
                'ip_denylist' => [],
                'permissions' => ['upload'],  // Can upload but cannot read
                'priority' => 30,
                'override_inherited' => false,  // Merge with admin permissions
            ],
        ],
    ],
]
```

### 9.5 Example 5: Time-Sensitive Contractor Access

**Scenario**: Contractors can access project files, but only from VPN.

```php
'groups' => [
    'contractors' => ['alice', 'charlie', 'david'],
],

'path_rules' => [
    '/' => [
        'rules' => [
            ['users' => ['*'], 'ip_allowlist' => ['*'], 'permissions' => ['read'], 'priority' => 0],
        ],
    ],

    '/projects/client-x' => [
        'inherit' => true,
        'rules' => [
            [
                'users' => ['@contractors'],
                'ip_allowlist' => ['10.8.0.0/24'],  // VPN only
                'ip_denylist' => [],
                'permissions' => ['read', 'download'],  // Read-only
                'priority' => 50,
                'override_inherited' => true,
            ],
        ],
    ],
]
```

### 9.6 Example 6: Admin Override from Specific IP

**Scenario**: Admins have full access, but sensitive operations only from specific IP.

```php
'path_rules' => [
    '/' => [
        'rules' => [
            ['users' => ['admin'], 'ip_allowlist' => ['*'], 'permissions' => ['read','write','upload','download'], 'priority' => 100],
        ],
    ],

    '/system/config' => [
        'inherit' => false,  // Don't inherit global admin permissions
        'rules' => [
            [
                'users' => ['admin'],
                'ip_allowlist' => ['192.168.1.10'],  // Only from admin workstation
                'ip_denylist' => [],
                'permissions' => ['read', 'write', 'delete', 'chmod'],
                'priority' => 200,
            ],
        ],
    ],
]
```

---

## 10. Migration Guide

### 10.1 From Global Permissions to Path-Based ACL

**Step 1: Backup Current Configuration**
```bash
cp private/users.json private/users.json.backup
cp configuration.php configuration.php.backup
```

**Step 2: Enable Path-Based ACL (Gradual Rollout)**
```php
// configuration.php
'Filegator\Services\PathACL\PathACLInterface' => [
    'handler' => '\Filegator\Services\PathACL\PathACL',
    'config' => [
        'acl_config_file' => __DIR__.'/private/acl_config.php',
        'enabled' => false,  // Start disabled
    ],
],
```

**Step 3: Create Initial ACL Config**
```php
// private/acl_config.php
return [
    'enabled' => true,
    'settings' => [
        'evaluation_mode' => 'most_specific_wins',
        'default_inherit' => true,
        'deny_overrides_allow' => true,
    ],
    'groups' => [],
    'path_rules' => [
        '/' => [
            'inherit' => false,
            'rules' => [
                // Replicate current global permissions
                [
                    'users' => ['*'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download'],  // Adjust based on your setup
                    'priority' => 0,
                ],
            ],
        ],
    ],
];
```

**Step 4: Test with ACL Enabled**
```php
// configuration.php
'enabled' => true,  // Enable PathACL
```

Verify existing users can still access files normally.

**Step 5: Add IP Restrictions Gradually**
```php
// Add IP restrictions for specific users
'path_rules' => [
    '/' => [
        'rules' => [
            [
                'users' => ['john'],
                'ip_allowlist' => ['192.168.1.0/24'],  // Add restriction for john
                'ip_denylist' => [],
                'permissions' => ['read', 'write', 'upload', 'download'],
                'priority' => 50,
            ],
            [
                'users' => ['*'],
                'ip_allowlist' => ['*'],
                'ip_denylist' => [],
                'permissions' => ['read', 'write', 'upload', 'download'],
                'priority' => 0,
            ],
        ],
    ],
]
```

**Step 6: Add Per-Folder Rules**
```php
'path_rules' => [
    '/' => [ /* existing root rule */ ],
    '/projects' => [
        'inherit' => true,
        'rules' => [
            [
                'users' => ['@developers'],
                'ip_allowlist' => ['192.168.1.0/24'],
                'ip_denylist' => [],
                'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                'priority' => 60,
                'override_inherited' => true,
            ],
        ],
    ],
]
```

### 10.2 Migration Script (Automatic)

```php
<?php
// scripts/migrate_to_path_acl.php

// Load existing users
$users = json_decode(file_get_contents('private/users.json'), true);

// Generate initial ACL config
$acl_config = [
    'enabled' => true,
    'settings' => [
        'evaluation_mode' => 'most_specific_wins',
        'default_inherit' => true,
        'deny_overrides_allow' => true,
        'cache_enabled' => true,
        'cache_ttl' => 300,
    ],
    'groups' => [],
    'path_rules' => [
        '/' => [
            'inherit' => false,
            'rules' => [],
        ],
    ],
];

// Convert each user's global permissions to path-based
foreach ($users as $user) {
    $permissions = explode('|', $user['permissions']);

    $acl_config['path_rules']['/']['rules'][] = [
        'users' => [$user['username']],
        'ip_allowlist' => ['*'],
        'ip_denylist' => [],
        'permissions' => $permissions,
        'priority' => 50,
    ];
}

// Write ACL config
file_put_contents(
    'private/acl_config.php',
    '<?php return ' . var_export($acl_config, true) . ';'
);

echo "Migration complete. Review private/acl_config.php\n";
```

### 10.3 Rollback Plan

**If something goes wrong**:

1. **Disable PathACL**
   ```php
   // configuration.php
   'enabled' => false,
   ```

2. **Restore backup**
   ```bash
   cp private/users.json.backup private/users.json
   cp configuration.php.backup configuration.php
   ```

3. **Clear cache**
   ```bash
   rm -rf private/cache/*
   php scripts/clear_cache.php
   ```

---

## 11. FAQ & Troubleshooting

### Q: Can a user access files if ACL is disabled?

**A:** Yes. If `enabled => false`, FileGator falls back to the existing global permission system. Users access files based on their `permissions` property in `users.json`.

### Q: What happens if a user's IP changes mid-session?

**A:** The ACL evaluation uses the current IP on every request. If the IP changes (e.g., mobile user switches networks), their permissions are re-evaluated based on the new IP.

**Recommendation**: Use short cache TTL (1-5 minutes) for environments with dynamic IPs.

### Q: Can I use domain names instead of IP addresses?

**A:** No. The system uses IP addresses only. If you need domain-based restrictions, resolve the domain to IPs and add all IPs to the allowlist:

```bash
# Get IPs for domain
nslookup example.com
dig example.com +short

# Add to allowlist
'ip_allowlist' => ['203.0.113.10', '203.0.113.11']
```

### Q: Does this work with CloudFlare or other CDNs?

**A:** Yes, but you must configure trusted proxies correctly:

```php
'settings' => [
    'trusted_proxies' => [
        '127.0.0.1',
        // Add CloudFlare IPs from https://www.cloudflare.com/ips/
        '173.245.48.0/20',
        '103.21.244.0/22',
        // ... (add all CloudFlare ranges)
    ],
]
```

### Q: How do I debug why a user is denied access?

**A:** Use the `explainPermission()` method:

```php
$explanation = $pathAcl->explainPermission($user, $clientIp, $path, $permission);
print_r($explanation);

// Output:
// [
//   'allowed' => false,
//   'reason' => 'IP not in allowlist',
//   'matched_rules' => [...],
//   'effective_permissions' => ['read'],
//   'requested_permission' => 'write',
// ]
```

### Q: What's the performance impact?

**A:** With caching enabled:
- **Cache hit**: < 1ms overhead (negligible)
- **Cache miss**: 1-15ms depending on rule complexity
- **Expected cache hit ratio**: > 99%

Without caching: 5-20ms per request.

### Q: Can I use wildcards in paths?

**A:** No. Path matching is exact (not glob patterns). However, inheritance provides similar functionality:

```php
// Instead of: '/projects/*' (not supported)
// Use:
'/projects' => [
    'inherit' => true,  // Applies to all descendants
    'rules' => [...]
]
```

### Q: How do I block a specific IP from everything?

**A:** Add to user-level denylist:

```json
{
  "username": "john",
  "ip_denylist": ["192.0.2.50"]
}
```

Or add a global deny rule at `/`:

```php
'/' => [
    'rules' => [
        [
            'users' => ['*'],
            'ip_allowlist' => ['*'],
            'ip_denylist' => ['192.0.2.50'],  // Blocked globally
            'permissions' => ['read', 'write', 'upload', 'download'],
            'priority' => 0,
        ],
    ],
]
```

### Q: Can I test ACL rules without affecting users?

**A:** Yes. Use `explainPermission()` in a test script:

```php
// test_acl.php
$user = $auth->find('john');
$ip = '192.168.1.50';
$path = '/projects/alpha/file.txt';
$permission = 'write';

$result = $pathAcl->explainPermission($user, $ip, $path, $permission);
echo json_encode($result, JSON_PRETTY_PRINT);
```

### Q: What if ACL config file is corrupted?

**A:** The system behavior depends on `fail_mode` setting:

- `fail_mode = 'deny'`: Deny all access (safe default)
- `fail_mode = 'allow'`: Allow all access (permissive)
- `fail_mode = 'fallback'`: Use global user permissions

**Recommendation**: Use `'deny'` in production with proper error logging.

---

## 12. Appendix

### A. FileGator Permission Types

| Permission | Description | Common Operations |
|------------|-------------|-------------------|
| `read` | View files and folders | List directory, view file properties |
| `write` | Create/modify files | Edit text files, create new files |
| `upload` | Upload files via HTTP | Upload button, drag-and-drop |
| `download` | Download files via HTTP | Download button, ZIP download |
| `batchdownload` | Download multiple files as ZIP | Batch download |
| `delete` | Delete files and folders | Delete button, trash |
| `zip` | Create ZIP archives | Compress button |
| `chmod` | Change file permissions | Chmod command (Unix) |

### B. IP CIDR Reference

| Notation | IP Range | Number of IPs |
|----------|----------|---------------|
| `192.168.1.0/32` | 192.168.1.0 | 1 IP (single IP) |
| `192.168.1.0/24` | 192.168.1.0 - 192.168.1.255 | 256 IPs |
| `192.168.0.0/16` | 192.168.0.0 - 192.168.255.255 | 65,536 IPs |
| `10.0.0.0/8` | 10.0.0.0 - 10.255.255.255 | 16,777,216 IPs |
| `0.0.0.0/0` | 0.0.0.0 - 255.255.255.255 | All IPv4 |
| `2001:db8::/32` | IPv6 subnet | 2^96 IPs |

### C. Symfony IpUtils Reference

```php
use Symfony\Component\HttpFoundation\IpUtils;

// Check if IP is in list
IpUtils::checkIp('192.168.1.50', ['192.168.1.0/24']); // true

// Check multiple ranges
IpUtils::checkIp('10.0.0.50', ['192.168.0.0/16', '10.0.0.0/8']); // true

// IPv6 support
IpUtils::checkIp('2001:db8::1', ['2001:db8::/32']); // true

// Anonymize IP (GDPR)
IpUtils::anonymize('192.168.1.50'); // "192.168.1.0"
```

### D. Configuration Template

Complete production-ready template available at:
`/mnt/ai/filegator/docs/design/acl_config.template.php`

### E. Related Documents

- **Architecture Analysis**: `/mnt/ai/filegator/docs/filegator-architecture-analysis.md`
- **IP Matching Research**: `/mnt/ai/filegator/docs/research/ip-matching-best-practices.md`
- **Path ACL Research**: `/mnt/ai/filegator/docs/research/path-based-acl-research.md`

### F. Support & Resources

- **FileGator Documentation**: https://docs.filegator.io/
- **Symfony IpUtils**: https://symfony.com/doc/current/components/http_foundation.html
- **CIDR Calculator**: https://www.subnet-calculator.com/cidr.php
- **IP Geolocation**: https://www.maxmind.com/en/geoip2-services-and-databases

---

## Document Revision History

| Version | Date | Changes | Author |
|---------|------|---------|--------|
| 1.0 | 2025-12-09 | Initial design complete | System Architect |

---

**End of Design Document**

This design is ready for implementation. Proceed to Phase 1 development.
