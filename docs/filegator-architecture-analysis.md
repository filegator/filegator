# FileGator Architecture Analysis - IP-Based Folder Permissions Extension Points

## Executive Summary

This document provides a technical analysis of FileGator's authentication and permission architecture, focusing on potential extension points for implementing IP-based folder access control. The analysis reveals a well-structured, extensible system with clear separation of concerns that can accommodate IP-based restrictions.

## 1. Application Initialization Flow

### Entry Point: `/dist/index.php`
```php
// 1. Create global objects
Request::createFromGlobals()  // Symfony Request wrapper
Response()
Container()                    // Dependency injection container

// 2. Load configuration from configuration.php
$config = require __DIR__.'/../configuration.php';

// 3. Initialize App which:
//    - Registers all services from config['services']
//    - Calls init() on each service with service-specific config
//    - Services are lazy-loaded via Container
```

**Key Insight**: Services are registered and initialized from `configuration.php` in a defined order. This allows injection of new services or decorators without modifying core code.

---

## 2. Authentication Architecture

### 2.1 AuthInterface Contract

**Location**: `/mnt/ai/filegator/backend/Services/Auth/AuthInterface.php`

```php
interface AuthInterface {
    public function user(): ?User;                          // Get current user
    public function authenticate($username, $password): bool; // Login
    public function forget();                               // Logout
    public function find($username): ?User;                 // Find user by username
    public function store(User $user);                      // Store in session
    public function update($username, User $user, $password = ''): User;
    public function add(User $user, $password): User;
    public function delete(User $user);
    public function getGuest(): User;                       // Anonymous access
    public function allUsers(): UsersCollection;
}
```

### 2.2 Available Authentication Adapters

#### JsonFile Adapter (Default)
- **Location**: `/mnt/ai/filegator/backend/Services/Auth/Adapters/JsonFile.php`
- **Storage**: JSON file (`private/users.json`)
- **Session Keys**: `json_auth`, `json_auth_hash`
- **Hash Validation**: Password + permissions + homedir + role

#### Database Adapter
- **Location**: `/mnt/ai/filegator/backend/Services/Auth/Adapters/Database.php`
- **Storage**: SQL database via Dibi library
- **Session Keys**: `database_auth`, `database_auth_hash`
- **Schema**: `users` table with columns: username, name, role, homedir, permissions, password

#### LDAP Adapter
- **Location**: `/mnt/ai/filegator/backend/Services/Auth/Adapters/LDAP.php`
- **Storage**: External LDAP server
- **Session Keys**: `LDAP_auth`
- **Features**: Private repos per user, admin usernames mapping

#### WPAuth Adapter (WordPress Integration)
- **Location**: `/mnt/ai/filegator/backend/Services/Auth/Adapters/WPAuth.php`
- **Storage**: WordPress user system
- **Integration**: `wp_get_current_user()`, `wp_signon()`

### 2.3 Session Management

**Location**: `/mnt/ai/filegator/backend/Services/Session/Session.php`

```php
class Session extends SymfonySession {
    // Extends Symfony\Component\HttpFoundation\Session\Session
    // Configuration in configuration.php:
    //   - NativeSessionStorage with NativeFileSessionHandler
    //   - Cookie settings: SameSite=Lax, HttpOnly, Secure
}
```

**Session Stores**:
- Current user object (adapter-specific key)
- Session hash for validation (password+permissions+homedir+role)
- Current working directory (`FileController::SESSION_CWD`)

---

## 3. User Object & Permissions

### 3.1 User Class

**Location**: `/mnt/ai/filegator/backend/Services/Auth/User.php`

```php
class User implements \JsonSerializable {
    protected $role = 'guest';           // guest, user, admin
    protected $permissions = [];         // Array of permission strings
    protected $username = '';
    protected $homedir = '';             // Root folder for this user
    protected $name = '';

    protected $available_permissions = [
        'read', 'write', 'upload', 'download',
        'batchdownload', 'zip', 'chmod'
    ];
}
```

### 3.2 Permission Checking

**Method**: `User::hasPermissions($check): bool`

```php
// Supports single permission or array
if (is_array($check)) {
    // ALL permissions in $check must be present
    return count(array_intersect($check, $this->getPermissions())) == count($check);
}
// Single permission check
return in_array($check, $this->getPermissions());
```

**Usage in Routes**: Every route definition includes roles and permissions requirements.

### 3.3 Role-Based Access

**Method**: `User::hasRole($check): bool`

```php
// Supports single role or array
if (is_array($check)) {
    return in_array($this->getRole(), $check);
}
return $this->getRole() == $check;
```

**Roles**: `guest`, `user`, `admin`

---

## 4. Folder Access Control (Current Implementation)

### 4.1 Home Directory Mechanism

**FileController Constructor** (`/mnt/ai/filegator/backend/Controllers/FileController.php`):

```php
public function __construct(Config $config, Session $session,
                            AuthInterface $auth, Filesystem $storage) {
    $user = $this->auth->user() ?: $this->auth->getGuest();

    // Path prefix restricts all filesystem operations
    $this->storage->setPathPrefix($user->getHomeDir());
}
```

**Filesystem Path Prefix** (`/mnt/ai/filegator/backend/Services/Storage/Filesystem.php`):

```php
protected $path_prefix = '/';  // Set per user

private function applyPathPrefix(string $path): string {
    // All filesystem operations go through this
    // Prevents directory traversal (.., /.., etc.)
    return $this->joinPaths($this->getPathPrefix(), $path);
}
```

**Key Operations Protected**:
- `createDir()`, `createFile()`
- `copyFile()`, `copyDir()`
- `deleteDir()`, `deleteFile()`
- `readStream()`, `move()`, `rename()`
- `getDirectoryCollection()` - Lists files

### 4.2 Per-User Directory Isolation

**LDAP Adapter Example**:
```php
if ($this->private_repos) {
    $user['homedir'] = '/' . $user['username'];  // Each user gets /username/
}
if ($user['role'] == 'admin') {
    $user['homedir'] = '/';  // Admins see everything
}
```

---

## 5. Request Handling & IP Access

### 5.1 Request Object

**Location**: `/mnt/ai/filegator/backend/Kernel/Request.php`

```php
class Request extends SymfonyRequest {
    // Inherits getClientIp() from Symfony Request
    // Supports X-Forwarded-For headers for proxy environments

    public function input($key, $default = null);
    public function all();
}
```

**IP Retrieval**: `$request->getClientIp()`
- Handles proxy headers automatically
- Respects `X-Forwarded-For`, `X-Real-IP`

### 5.2 Current IP-Based Security

**Location**: `/mnt/ai/filegator/backend/Services/Security/Security.php`

```php
public function init(array $config = []) {
    // IP Allowlist (whitelist)
    if (!empty($config['ip_allowlist'])) {
        foreach ($config['ip_allowlist'] as $ip) {
            if ($this->request->getClientIp() == $ip) {
                $pass = true;
            }
        }
        if (!$pass) {
            $this->response->setStatusCode(403);
            die;  // Application-wide IP block
        }
    }

    // IP Denylist (blacklist)
    if (!empty($config['ip_denylist'])) {
        foreach ($config['ip_denylist'] as $ip) {
            if ($this->request->getClientIp() == $ip) {
                $this->response->setStatusCode(403);
                die;  // Application-wide IP block
            }
        }
    }
}
```

**Configuration** (`configuration.php`):
```php
'Filegator\Services\Security\Security' => [
    'handler' => '\Filegator\Services\Security\Security',
    'config' => [
        'ip_allowlist' => [],  // Empty = allow all
        'ip_denylist' => [],   // Block specific IPs globally
    ],
]
```

**Limitation**: Current IP checks are **application-wide**, not per-folder or per-user.

### 5.3 Login Lockout (IP-Based)

**Location**: `/mnt/ai/filegator/backend/Controllers/AuthController.php`

```php
public function login(Request $request, Response $response,
                      AuthInterface $auth, TmpfsInterface $tmpfs, Config $config) {
    $ip = $request->getClientIp();
    $lockfile = md5($ip).'.lock';

    // Track failed login attempts per IP
    if ($tmpfs->exists($lockfile) &&
        strlen($tmpfs->read($lockfile)) >= $lockout_attempts) {
        $this->logger->log("Too many login attempts from IP ".$ip);
        return $response->json('Not Allowed', 429);
    }

    if (!$auth->authenticate($username, $password)) {
        $tmpfs->write($lockfile, 'x', true);  // Append 'x' per failure
    }
}
```

---

## 6. Routing & Permission Enforcement

### 6.1 Route Definition

**Location**: `/mnt/ai/filegator/backend/Controllers/routes.php`

```php
return [
    [
        'route' => ['GET', '/getdir', 'FileController@getDirectory'],
        'roles' => ['guest', 'user', 'admin'],
        'permissions' => ['read'],  // Required permissions
    ],
    [
        'route' => ['POST', '/deleteitems', 'FileController@deleteItems'],
        'roles' => ['guest', 'user', 'admin'],
        'permissions' => ['read', 'write'],  // Multiple required
    ],
    // ... 30+ routes
];
```

### 6.2 Router Permission Check

**Location**: `/mnt/ai/filegator/backend/Services/Router/Router.php`

```php
public function init(array $config = []) {
    $this->user = $auth->user() ?: $auth->getGuest();

    $dispatcher = FastRoute\simpleDispatcher(function($r) use ($routes) {
        foreach ($routes as $params) {
            // Only register route if user has role AND permissions
            if ($this->user->hasRole($params['roles']) &&
                $this->user->hasPermissions($params['permissions'])) {
                $r->addRoute($params['route'][0],
                            $params['route'][1],
                            $params['route'][2]);
            }
        }
    });

    // Route is only available if user qualifies
}
```

**Security Model**: Routes don't exist for users without proper permissions (fail closed).

---

## 7. Extension Points for IP-Based Folder Permissions

### 7.1 **RECOMMENDED APPROACH: Custom Auth Adapter Decorator**

**Strategy**: Wrap existing auth adapter and modify User object based on IP.

```php
// backend/Services/Auth/Adapters/IpBasedAccessDecorator.php
class IpBasedAccessDecorator implements AuthInterface {
    private $wrapped;
    private $request;
    private $ipRules;  // IP -> folder restrictions map

    public function user(): ?User {
        $user = $this->wrapped->user();
        if ($user) {
            $ip = $this->request->getClientIp();
            $user = $this->applyIpRestrictions($user, $ip);
        }
        return $user;
    }

    private function applyIpRestrictions(User $user, string $ip): User {
        // Modify user->homedir based on IP rules
        // Or: Store IP-specific folder restrictions in user object
        return $user;
    }

    // Delegate all other methods to $this->wrapped
}
```

**Configuration**:
```php
'Filegator\Services\Auth\AuthInterface' => [
    'handler' => '\Filegator\Services\Auth\Adapters\IpBasedAccessDecorator',
    'config' => [
        'wrapped_adapter' => '\Filegator\Services\Auth\Adapters\JsonFile',
        'wrapped_config' => ['file' => __DIR__.'/private/users.json'],
        'ip_rules' => [
            '192.168.1.0/24' => ['allowed_folders' => ['/public', '/shared']],
            '10.0.0.50' => ['allowed_folders' => ['/admin']],
        ],
    ],
]
```

**Pros**:
- Clean separation of concerns
- Works with any existing auth adapter
- No changes to core User class
- Configuration-driven

**Cons**:
- Need to extend User class to store IP-specific folder list
- Session stores user state, so changing IP requires re-authentication or session invalidation

---

### 7.2 **ALTERNATIVE: Custom Filesystem Decorator**

**Strategy**: Wrap Filesystem service and filter operations based on IP.

```php
// backend/Services/Storage/IpBasedFilesystem.php
class IpBasedFilesystem extends Filesystem {
    private $request;
    private $ipFolderRules;

    public function getDirectoryCollection(string $path, bool $recursive = false) {
        $ip = $this->request->getClientIp();

        if (!$this->isPathAllowedForIp($path, $ip)) {
            throw new \Exception('Access denied from your IP address');
        }

        return parent::getDirectoryCollection($path, $recursive);
    }

    // Override all critical methods: createDir, deleteFile, etc.
}
```

**Pros**:
- Centralized enforcement point
- Works independently of auth system
- Can restrict access to specific folders dynamically

**Cons**:
- Need to override many methods
- Duplicate logic across all file operations
- Hard to compose with existing path prefix logic

---

### 7.3 **ALTERNATIVE: Middleware/Hook Approach**

**Strategy**: Add a service that runs before FileController operations.

```php
// backend/Services/IpFolderAccess/IpFolderAccess.php
class IpFolderAccess implements Service {
    public function init(array $config = []) {
        // Register event listeners or hooks
    }

    public function checkAccess(User $user, string $path, string $ip): bool {
        // Validate IP can access path
        // Throw exception if denied
    }
}
```

**Integration**: Modify FileController to call this service before operations.

**Pros**:
- Minimal changes to existing code
- Can be enabled/disabled via configuration
- Clear audit trail

**Cons**:
- Requires modifying controllers
- Less elegant than decorator pattern

---

### 7.4 **ALTERNATIVE: Extended User Object with IP Context**

**Strategy**: Add IP-based restrictions directly to User class.

```php
// backend/Services/Auth/IpAwareUser.php
class IpAwareUser extends User {
    protected $ip_restrictions = [];  // IP -> folders mapping
    protected $current_ip = '';

    public function setIpRestrictions(array $restrictions);
    public function setCurrentIp(string $ip);

    public function canAccessFolder(string $path): bool {
        // Check if current IP can access path
        // Based on ip_restrictions rules
    }
}
```

**Modify AuthInterface adapters** to return IpAwareUser instead of User.

**Pros**:
- Natural extension of existing permission model
- Permissions and IP rules in one place

**Cons**:
- User object becomes IP-aware (mixing concerns)
- Session serialization may need adjustment
- All auth adapters need updates

---

## 8. Database Schema Considerations

### Current User Storage (JsonFile)
```json
[
  {
    "username": "admin",
    "name": "Admin",
    "role": "admin",
    "homedir": "/",
    "permissions": "read|write|upload|download|batchdownload|zip|chmod",
    "password": "$2y$10$hash..."
  }
]
```

### Proposed Extension for IP Rules (Database Adapter)
```sql
-- Option 1: Add to users table
ALTER TABLE users ADD COLUMN ip_folder_rules TEXT;  -- JSON encoded

-- Option 2: Separate table
CREATE TABLE ip_folder_rules (
    id INT PRIMARY KEY AUTO_INCREMENT,
    username VARCHAR(255),
    ip_address VARCHAR(45),     -- Supports IPv6
    ip_cidr VARCHAR(50),        -- For subnet masks
    allowed_folders TEXT,       -- JSON array of paths
    denied_folders TEXT,        -- JSON array of paths
    priority INT DEFAULT 0,     -- Rule precedence
    created_at TIMESTAMP,
    FOREIGN KEY (username) REFERENCES users(username)
);
```

---

## 9. Configuration Extension Example

### Current Configuration Structure
```php
// configuration.php
return [
    'services' => [
        'Filegator\Services\Auth\AuthInterface' => [
            'handler' => '\Filegator\Services\Auth\Adapters\JsonFile',
            'config' => ['file' => __DIR__.'/private/users.json'],
        ],
    ],
];
```

### Proposed IP-Based Rules Configuration
```php
return [
    'services' => [
        // ... existing services ...

        // New service for IP-based folder access
        'Filegator\Services\IpFolderAccess\IpFolderAccessInterface' => [
            'handler' => '\Filegator\Services\IpFolderAccess\IpFolderAccess',
            'config' => [
                'enabled' => true,
                'rules' => [
                    // CIDR notation support
                    '192.168.1.0/24' => [
                        'allowed_folders' => ['/public', '/shared'],
                        'denied_folders' => [],
                    ],
                    // Specific IP
                    '10.0.0.50' => [
                        'allowed_folders' => ['/admin', '/reports'],
                        'denied_folders' => [],
                    ],
                    // Deny specific path for IP range
                    '203.0.113.0/24' => [
                        'allowed_folders' => [],
                        'denied_folders' => ['/private', '/admin'],
                    ],
                ],
                // Global default
                'default_policy' => 'allow',  // or 'deny'

                // Per-user overrides
                'user_overrides' => [
                    'admin' => [
                        'bypass_ip_restrictions' => true,
                    ],
                ],

                // Trusted proxy IPs for X-Forwarded-For
                'trusted_proxies' => ['127.0.0.1'],
            ],
        ],
    ],
];
```

---

## 10. Implementation Roadmap

### Phase 1: Foundation
1. Create `IpFolderAccessInterface` service interface
2. Implement basic IP CIDR matching utility class
3. Add configuration schema for IP rules
4. Unit tests for IP matching logic

### Phase 2: Integration
5. Create `IpBasedAccessDecorator` for AuthInterface
6. Modify User class or create `IpAwareUser` extension
7. Integrate IP checks into FileController (decorator or direct)
8. Add logging for IP-based access denials

### Phase 3: Storage
9. Extend database schema for ip_folder_rules table
10. Create admin UI for managing IP rules (optional)
11. Add import/export for IP rules configuration
12. Migration scripts for existing installations

### Phase 4: Testing & Documentation
13. Integration tests for various IP scenarios
14. Performance testing with large rule sets
15. Security audit (bypass attempts, edge cases)
16. User documentation and configuration examples

---

## 11. Key Takeaways

### Strengths of Current Architecture
1. **Clean separation**: Auth, Storage, Routing are independent services
2. **Extensible**: Decorator pattern works well with AuthInterface
3. **Secure by default**: Path traversal prevention, permission checks at router level
4. **Multiple auth adapters**: JsonFile, Database, LDAP, WPAuth all compatible

### Challenges for IP-Based Folders
1. **Session state**: User object stored in session lacks IP context
2. **Dynamic IP changes**: User's IP may change mid-session (mobile, VPN)
3. **Performance**: IP checks on every file operation need optimization
4. **Rule complexity**: CIDR, subnets, wildcards add complexity

### Recommended Solution
**Use Decorator Pattern on AuthInterface**:
- Wraps existing auth adapter
- Injects IP-specific restrictions into User object at login
- Minimal changes to core codebase
- Configuration-driven rules
- Works with all existing auth adapters

### Critical Files for Implementation
1. `/mnt/ai/filegator/backend/Services/Auth/User.php` - Extend with IP context
2. `/mnt/ai/filegator/backend/Controllers/FileController.php` - Add IP checks
3. `/mnt/ai/filegator/backend/Services/Storage/Filesystem.php` - Path validation
4. `/mnt/ai/filegator/configuration.php` - Add IP rules configuration
5. `/mnt/ai/filegator/backend/Services/Auth/Adapters/JsonFile.php` - Reference implementation

---

## 12. Security Considerations

### IP Spoofing Protection
- Validate `X-Forwarded-For` headers
- Configure trusted proxy IPs
- Log actual client IP vs. reported IP

### Performance
- Cache IP rule lookups
- Use Trie data structure for CIDR matching
- Avoid repeated rule evaluation per request

### Logging & Auditing
- Log all IP-based access denials
- Include username, IP, attempted path, timestamp
- Support log rotation and external SIEM integration

### Fail-Safe Behavior
- If IP rules fail to load, default to deny or allow based on config
- Admin users should have bypass option
- Emergency override mechanism via configuration

---

## Appendix A: Code Locations Summary

| Component | File Path |
|-----------|-----------|
| AuthInterface | `/backend/Services/Auth/AuthInterface.php` |
| User class | `/backend/Services/Auth/User.php` |
| JsonFile adapter | `/backend/Services/Auth/Adapters/JsonFile.php` |
| Database adapter | `/backend/Services/Auth/Adapters/Database.php` |
| LDAP adapter | `/backend/Services/Auth/Adapters/LDAP.php` |
| FileController | `/backend/Controllers/FileController.php` |
| Filesystem service | `/backend/Services/Storage/Filesystem.php` |
| Router | `/backend/Services/Router/Router.php` |
| Routes definition | `/backend/Controllers/routes.php` |
| Security service | `/backend/Services/Security/Security.php` |
| AuthController | `/backend/Controllers/AuthController.php` |
| Request wrapper | `/backend/Kernel/Request.php` |
| Session wrapper | `/backend/Services/Session/Session.php` |
| App bootstrap | `/backend/App.php` |
| Entry point | `/dist/index.php` |
| Configuration | `/configuration.php` |

---

## Appendix B: Permission Flow Diagram

```
Request → Security Service (IP allowlist/denylist check)
    ↓
Router → Load routes
    ↓
Router → Get current user from AuthInterface
    ↓
Router → Filter routes by user->hasRole() && user->hasPermissions()
    ↓
Router → Dispatch to controller (or 404 if no matching route)
    ↓
FileController → Get user from AuthInterface
    ↓
FileController → Set storage path prefix to user->getHomeDir()
    ↓
FileController → Perform operation (all paths go through applyPathPrefix)
    ↓
Filesystem → applyPathPrefix() prevents directory traversal
    ↓
Filesystem → Perform actual file operation via Flysystem
```

**Proposed IP Check Insertion Point**:
- After Router gets user, before FileController operations
- OR: In AuthInterface decorator when returning User object
- OR: In Filesystem before applyPathPrefix()

---

## Appendix C: Sample IP Rules Configuration

```php
'ip_folder_rules' => [
    // Internal network - full access
    '192.168.0.0/16' => [
        'allowed_folders' => ['/'],
        'denied_folders' => [],
    ],

    // VPN users - limited access
    '10.8.0.0/24' => [
        'allowed_folders' => ['/public', '/shared', '/team-docs'],
        'denied_folders' => ['/admin', '/private', '/confidential'],
    ],

    // Public internet - guest access only
    '0.0.0.0/0' => [
        'allowed_folders' => ['/public'],
        'denied_folders' => [],
    ],

    // Specific IP - admin access
    '203.0.113.42' => [
        'allowed_folders' => ['/'],
        'denied_folders' => [],
        'priority' => 100,  // Higher priority than default rules
    ],
],

// Rule matching algorithm:
// 1. Match specific IP first (203.0.113.42)
// 2. Match smallest CIDR that contains IP (/24 before /16)
// 3. Fall back to default rule (0.0.0.0/0)
// 4. If no match, use global default_policy
```

---

**Document Version**: 1.0
**Date**: 2025-12-09
**FileGator Version Analyzed**: 7.13.0
**Analysis Focus**: IP-based folder permission extension points
