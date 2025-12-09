# Path ACL Configuration Examples

This document provides practical examples for common Path-Based ACL scenarios.

## Table of Contents

- [Basic Examples](#basic-examples)
  - [Office Network Only](#office-network-only)
  - [VPN Access Control](#vpn-access-control)
  - [Public Upload Folder](#public-upload-folder)
- [Advanced Examples](#advanced-examples)
  - [Department Folder Isolation](#department-folder-isolation)
  - [One User, Multiple Permissions](#one-user-multiple-permissions)
  - [Contractor Limited Access](#contractor-limited-access)
- [Enterprise Scenarios](#enterprise-scenarios)
  - [Office vs VPN vs Remote](#office-vs-vpn-vs-remote)
  - [IP Exclusions](#ip-exclusions)
  - [Time-Sensitive Projects](#time-sensitive-projects)

## Basic Examples

### Office Network Only

Restrict all file access to users connecting from the office network.

```php
<?php
return [
    'enabled' => true,

    'settings' => [
        'evaluation_mode' => 'most_specific_wins',
        'cache_enabled' => true,
    ],

    'groups' => [],

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
    ],
];
```

**Result**: All authenticated users can access files, but only when connecting from the office network (203.0.113.0/24).

---

### VPN Access Control

Allow VPN users read-only access, office users full access.

```php
<?php
return [
    'enabled' => true,

    'settings' => [
        'evaluation_mode' => 'most_specific_wins',
        'deny_overrides_allow' => true,
    ],

    'groups' => [],

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
    ],
];
```

**Result**:
- From VPN (10.8.0.0/24): Read and download only
- From office (192.168.1.0/24): Full access

---

### Public Upload Folder

Allow anyone to upload files, but only admins can view/download them.

```php
<?php
return [
    'enabled' => true,

    'settings' => [
        'evaluation_mode' => 'most_specific_wins',
    ],

    'groups' => [
        'admins' => ['admin', 'supervisor'],
    ],

    'path_rules' => [
        '/' => [
            'rules' => [
                [
                    'users' => ['@admins'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete', 'zip'],
                    'priority' => 100,
                ],
            ],
        ],

        '/uploads' => [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['*'],  // All users
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['upload'],  // Upload only, no read
                    'priority' => 30,
                    'override_inherited' => false,  // Merge with admin permissions
                ],
            ],
        ],
    ],
];
```

**Result**:
- Regular users: Can upload to `/uploads` but cannot see or download files
- Admins: Full access including viewing uploaded files

---

## Advanced Examples

### Department Folder Isolation

Each department has a folder accessible only from their network subnet.

```php
<?php
return [
    'enabled' => true,

    'settings' => [
        'evaluation_mode' => 'most_specific_wins',
        'deny_overrides_allow' => true,
    ],

    'groups' => [
        'hr-staff' => ['susan', 'tom', 'mary'],
        'developers' => ['john', 'jane', 'bob'],
        'sales-team' => ['alice', 'charlie', 'david'],
    ],

    'path_rules' => [
        // Root: minimal access
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

        // HR department folder
        '/departments/hr' => [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['@hr-staff'],
                    'ip_allowlist' => ['192.168.10.0/24'],  // HR subnet
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 60,
                    'override_inherited' => true,
                ],
            ],
        ],

        // Engineering department folder
        '/departments/engineering' => [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['@developers'],
                    'ip_allowlist' => ['192.168.20.0/24'],  // Engineering subnet
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 60,
                    'override_inherited' => true,
                ],
            ],
        ],

        // Sales department folder
        '/departments/sales' => [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['@sales-team'],
                    'ip_allowlist' => ['192.168.30.0/24'],  // Sales subnet
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 60,
                    'override_inherited' => true,
                ],
            ],
        ],
    ],
];
```

**Result**: Each department can only access their folder when connected from their network subnet.

---

### One User, Multiple Permissions

User "john" has different permissions based on location and folder.

```php
<?php
return [
    'enabled' => true,

    'settings' => [
        'evaluation_mode' => 'most_specific_wins',
        'deny_overrides_allow' => true,
        'cache_enabled' => true,
    ],

    'groups' => [],

    'path_rules' => [
        // Root: basic read access from anywhere
        '/' => [
            'inherit' => false,
            'rules' => [
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read'],
                    'priority' => 10,
                ],
            ],
        ],

        // John's private folder: full access from any IP
        '/john-private' => [
            'inherit' => false,
            'rules' => [
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete', 'zip'],
                    'priority' => 100,
                ],
            ],
        ],

        // Projects folder: access varies by IP
        '/projects' => [
            'inherit' => true,
            'rules' => [
                // From office: full access
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['192.168.1.0/24'],  // Office
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 80,
                    'override_inherited' => true,
                ],
                // From VPN: read-only
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['10.8.0.0/24'],  // VPN
                    'ip_denylist' => [],
                    'permissions' => ['read', 'download'],
                    'priority' => 70,
                    'override_inherited' => true,
                ],
            ],
        ],

        // Restricted folder: no access
        '/restricted' => [
            'inherit' => false,
            'rules' => [
                // John is explicitly excluded
                [
                    'users' => ['admin'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'delete'],
                    'priority' => 100,
                ],
            ],
        ],

        // Uploads folder: upload-only from external IPs, no access from office/VPN
        '/uploads' => [
            'inherit' => false,  // Don't inherit - explicit rules only
            'rules' => [
                // From external IPs only: upload only (no read/download)
                [
                    'users' => ['john'],
                    'ip_allowlist' => ['0.0.0.0/0'],  // Any IP
                    'ip_denylist' => ['192.168.1.0/24', '10.8.0.0/24'],  // Exclude office/VPN
                    'permissions' => ['upload'],
                    'priority' => 50,
                    'override_inherited' => true,
                ],
                // No rule for office/VPN = no access
            ],
        ],
    ],
];
```

**Result for user "john"**:

| Path | From Office (192.168.1.x) | From VPN (10.8.0.x) | From External IP |
|------|---------------------------|---------------------|------------------|
| `/` | Read | Read | Read |
| `/john-private` | Full access | Full access | Full access |
| `/projects` | Read, write, upload, download, delete | Read, download | Read only |
| `/restricted` | No access | No access | No access |
| `/uploads` | No access | No access | Upload only |

---

### Contractor Limited Access

Contractors have limited access from VPN only, to specific project folders.

```php
<?php
return [
    'enabled' => true,

    'settings' => [
        'evaluation_mode' => 'most_specific_wins',
        'cache_enabled' => true,
    ],

    'groups' => [
        'contractors' => ['alice', 'charlie', 'david'],
        'employees' => ['john', 'jane', 'bob'],
    ],

    'path_rules' => [
        '/' => [
            'rules' => [
                [
                    'users' => ['@employees'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 50,
                ],
            ],
        ],

        '/projects/client-alpha' => [
            'inherit' => false,
            'rules' => [
                // Contractors: VPN only, read-only
                [
                    'users' => ['@contractors'],
                    'ip_allowlist' => ['10.8.0.0/24'],  // VPN only
                    'ip_denylist' => [],
                    'permissions' => ['read', 'download'],
                    'priority' => 60,
                ],
                // Employees: full access from anywhere
                [
                    'users' => ['@employees'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 70,
                ],
            ],
        ],

        '/projects/client-beta' => [
            'inherit' => false,
            'rules' => [
                // Only employees, no contractor access
                [
                    'users' => ['@employees'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 70,
                ],
            ],
        ],
    ],
];
```

**Result**:
- Contractors can only access `/projects/client-alpha` from VPN with read-only permissions
- Employees have full access to all projects from any IP

---

## Enterprise Scenarios

### Office vs VPN vs Remote

Different permission levels based on connection source.

```php
<?php
return [
    'enabled' => true,

    'settings' => [
        'evaluation_mode' => 'most_specific_wins',
        'deny_overrides_allow' => true,
        'trusted_proxies' => ['127.0.0.1', '10.0.0.1'],
    ],

    'groups' => [
        'staff' => ['john', 'jane', 'bob', 'alice'],
    ],

    'path_rules' => [
        '/' => [
            'inherit' => false,
            'rules' => [
                // From office: full access
                [
                    'users' => ['@staff'],
                    'ip_allowlist' => ['192.168.0.0/16'],  // Office networks
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete', 'zip', 'chmod'],
                    'priority' => 80,
                ],
                // From VPN: read-write, no delete
                [
                    'users' => ['@staff'],
                    'ip_allowlist' => ['10.8.0.0/24'],  // VPN network
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download'],
                    'priority' => 60,
                ],
                // From internet: read-only
                [
                    'users' => ['@staff'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => ['192.168.0.0/16', '10.8.0.0/24'],  // Exclude office/VPN
                    'permissions' => ['read', 'download'],
                    'priority' => 40,
                ],
            ],
        ],

        '/confidential' => [
            'inherit' => false,
            'rules' => [
                // Office only, no VPN or remote access
                [
                    'users' => ['@staff'],
                    'ip_allowlist' => ['192.168.0.0/16'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 100,
                ],
            ],
        ],
    ],
];
```

**Result**:

| Location | Root Access | Confidential Access |
|----------|-------------|---------------------|
| Office (192.168.x.x) | Full access | Full access |
| VPN (10.8.0.x) | Read-write (no delete) | No access |
| Remote (other IPs) | Read-only | No access |

---

### IP Exclusions

Allow a subnet but exclude specific IPs.

```php
<?php
return [
    'enabled' => true,

    'settings' => [
        'deny_overrides_allow' => true,
    ],

    'groups' => [],

    'path_rules' => [
        '/' => [
            'rules' => [
                [
                    'users' => ['*'],
                    'ip_allowlist' => ['192.168.1.0/24'],  // Entire office subnet
                    'ip_denylist' => [
                        '192.168.1.50',   // Specific blocked device
                        '192.168.1.99',   // Another blocked device
                    ],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 50,
                ],
            ],
        ],

        '/sensitive' => [
            'inherit' => false,
            'rules' => [
                [
                    'users' => ['admin', 'manager'],
                    'ip_allowlist' => ['192.168.1.0/24'],
                    'ip_denylist' => [
                        '192.168.1.50',
                        '192.168.1.99',
                        '192.168.1.100',  // Additional exclusion for sensitive folder
                    ],
                    'permissions' => ['read', 'write', 'upload', 'download'],
                    'priority' => 80,
                ],
            ],
        ],
    ],
];
```

**Result**:
- 192.168.1.1-49, 51-98, 100-255: Allowed
- 192.168.1.50, 99: Denied everywhere
- 192.168.1.100: Denied only for `/sensitive`

---

### Time-Sensitive Projects

Different access for ongoing vs completed projects.

```php
<?php
return [
    'enabled' => true,

    'settings' => [
        'evaluation_mode' => 'most_specific_wins',
    ],

    'groups' => [
        'project-team' => ['john', 'jane', 'bob'],
        'archives-team' => ['archivist', 'admin'],
    ],

    'path_rules' => [
        '/' => [
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

        // Active project: team has full access
        '/projects/2025/active-project' => [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['@project-team'],
                    'ip_allowlist' => ['192.168.1.0/24', '10.8.0.0/24'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 70,
                    'override_inherited' => true,
                ],
            ],
        ],

        // Completed projects: read-only for team, archives team can manage
        '/projects/2024' => [
            'inherit' => true,
            'rules' => [
                // Project team: read-only
                [
                    'users' => ['@project-team'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'download'],
                    'priority' => 50,
                    'override_inherited' => true,
                ],
                // Archives team: full management
                [
                    'users' => ['@archives-team'],
                    'ip_allowlist' => ['192.168.1.0/24'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete', 'zip'],
                    'priority' => 80,
                    'override_inherited' => true,
                ],
            ],
        ],
    ],
];
```

**Result**:
- Active projects: Full team access from office/VPN
- Completed projects: Team can only read, archives team manages

---

## Testing Configurations

Before deploying ACL changes, test with the explain method:

```php
// In a test script or debug hook
$user = $auth->find('john');
$ip = '192.168.1.50';
$path = '/projects/file.txt';
$permission = 'write';

$result = $pathAcl->explainPermission($user, $ip, $path, $permission);
print_r($result);

// Output shows:
// - Whether access is allowed
// - Which rules matched
// - Effective permissions
// - Reason for decision
```

## Common Patterns

### Pattern: Multi-Tier Access

```php
// Public tier: Everyone
// User tier: Authenticated users
// Team tier: Specific teams
// Admin tier: Administrators

'/' => [
    'rules' => [
        ['users' => ['*'], 'permissions' => ['read'], 'priority' => 10],
        ['users' => ['@authenticated'], 'permissions' => ['read', 'download'], 'priority' => 20],
        ['users' => ['@team'], 'permissions' => ['read', 'write', 'upload', 'download'], 'priority' => 30],
        ['users' => ['@admins'], 'permissions' => ['read', 'write', 'upload', 'download', 'delete', 'chmod'], 'priority' => 100],
    ]
]
```

### Pattern: Geo-Restriction

```php
// Different regions have different access
'/' => [
    'rules' => [
        ['users' => ['*'], 'ip_allowlist' => ['203.0.113.0/24'], 'permissions' => ['read', 'write'], 'priority' => 50],  // US office
        ['users' => ['*'], 'ip_allowlist' => ['198.51.100.0/24'], 'permissions' => ['read', 'write'], 'priority' => 50],  // EU office
        ['users' => ['*'], 'ip_allowlist' => ['192.0.2.0/24'], 'permissions' => ['read'], 'priority' => 40],  // APAC office (read-only)
    ]
]
```

### Pattern: Upload Quarantine

```php
'/uploads/quarantine' => [
    'inherit' => false,
    'rules' => [
        // Users can upload but not see files
        ['users' => ['*'], 'permissions' => ['upload'], 'priority' => 30],
        // Admin can review
        ['users' => ['@admins'], 'permissions' => ['read', 'download', 'delete'], 'priority' => 100],
    ]
]
```

See [Troubleshooting](./troubleshooting.md) for debugging ACL configurations.
