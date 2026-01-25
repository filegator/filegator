# Path ACL Troubleshooting Guide

This guide helps diagnose and resolve common issues with FileGator's Path-Based ACL system.

## Table of Contents

- [Common Issues](#common-issues)
- [Debugging Techniques](#debugging-techniques)
- [Using explainPermission](#using-explainpermission)
- [Logging Configuration](#logging-configuration)
- [Performance Issues](#performance-issues)
- [Cache Management](#cache-management)
- [Configuration Errors](#configuration-errors)
- [IP Detection Problems](#ip-detection-problems)

## Common Issues

### User Denied Access Unexpectedly

**Symptoms**: User reports "Access Denied" but should have permission.

**Diagnosis Steps**:

1. Verify ACL is enabled:
```php
// In configuration.php
'config' => [
    'enabled' => true,  // Must be true
]
```

2. Check user's IP address:
```php
// Create debug script: debug_ip.php
<?php
require 'backend/Services/Http/Request.php';
$request = new Request();
echo "Detected IP: " . $request->getClientIp() . "\n";
```

3. Use `explainPermission()` to see why access was denied:
```php
$result = $pathAcl->explainPermission($user, $clientIp, $path, $permission);
print_r($result);
```

**Common Causes**:

| Cause | Solution |
|-------|----------|
| IP not in allowlist | Add user's IP to `ip_allowlist` |
| IP in denylist | Remove from `ip_denylist` or use different IP |
| Wrong path in config | Check path matches exactly (case-sensitive) |
| Inheritance disabled | Set `'inherit' => true` on parent path |
| Low priority rule overridden | Increase `priority` value |
| Permission not listed | Add missing permission to `permissions` array |

---

### Wrong IP Address Detected

**Symptoms**: System detects wrong IP (often shows proxy IP instead of client IP).

**Cause**: FileGator is behind a reverse proxy but trusted proxies not configured.

**Solution**: Configure trusted proxies in ACL settings:

```php
'settings' => [
    'trusted_proxies' => [
        '127.0.0.1',      // localhost
        '10.0.0.1',       // nginx proxy IP
        '172.17.0.0/16',  // Docker network
    ],
]
```

**Verification**:

```php
// Check X-Forwarded-For header handling
var_dump($_SERVER['HTTP_X_FORWARDED_FOR']);  // Client IP
var_dump($_SERVER['REMOTE_ADDR']);           // Proxy IP

// After trusted proxy config, getClientIp() should return client IP
echo $request->getClientIp();
```

---

### Permissions Not Inherited

**Symptoms**: Child folders don't inherit parent permissions.

**Common Causes**:

1. **Inheritance disabled**:
```php
'/parent' => [
    'inherit' => false,  // Blocks inheritance to children
]
```
Fix: Set `'inherit' => true`

2. **Child overrides parent**:
```php
'/parent/child' => [
    'rules' => [
        [
            'override_inherited' => true,  // Replaces parent permissions
        ]
    ]
]
```
Fix: Set `'override_inherited' => false` to merge instead

3. **No matching rule at parent level**:
Check that a rule exists for the user/IP at the parent path.

---

### Cache Returns Stale Permissions

**Symptoms**: Changes to ACL config not taking effect immediately.

**Cause**: Permission cache is enabled with long TTL.

**Solution**: Clear cache after configuration changes:

```php
// Option 1: Programmatically
$pathAcl->clearCache();

// Option 2: Via CLI script
<?php
// scripts/clear_acl_cache.php
require __DIR__ . '/../backend/bootstrap.php';
$container = get_service_container();
$pathAcl = $container->get('Filegator\Services\PathACL\PathACLInterface');
$pathAcl->clearCache();
echo "ACL cache cleared.\n";
```

Run: `php scripts/clear_acl_cache.php`

**Prevent in Development**: Disable cache during testing:
```php
'settings' => [
    'cache_enabled' => false,  // Disable for development
]
```

---

### Groups Not Recognized

**Symptoms**: Group rules don't apply to users.

**Verification**:

```php
// Check group membership
$groups = $config['groups'];
$username = 'john';

foreach ($groups as $groupName => $members) {
    if (in_array($username, $members)) {
        echo "User '$username' is in group: $groupName\n";
    }
}
```

**Common Issues**:

1. **Missing @ prefix**:
```php
// Wrong
'users' => ['developers']

// Correct
'users' => ['@developers']
```

2. **Group not defined**:
```php
// Must exist in config
'groups' => [
    'developers' => ['john', 'jane'],
]
```

3. **Username typo**:
Usernames are case-sensitive and must match exactly.

---

### Performance Degradation

**Symptoms**: Slow file operations, timeouts.

**Diagnosis**: Check ACL evaluation time:

```php
$start = microtime(true);
$allowed = $pathAcl->checkPermission($user, $clientIp, $path, $permission);
$elapsed = (microtime(true) - $start) * 1000;
echo "ACL check took: {$elapsed}ms\n";
```

**Acceptable Benchmarks**:
- Cache hit: < 1ms
- Cache miss (simple config): 1-5ms
- Cache miss (complex config): 5-15ms

**Performance Issues**:

| Problem | Solution |
|---------|----------|
| Every request > 20ms | Enable caching, reduce rule count |
| Cache hit ratio < 90% | Increase cache TTL |
| Too many rules | Consolidate using groups, simplify paths |
| Complex IP lists | Pre-compile IP ranges at init |

---

## Debugging Techniques

### Enable Debug Logging

Add logging to ACL checks:

```php
// In PathACL.php or custom wrapper
public function checkPermission($user, $clientIp, $path, $permission): bool
{
    $result = $this->evaluatePermission($user, $clientIp, $path, $permission);

    // Log the decision
    error_log(sprintf(
        "[ACL] user=%s ip=%s path=%s perm=%s result=%s",
        $user->getUsername(),
        $clientIp,
        $path,
        $permission,
        $result ? 'ALLOW' : 'DENY'
    ));

    return $result;
}
```

Log location: `/private/logs/acl.log`

---

### Test Specific Scenarios

Create a test script:

```php
<?php
// test_acl.php

require __DIR__ . '/backend/bootstrap.php';
$container = get_service_container();
$pathAcl = $container->get('Filegator\Services\PathACL\PathACLInterface');
$auth = $container->get('Filegator\Services\Auth\AuthInterface');

// Test parameters
$username = 'john';
$clientIp = '192.168.1.50';
$path = '/projects/file.txt';
$permission = 'write';

// Get user
$user = $auth->find($username);

if (!$user) {
    die("User not found: $username\n");
}

// Test permission
$allowed = $pathAcl->checkPermission($user, $clientIp, $path, $permission);
echo "Access " . ($allowed ? "ALLOWED" : "DENIED") . "\n\n";

// Get explanation
$explanation = $pathAcl->explainPermission($user, $clientIp, $path, $permission);
print_r($explanation);
```

Run: `php test_acl.php`

---

### Validate Configuration

Check for common config errors:

```php
<?php
// validate_acl_config.php

$config = require __DIR__ . '/private/acl_config.php';

// Check required keys
$required = ['enabled', 'settings', 'groups', 'path_rules'];
foreach ($required as $key) {
    if (!isset($config[$key])) {
        echo "ERROR: Missing required key: $key\n";
    }
}

// Validate groups
foreach ($config['groups'] as $groupName => $members) {
    if (!is_array($members)) {
        echo "ERROR: Group '$groupName' members must be array\n";
    }
}

// Validate path rules
foreach ($config['path_rules'] as $path => $pathConfig) {
    if (!isset($pathConfig['rules'])) {
        echo "ERROR: Path '$path' missing 'rules' key\n";
    }

    foreach ($pathConfig['rules'] as $i => $rule) {
        // Check required rule keys
        if (!isset($rule['users'])) {
            echo "ERROR: Rule #$i in path '$path' missing 'users'\n";
        }
        if (!isset($rule['ip_allowlist'])) {
            echo "ERROR: Rule #$i in path '$path' missing 'ip_allowlist'\n";
        }
        if (!isset($rule['permissions'])) {
            echo "ERROR: Rule #$i in path '$path' missing 'permissions'\n";
        }

        // Validate IP formats
        if (isset($rule['ip_allowlist'])) {
            foreach ($rule['ip_allowlist'] as $ip) {
                if ($ip !== '*' && !filter_var($ip, FILTER_VALIDATE_IP) && !preg_match('/^[\d.\/]+$/', $ip)) {
                    echo "WARNING: Possibly invalid IP format: $ip\n";
                }
            }
        }
    }
}

echo "Validation complete.\n";
```

---

## Using explainPermission

The `explainPermission()` method provides detailed debugging information:

```php
$explanation = $pathAcl->explainPermission($user, $clientIp, $path, $permission);
```

**Output Structure**:

```php
[
    'allowed' => false,                    // Final decision
    'reason' => 'IP not in allowlist',     // Denial reason
    'user' => 'john',                      // Username
    'client_ip' => '192.168.1.50',        // Client IP
    'path' => '/projects/file.txt',        // Requested path
    'permission' => 'write',               // Requested permission
    'matched_rules' => [                   // Rules that matched
        [
            'path' => '/projects',
            'rule' => [ /* rule details */ ],
            'matched_users' => true,
            'matched_ip' => false,         // IP didn't match
        ],
    ],
    'effective_permissions' => ['read'],   // Granted permissions
    'evaluation_steps' => [                // Step-by-step evaluation
        'Step 1: User IP check - PASSED',
        'Step 2: Find rules for /projects - 1 rule found',
        'Step 3: IP 192.168.1.50 not in allowlist - DENIED',
    ],
]
```

**Usage Example**:

```php
$explanation = $pathAcl->explainPermission($user, $clientIp, $path, $permission);

if (!$explanation['allowed']) {
    echo "Access denied: {$explanation['reason']}\n";
    echo "Effective permissions: " . implode(', ', $explanation['effective_permissions']) . "\n";
    echo "\nEvaluation steps:\n";
    foreach ($explanation['evaluation_steps'] as $step) {
        echo "  - $step\n";
    }
}
```

---

## Logging Configuration

Configure detailed ACL logging:

```php
'settings' => [
    'log_enabled' => true,
    'log_file' => __DIR__ . '/../logs/acl.log',
    'log_level' => 'debug',  // 'error', 'warning', 'info', 'debug'
    'log_denied_only' => false,  // Log all decisions or only denials
]
```

**Log Format**:

```
[2025-12-09 14:23:15] ACL DENY: user=john ip=192.168.1.50 path=/restricted perm=read reason="No matching rules"
[2025-12-09 14:23:20] ACL ALLOW: user=jane ip=192.168.1.100 path=/projects/file.txt perm=write
```

**Rotate Logs**:

```bash
# logrotate configuration
/var/www/filegator/private/logs/acl.log {
    daily
    rotate 30
    compress
    missingok
    notifempty
}
```

---

## Performance Issues

### High Latency on Every Request

**Diagnosis**:

```php
// Add timing to ACL checks
$start = microtime(true);
$result = $pathAcl->checkPermission($user, $clientIp, $path, $permission);
$time = (microtime(true) - $start) * 1000;

if ($time > 10) {
    error_log("SLOW ACL check: {$time}ms for path=$path");
}
```

**Solutions**:

1. **Enable caching**:
```php
'cache_enabled' => true,
'cache_ttl' => 300,  // 5 minutes
```

2. **Reduce rule complexity**:
```php
// Bad: 100 individual user rules
'users' => ['user1', 'user2', /* ... 100 users */]

// Good: Use groups
'groups' => ['team' => ['user1', 'user2', /* ... */]],
'users' => ['@team']
```

3. **Optimize IP lists**:
```php
// Bad: Many individual IPs
'ip_allowlist' => ['192.168.1.1', '192.168.1.2', /* ... */]

// Good: Use CIDR
'ip_allowlist' => ['192.168.1.0/24']
```

4. **Limit path depth**:
Deep folder hierarchies require more rule lookups. Consider flatter structures.

---

### Cache Not Working

**Verification**:

```php
// Check cache is enabled
$isEnabled = $pathAcl->isCacheEnabled();
echo "Cache enabled: " . ($isEnabled ? 'yes' : 'no') . "\n";

// Check cache hit rate
// Add counters to PathACL service
private $cacheHits = 0;
private $cacheMisses = 0;

public function getCacheStats(): array
{
    return [
        'hits' => $this->cacheHits,
        'misses' => $this->cacheMisses,
        'hit_rate' => $this->cacheHits / ($this->cacheHits + $this->cacheMisses),
    ];
}
```

**Common Issues**:

1. **APCu not installed**:
```bash
# Check PHP modules
php -m | grep apcu

# Install if missing
sudo apt-get install php-apcu
sudo systemctl restart php-fpm
```

2. **Cache storage full**:
```bash
# Check APCu status
php -r "print_r(apcu_cache_info());"

# Clear if needed
php -r "apcu_clear_cache();"
```

3. **Short TTL**:
```php
// Too short
'cache_ttl' => 10,  // 10 seconds

// Better
'cache_ttl' => 300,  // 5 minutes
```

---

## Cache Management

### Manual Cache Control

```php
// Clear entire ACL cache
$pathAcl->clearCache();

// Clear cache for specific user
$pathAcl->clearCache(['user' => 'john']);

// Clear cache for specific path
$pathAcl->clearCache(['path' => '/projects']);
```

### Automatic Cache Invalidation

Invalidate cache when config changes:

```php
// In admin panel or config update script
if ($configChanged) {
    $pathAcl->clearCache();
    error_log("ACL config updated, cache cleared");
}
```

### Cache Warming

Pre-populate cache for common paths:

```php
// scripts/warm_acl_cache.php
$commonPaths = ['/', '/projects', '/public', '/uploads'];
$users = $auth->getAllUsers();

foreach ($users as $user) {
    foreach ($commonPaths as $path) {
        // Trigger cache population
        $pathAcl->getEffectivePermissions($user, $user->getIp(), $path);
    }
}
```

---

## Configuration Errors

### Syntax Errors in Config File

**Symptoms**: PHP fatal error when loading config.

**Diagnosis**:

```bash
# Check PHP syntax
php -l private/acl_config.php

# Check for common issues
php -r "var_dump(require 'private/acl_config.php');"
```

**Common Mistakes**:

```php
// Missing comma
'permissions' => ['read', 'write'  'delete']  // Error
'permissions' => ['read', 'write', 'delete']  // Correct

// Wrong quotes
'users' => ["john', 'jane"]  // Error: mixed quotes
'users' => ['john', 'jane']  // Correct

// Trailing comma in array (PHP < 7.2)
'permissions' => ['read',]  // May error in old PHP
'permissions' => ['read']   // Safe
```

---

### IP Format Errors

**Symptoms**: IPs not matching or warnings in logs.

**Valid Formats**:

```php
// Correct
'192.168.1.50'        // Single IP
'192.168.1.0/24'      // CIDR notation
'10.0.0.0/8'          // Large network
'2001:db8::/32'       // IPv6 CIDR
'*'                   // Wildcard

// Incorrect
'192.168.1.*'         // No wildcard in octets (use CIDR)
'192.168.1.0-255'     // No range notation (use CIDR)
'192.168.1'           // Incomplete IP
```

**Validation Script**:

```php
use Symfony\Component\HttpFoundation\IpUtils;

$testIps = ['192.168.1.50', '10.0.0.0/8', '192.168.1.*'];

foreach ($testIps as $ip) {
    try {
        // Test if format is valid
        IpUtils::checkIp('127.0.0.1', [$ip]);
        echo "$ip - Valid\n";
    } catch (Exception $e) {
        echo "$ip - Invalid: {$e->getMessage()}\n";
    }
}
```

---

## IP Detection Problems

### Behind Multiple Proxies

**Scenario**: Request goes through multiple proxies (CDN → Load Balancer → nginx → PHP-FPM).

**Solution**: Trust all proxies in chain:

```php
'trusted_proxies' => [
    '10.0.0.1',        // Load balancer
    '10.0.0.2',        // nginx
    '172.17.0.0/16',   // Docker network
    // CloudFlare IPs
    '173.245.48.0/20',
    '103.21.244.0/22',
    // ... (add all CloudFlare ranges)
]
```

Get CloudFlare IPs: https://www.cloudflare.com/ips/

---

### IPv6 Issues

**Symptoms**: IPv6 users can't access, or IPs not detected correctly.

**Check IPv6 Support**:

```php
// Test IPv6 detection
$ipv6 = '2001:db8::1';
echo filter_var($ipv6, FILTER_VALIDATE_IP, FILTER_FLAG_IPV6) ? 'Valid' : 'Invalid';
```

**Common Issues**:

1. **IPv6 not in allowlist**:
```php
// Add IPv6 ranges
'ip_allowlist' => [
    '192.168.1.0/24',  // IPv4
    '2001:db8::/32',   // IPv6
]
```

2. **IPv6 normalization**:
IPv6 addresses have multiple formats. Symfony IpUtils handles this automatically.

---

## Emergency Disable

If ACL is causing critical issues, quickly disable:

```php
// configuration.php - Set to false
'config' => [
    'enabled' => false,  // Disables ACL, falls back to global permissions
]
```

Or in ACL config:

```php
// private/acl_config.php
return [
    'enabled' => false,  // Emergency disable
    // ... rest of config
];
```

---

## Getting Help

If issues persist:

1. **Check logs**: `/private/logs/acl.log` and PHP error log
2. **Use explainPermission()**: Get detailed decision reasoning
3. **Test in isolation**: Create minimal test config
4. **Verify basics**: PHP version, Symfony components, file permissions
5. **Community support**:
   - GitHub Issues: https://github.com/filegator/filegator/issues
   - Discussions: https://github.com/filegator/filegator/discussions

**Include in Bug Reports**:
- FileGator version
- PHP version
- ACL configuration (sanitized)
- Output of `explainPermission()`
- Relevant log entries
- Expected vs actual behavior
