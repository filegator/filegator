# PathACL Integration Guide

## Overview

The PathACL service has been successfully integrated with FileGator's authentication system and file controllers. This provides three-dimensional access control based on:

1. **User Identity** - Username and group membership
2. **Source IP Address** - IPv4/IPv6 with CIDR support  
3. **File/Folder Path** - With inheritance from parent directories

## Integration Points

### 1. User Class Extensions

**File**: `/mnt/ai/filegator/backend/Services/Auth/User.php`

Added user-level IP restrictions:
- `ip_allowlist` - Array of allowed IP addresses/CIDR blocks
- `ip_denylist` - Array of denied IP addresses/CIDR blocks
- Getter and setter methods: `getIpAllowlist()`, `setIpAllowlist()`, etc.
- Included in JSON serialization for API responses

### 2. Auth Adapter Updates

**File**: `/mnt/ai/filegator/backend/Services/Auth/Adapters/JsonFile.php`

Updated to load and save IP restrictions:
- `mapToUserObject()` - Loads IP restrictions from JSON
- `update()` - Saves IP restrictions when updating users
- `add()` - Saves IP restrictions when creating users

### 3. Controller Integration

All file operation controllers now check PathACL permissions before executing operations:

#### FileController.php
- `getDirectory()` - Checks 'read' permission
- `createNew()` - Checks 'write' permission
- `copyItems()` - Checks 'read' on source, 'write' on destination
- `moveItems()` - Checks 'write' on both source and destination
- `zipItems()` - Checks 'zip' on destination, 'read' on items
- `unzipItem()` - Checks 'read' on archive, 'write' on destination
- `chmodItems()` - Checks 'chmod' permission
- `renameItem()` - Checks 'write' permission
- `deleteItems()` - Checks 'write' permission
- `saveContent()` - Checks 'write' permission

#### DownloadController.php
- `download()` - Checks 'download' permission
- `batchDownloadCreate()` - Checks 'download' on all items

#### UploadController.php
- `upload()` - Checks 'upload' permission (on first chunk)

### 4. Helper Methods

Each controller has two helper methods:

```php
protected function checkPathACL(Request $request, string $path, string $permission): bool
{
    // Returns true if PathACL is disabled or permission is granted
    // Returns false if permission is denied
}

protected function forbidden(Response $response, string $message): Response
{
    // Returns 403 Forbidden response with error message
}
```

### 5. Configuration

**File**: `/mnt/ai/filegator/configuration.php`

PathACL service is registered:

```php
'Filegator\Services\PathACL\PathACLInterface' => [
    'handler' => '\Filegator\Services\PathACL\PathACL',
    'config' => [
        'acl_config_file' => __DIR__.'/private/acl_config.php',
    ],
],
```

## Permission Flow

When a file operation is requested:

1. **Global Permission Check** (existing)
   - Router checks user has required global permission
   - If not, returns 403 immediately

2. **PathACL Check** (new)
   - Controller calls `checkPathACL()`
   - PathACL service evaluates:
     a. User-level IP restrictions (from users.json)
     b. Path-based rules (from acl_config.php)
     c. Inheritance from parent folders
   - If PathACL is disabled, check passes automatically

3. **Operation Execution**
   - Both checks must pass
   - Operation proceeds normally

## User-Level IP Restrictions

Users can have IP restrictions defined in `private/users.json`:

```json
{
    "username": "john",
    "ip_allowlist": ["192.168.1.0/24", "10.8.0.0/24"],
    "ip_denylist": ["192.168.1.99"]
}
```

- `ip_allowlist` - If specified, user can only access from these IPs
- `ip_denylist` - User cannot access from these IPs (overrides allowlist)
- Empty arrays = no IP restrictions at user level

## Path-Based ACL Rules

Path rules are defined in `private/acl_config.php`:

```php
'/projects' => [
    'inherit' => true,
    'rules' => [
        [
            'users' => ['@developers'],
            'ip_allowlist' => ['192.168.1.0/24'],
            'ip_denylist' => [],
            'permissions' => ['read', 'write', 'upload'],
            'priority' => 60,
        ],
    ],
],
```

See `/mnt/ai/filegator/private/acl_config.php.example` for comprehensive examples.

## Permission Types

The following permissions are checked by PathACL:

- `read` - View directory listings and file previews
- `write` - Create, modify, delete, rename files/folders
- `upload` - Upload files
- `download` - Download individual files
- `batchdownload` - Download multiple files as zip
- `zip` - Create zip archives
- `chmod` - Change file permissions

## Enabling/Disabling PathACL

To enable PathACL:

```php
// private/acl_config.php
return [
    'enabled' => true,
    // ... rest of configuration
];
```

To disable PathACL (fall back to global permissions only):

```php
return [
    'enabled' => false,
];
```

Or remove the PathACL service from configuration.php entirely.

## Error Messages

When PathACL denies access, users receive:

```json
{
    "error": "Access denied: cannot [operation] [path/item]"
}
```

HTTP Status: 403 Forbidden

## Testing

To test the integration:

1. Enable PathACL in `private/acl_config.php`
2. Define test rules for specific paths
3. Add user-level IP restrictions in `private/users.json`
4. Access FileGator from different IPs
5. Verify operations are allowed/denied according to rules

## Debugging

To debug permission issues:

1. Check PathACL logs (if enabled)
2. Use `explainPermission()` method for detailed evaluation:

```php
$explanation = $pathacl->explainPermission($user, $clientIp, $path, $permission);
var_dump($explanation);
```

3. Verify:
   - User-level IP restrictions
   - Path rules and inheritance
   - Rule priorities and ordering
   - CIDR block calculations

## Security Considerations

1. **Fail-Safe**: PathACL defaults to deny when configuration errors occur
2. **Additive Security**: PathACL adds restrictions, doesn't remove global permissions
3. **IP Verification**: Uses Symfony's IpUtils for robust IP matching
4. **Proxy Support**: Configure trusted proxies for X-Forwarded-For validation
5. **Cache**: Permission results are cached (5 minutes default) for performance

## Migration from Global Permissions

Existing installations continue to work without changes:
- PathACL is optional (injected as null by default)
- When disabled, global permissions work as before
- Enable PathACL gradually by adding rules for specific paths

## Files Modified

1. `/mnt/ai/filegator/backend/Services/Auth/User.php`
2. `/mnt/ai/filegator/backend/Services/Auth/Adapters/JsonFile.php`
3. `/mnt/ai/filegator/backend/Services/PathACL/PathACL.php`
4. `/mnt/ai/filegator/backend/Controllers/FileController.php`
5. `/mnt/ai/filegator/backend/Controllers/DownloadController.php`
6. `/mnt/ai/filegator/backend/Controllers/UploadController.php`

## Additional Resources

- PathACL Design Document: `/docs/design/ip-folder-acl-design.md`
- PathACL Implementation: `/docs/design/ip-folder-acl-implementation.md`
- Example Configuration: `/private/acl_config.php.example`
- PathACL README: `/backend/Services/PathACL/README.md`
