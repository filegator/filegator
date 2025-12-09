# Path-Based Access Control List (ACL) Systems Research

**Research Date:** 2025-12-09
**Project:** FileGator - Path-Based Permissions Enhancement
**Objective:** Design an intuitive three-dimensional ACL system (User + IP + Path → Permissions)

---

## Executive Summary

This research analyzes industry-standard ACL systems to design a path-based permission model for FileGator that supports:
- **Cascading inheritance** from parent to child folders
- **Explicit override** capability at any level
- **Multi-dimensional access control** combining User + IP Address + Path
- **Intuitive configuration** for PHP applications

### Key Findings

1. **AWS S3** uses hierarchical ARN patterns with wildcards and specificity-based evaluation
2. **NFS** employs IP-based restrictions with first-match rule processing
3. **Samba/CIFS** implements host-based allow/deny with priority rules (deny overrides allow)
4. **POSIX ACLs** use default ACLs for inheritance at file creation time only (not dynamic)
5. **Azure Blob** provides attribute-based conditions with path expressions
6. **Industry Best Practices** favor RBAC with inheritance, least privilege, and explicit overrides

---

## 1. AWS S3 Bucket Policies

### Permission Model

AWS S3 uses **Resource ARN patterns** with **policy-based access control** (PBAC).

#### Key Features

- **Wildcard Support**: `*` matches any characters including path delimiters, `?` matches single character
- **Path-Based Resources**: `arn:aws:s3:::bucket-name/folder/*/subfolder/*`
- **Condition Expressions**: StringLike, IpAddress, DateGreaterThan, etc.
- **Policy Evaluation**: Most specific resource match wins

#### Configuration Format

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowUserAccessToFolder",
      "Effect": "Allow",
      "Principal": {"AWS": "arn:aws:iam::account:user/username"},
      "Action": ["s3:GetObject", "s3:PutObject"],
      "Resource": "arn:aws:s3:::bucket-name/users/${aws:username}/*",
      "Condition": {
        "IpAddress": {"aws:SourceIp": ["192.168.1.0/24", "10.0.0.0/16"]},
        "StringLike": {"s3:prefix": ["documents/*", "images/*"]}
      }
    }
  ]
}
```

#### Inheritance Model

- **Explicit Only**: No automatic inheritance; permissions must be explicitly defined
- **Wildcard Cascading**: `folder/*` implicitly covers all subfolders (`folder/a/b/c`)
- **Override Pattern**: More specific paths override broader wildcards
- **Priority**: Deny always overrides Allow when both match

#### Evaluation Algorithm

1. Start with implicit Deny
2. Evaluate all matching policies
3. If any policy explicitly Denies → access Denied
4. If any policy Allows and no Deny → access Allowed
5. Otherwise → access Denied (default)

#### Best Practices

- Never wildcard the account ID segment (security risk)
- Use policy variables for user-specific paths (`${aws:username}`)
- Combine multiple wildcards carefully (can match unintended paths)
- Use StringLike operator for wildcard matching in conditions

**Sources:**
- [IAM JSON policy elements: Resource](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_resource.html)
- [Examples of Amazon S3 bucket policies](https://docs.aws.amazon.com/AmazonS3/latest/userguide/example-bucket-policies.html)
- [AWS IAM Policies: Multiple Wildcards Explained](https://www.codestudy.net/blog/aws-iam-can-you-use-multiple-wildcards-in-a-value/)

---

## 2. NFS Exports

### Permission Model

NFS uses **IP-based host access control** with **path export mappings**.

#### Key Features

- **IP Specification Methods**:
  - Single IP: `192.168.1.10`
  - CIDR notation: `192.168.1.0/24`
  - Hostname: `server.example.com`
  - Wildcards: `*.example.com`
  - Netgroups: `@trusted-hosts`
- **Export Options**: ro (read-only), rw (read-write), no_root_squash, sync/async
- **First-Match Rule**: Only the first matching export option is applied

#### Configuration Format

```bash
# /etc/exports
/srv/files          192.168.1.0/24(rw,sync,no_subtree_check)
/srv/files/public   *(ro)
/srv/files/admin    192.168.1.10(rw) 192.168.1.11(rw)
/srv/files/project  @developers(rw,no_root_squash) *(ro)
```

#### Critical Syntax Rules

- **NO SPACE** between host and options: `host(options)` not `host (options)`
- Space creates default options for host and applies options to all others (security risk)

#### Inheritance Model

- **No Automatic Inheritance**: Each path must be explicitly exported
- **Processing Order**: First matching entry wins; subsequent entries ignored
- **Default Permissions**: ro (read-only), root_squash enabled
- **Override Mechanism**: Must explicitly export child paths to override parent

#### Permission Algorithm

1. Client requests access to path
2. NFS scans exports list in order
3. First matching Source IP + Path combination applies
4. All other matches are ignored
5. No match → access denied

#### UID/GID Matching

- User permissions based on numeric UID matching between client and server
- UIDs must match for proper file access
- root_squash maps root (UID 0) to anonymous user for security

**Sources:**
- [The /etc/exports Configuration File - Red Hat](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/5/html/deployment_guide/s1-nfs-server-config-exports)
- [NFS exports options examples - GoLinuxCloud](https://www.golinuxcloud.com/nfs-exports-options-examples/)
- [Understanding the /etc/exports File](https://www.thegeekdiary.com/understanding-the-etc-exports-file/)

---

## 3. Samba/CIFS Shares

### Permission Model

Samba uses **host-based access control** combined with **user-based permissions** and **path definitions**.

#### Key Features

- **Host Allow/Deny Lists**: Control access by IP, hostname, or network
- **Priority Rule**: `hosts deny` has higher priority than `hosts allow`
- **User Validation**: `valid users` and `invalid users` parameters
- **Path-Based Shares**: Each share maps to a specific directory path

#### Configuration Format

```ini
# smb.conf
[global]
    hosts allow = 127.0.0.1 192.168.1.0/24
    hosts deny = ALL

[sharename]
    path = /srv/samba/folder
    hosts allow = 192.168.1.0/24
    hosts deny = 192.168.1.50
    valid users = john @developers
    invalid users = guest
    read only = no
    browseable = yes
    inherit permissions = yes
    inherit acls = yes
```

#### Inheritance Model

- **Global to Share**: Global settings apply to all shares unless overridden
- **POSIX ACL Inheritance**: When enabled, inherits filesystem ACLs
- **Permission Inheritance**: `inherit permissions = yes` copies parent folder permissions
- **ACL Inheritance**: `inherit acls = yes` uses extended attributes for Windows-style ACLs

#### Evaluation Algorithm

1. Check if host is in `hosts allow` list (if defined)
2. Check if host is in `hosts deny` list
3. If in deny list → access Denied (even if in allow list)
4. If allow list exists and host not in it → access Denied
5. Check user authorization (`valid users`, `invalid users`)
6. `invalid users` has priority over `valid users`
7. Apply POSIX file permissions

#### Priority Rules

- **Deny Overrides Allow**: Being in `hosts deny` blocks access even if in `hosts allow`
- **Invalid Users Override**: `invalid users` blocks access even if in `valid users`
- **Localhost Exception**: 127.0.0.1 always allowed unless explicitly denied
- **Global vs Local**: Share-level settings override global settings

#### Best Practices

- Always include 127.0.0.1 in allow list (needed for smbpasswd)
- Use EXCEPT keyword to create exceptions: `hosts allow = 192.168.1. EXCEPT 192.168.1.50`
- Combine IP and user restrictions for defense in depth

**Sources:**
- [Samba securing shares](http://linux-training.be/networking/ch21.html)
- [Samba Restrict File Sharing - nixCraft](https://www.cyberciti.biz/faq/samba-user-network-file-sharing-restictions/)
- [smb.conf manual](https://www.samba.org/samba/docs/current/man-html/smb.conf.5.html)

---

## 4. POSIX Access Control Lists

### Permission Model

POSIX ACLs extend traditional Unix permissions with **access ACLs** and **default ACLs**.

#### Key Features

- **Two ACL Types**:
  - Access ACLs: Control current access to files/directories
  - Default ACLs: Define inherited permissions for new objects
- **ACL Entries**: user::, group::, other::, user:name:, group:name:, mask::
- **Inheritance at Creation**: Permissions inherited only when files/directories created
- **Not Dynamic**: Changes to parent ACLs don't affect existing children

#### Configuration Format

```bash
# Set access ACL
setfacl -m u:john:rwx,g:developers:rx /srv/files

# Set default ACL (inheritance)
setfacl -d -m u:john:rwx,g:developers:rx /srv/files

# View ACLs
getfacl /srv/files
```

#### Inheritance Model

**Critical Difference from Other Systems**: POSIX ACLs are **creation-time only inheritance**.

- **Files**: Inherit default ACL from parent directory as access ACL
- **Directories**: Inherit default ACL both as access ACL and default ACL (cascading)
- **No Retroactive Changes**: Modifying parent ACL doesn't affect existing children
- **Mask Entry**: Limits maximum permissions for named users/groups

#### Permission Algorithm

1. If UID matches file owner → use user:: permissions
2. If UID matches named user entry → use that entry (limited by mask)
3. If GID matches file group or named group entry → use that entry (limited by mask)
4. Otherwise use other:: permissions
5. **Most specific match wins** (not cumulative like some systems)

#### Comparison with NFSv4/Windows ACLs

- **POSIX**: Static inheritance at creation time only
- **NFSv4/Windows**: Dynamic inheritance with propagation flags
- **POSIX**: Simpler model, less overhead
- **NFSv4/Windows**: More flexible, can modify inheritance after creation

**Sources:**
- [POSIX Access Control Lists - SUSE](https://documentation.suse.com/sles/12-SP5/html/SLES-all/cha-security-acls.html)
- [Setting up a Share Using POSIX ACLs - SambaWiki](https://wiki.samba.org/index.php/Setting_up_a_Share_Using_POSIX_ACLs)
- [ACL Inheritance - Oracle](https://docs.oracle.com/cd/E19253-01/819-5461/gbaax/index.html)

---

## 5. Azure Blob Storage - Attribute-Based Access Control (ABAC)

### Permission Model

Azure uses **RBAC with ABAC conditions** for fine-grained path-based access control.

#### Key Features

- **Role Assignment Conditions**: Add conditions to RBAC role assignments
- **Attribute Types**: @Principal, @Resource, @Request, @Environment
- **Path-Based Conditions**: Use blob path as virtual directory attribute
- **Container + Path**: Separate container name from blob path in conditions

#### Configuration Format

```json
{
  "roleDefinitionId": "ba92f5b4-2d11-453d-a403-e96b0029c9fe",
  "principalId": "user-guid",
  "condition": "(
    (
      !(ActionMatches{'Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read'})
    )
    OR
    (
      @Resource[Microsoft.Storage/storageAccounts/blobServices/containers/blobs:path] StringStartsWith 'HRRead/'
    )
  )",
  "conditionVersion": "2.0"
}
```

#### Path Expression Rules

- **No Container Name**: Path shouldn't include container name
- **No Leading Slash**: Use `HRRead/*` not `/HRRead/*`
- **No URL Encoding**: Use plain path characters
- **Virtual Directories**: Blobs use flat namespace; paths are part of blob name

#### Inheritance Model

Azure uses **role-based inheritance** with **condition-based filtering**:

- **Role Assignment**: Inherits from subscription → resource group → storage account
- **Conditions**: Applied at assignment level, filters inherited permissions
- **Path Patterns**: Use StringStartsWith, StringLike for path matching
- **Override**: More specific role assignments override broader ones

#### Authorization Methods

1. **Azure AD + RBAC**: Recommended for production
2. **Shared Key**: Account-level access (all or nothing)
3. **SAS Tokens**: Temporary delegated access with conditions
4. **Conditional Access Policies**: Require disabling Shared Key authorization

#### Built-in Roles

- **Storage Blob Data Owner**: Full control including ACLs
- **Storage Blob Data Contributor**: Read/write/delete blobs
- **Storage Blob Data Reader**: Read-only access
- **Storage Blob Delegator**: Create user delegation SAS tokens

**Sources:**
- [Tutorial: Add role assignment condition - Azure](https://learn.microsoft.com/en-us/azure/storage/blobs/storage-auth-abac-portal)
- [Attribute-Based Access Control For Azure Blob Storage](https://charbelnemnom.com/attribute-based-access-control-for-azure-blob-storage/)
- [Authorize access to blobs using Microsoft Entra ID](https://learn.microsoft.com/en-us/azure/storage/blobs/authorize-access-azure-active-directory)

---

## 6. Web File Manager Applications - Best Practices

### Industry Standards

Research into web-based file managers reveals common patterns and best practices.

#### Core Principles

1. **Principle of Least Privilege**: Grant minimum necessary permissions
2. **Role-Based Access Control (RBAC)**: Assign permissions by role, not individual users
3. **Group-Based Management**: Use groups for scalability
4. **Permission Inheritance**: Leverage cascading from parent folders
5. **Regular Audits**: Periodic review and cleanup of permissions

#### Standard Permission Levels (Linux Web Servers)

```bash
Root folder:        755 (rwxr-xr-x)
Subdirectories:     755 (rwxr-xr-x)
Regular files:      644 (rw-r--r--)
Sensitive configs:  600 or 640 (rw------- or rw-r-----)
Web-writable dirs:  775 (rwxrwxr-x) - apply sparingly
```

#### Permission Hierarchy Best Practices

**Inheritance Pattern:**
- Parent folder sets default permissions
- Child folders inherit unless explicitly overridden
- Explicit permissions always take precedence over inherited
- Most specific path match wins

**Override Pattern:**
```
/files (755, read-only for users)
  /files/shared (775, read-write for users) ← Override
    /files/shared/public (755, read-only) ← Override again
```

#### Multi-Dimensional Access Control

Modern file managers implement 2-3 dimensional access control:

1. **User/Group Dimension**: Who is accessing
2. **Path Dimension**: What resource is being accessed
3. **IP/Network Dimension**: Where access is coming from (optional)
4. **Time Dimension**: When access is allowed (optional)

#### Combining Permission Types

- **Share Permissions**: Network-level access (coarse-grained)
- **Filesystem Permissions**: File-level access (fine-grained)
- **Apply most restrictive**: Intersection of all permission types

**Sources:**
- [Understanding File Server Permissions](https://www.comparitech.com/net-admin/file-server-permissions/)
- [Right folder permissions for a website - Internal Pointers](https://www.internalpointers.com/post/right-folder-permission-website)
- [Share and NTFS Permissions](https://learn.microsoft.com/en-us/iis/web-hosting/configuring-servers-in-the-windows-web-platform/configuring-share-and-ntfs-permissions)

---

## 7. ACL Inheritance & Override Patterns

### Common Inheritance Models

Research identifies three primary inheritance patterns across systems:

#### 1. Monotonic (Cumulative) Inheritance

**Model**: Permissions accumulate from all ancestor ACLs
- Used by: Some Web ACL systems
- **Pros**: Simple to understand, permissions never decrease
- **Cons**: Can't restrict access in subdirectories
- **Algorithm**: Union of all permissions from root to target

```
/root (read)
  /folder (write)          ← Has: read + write
    /subfolder (delete)    ← Has: read + write + delete
```

#### 2. DefaultLocal (Most Specific) Inheritance

**Model**: Use closest parent ACL, ignore others
- Used by: Solid Web ACL, many file systems
- **Pros**: Clear override behavior, predictable
- **Cons**: No permission accumulation
- **Algorithm**: Search from target to root, stop at first ACL

```
/root (read, write)
  /folder (delete)         ← Has: delete only (ignores parent)
    /subfolder (none)      ← Has: delete (inherits from /folder)
```

#### 3. Explicit Override with Inheritance

**Model**: Inherit from parent, but explicit rules override
- Used by: Windows/NTFS, NFSv4, modern systems
- **Pros**: Flexible, supports both inheritance and override
- **Cons**: More complex evaluation logic
- **Algorithm**: Merge inherited + explicit, explicit wins on conflict

```
/root (read, write)
  /folder (inherit + delete)     ← Has: read, write, delete
    /subfolder (inherit + !write) ← Has: read, delete (overrides write)
```

### Windows/NTFS Inheritance Flags

```bash
OI (Object Inherit)    - This folder and files
CI (Container Inherit) - This folder and subfolders
IO (Inherit Only)      - Don't apply to current object
NP (No Propagate)      - Stop inheritance at next level

Examples:
(OI)(CI)     - Full inheritance (folder, subfolders, files)
(OI)(CI)(IO) - Inherit to children only, not current folder
(CI)         - Subfolders only, not files
```

### Override Priority Patterns

Most systems follow this priority order (highest to lowest):

1. **Explicit Deny** - Always blocks access
2. **Explicit Allow** - Grants access if no Deny
3. **Inherited Deny** - Blocks access unless overridden
4. **Inherited Allow** - Grants access unless overridden
5. **Implicit Deny** - Default when no rules match

### Hierarchical RBAC with Inheritance

Many modern systems use role hierarchies:

```
Admin Role
  ├── permissions: [read, write, delete, admin]
  └── Moderator Role (inherits from Admin)
      ├── permissions: [read, write, delete]
      └── User Role (inherits from Moderator)
          └── permissions: [read, write]
```

**Benefits:**
- Simplified administration
- Consistent permission sets
- Easy to modify role definitions
- Natural organizational mapping

**Sources:**
- [Implementing Google Drive Style Hierarchical ACL](https://hasura.io/blog/implementing-a-google-drive-style-hierarchical-role-based-acl-system)
- [Solid ACL Inheritance Specification](https://github.com/solid/solid-spec/blob/master/acl-inheritance.md)
- [How ACL inheritance works - NetApp](https://library.netapp.com/ecmdocs/ECMP1401220/html/GUID-9320A39F-F278-4368-B8AB-561856881E5F.html)

---

## 8. Access Control Design Patterns

### Role-Based Access Control (RBAC)

The dominant pattern in modern systems.

**Core Concepts:**
- **Roles**: Named permission sets (admin, editor, viewer)
- **Users**: Assigned to one or more roles
- **Permissions**: Actions users can perform
- **Resources**: Objects being accessed

**Evaluation:**
```
User → [Roles] → [Permissions] → Action on Resource
```

**Advantages:**
- Reduces admin overhead (manage roles, not individual permissions)
- Scales well with organizational growth
- Supports role hierarchies
- Industry standard, well-understood

### Attribute-Based Access Control (ABAC)

More flexible, context-aware access control.

**Attribute Dimensions:**
- **Subject**: User attributes (department, clearance level, location)
- **Resource**: Object attributes (classification, owner, path)
- **Action**: Operation type (read, write, delete)
- **Environment**: Contextual factors (time, IP address, device)

**Policy Example:**
```
ALLOW if (
  user.department == "HR" AND
  resource.path STARTS_WITH "/hr/" AND
  time.hour BETWEEN 9 AND 17 AND
  request.ip IN company_network
)
```

**Advantages:**
- Highly flexible and expressive
- Supports complex policies
- Reduces explicit permission assignments
- Dynamic evaluation

**Disadvantages:**
- More complex to implement
- Harder to audit
- Performance overhead
- Requires policy language/engine

### Policy-Based Access Control (PBAC)

Centralized policy repository with rule evaluation.

**Components:**
- **Policy Decision Point (PDP)**: Evaluates policies
- **Policy Enforcement Point (PEP)**: Enforces decisions
- **Policy Repository**: Stores policy definitions
- **Policy Administration Point (PAP)**: Manages policies

**Example (XACML-style):**
```xml
<Policy PolicyId="path-based-policy">
  <Target>
    <Resource path="/sensitive/*"/>
  </Target>
  <Rule Effect="Allow">
    <Condition>
      <Apply FunctionId="string-equal">
        <AttributeValue>admin</AttributeValue>
        <AttributeValue DataType="user.role"/>
      </Apply>
    </Condition>
  </Rule>
</Policy>
```

**Sources:**
- [Role-Based Access Control: Five Common Authorization Patterns](https://thenewstack.io/role-based-access-control-five-common-authorization-patterns/)
- [Pattern-Oriented Software Architecture - Access Control](https://transang.me/pattern-oriented-software-architecture-access-control-pattern/)
- [NIST RBAC Models](https://csrc.nist.gov/csrc/media/projects/role-based-access-control/documents/sandhu96.pdf)

---

## 9. FileGator Current Implementation Analysis

### Current Architecture

**User Model** (`backend/Services/Auth/User.php`):
```php
- role: 'guest' | 'user' | 'admin'
- permissions: ['read', 'write', 'upload', 'download', 'batchdownload', 'zip', 'chmod']
- username: string
- homedir: string (single directory path)
- name: string
```

**Current Limitations:**
1. **Global Permissions**: Permissions apply to entire homedir, no per-folder control
2. **Single Home Directory**: Each user has one homedir, can't access multiple paths
3. **No IP Restrictions**: No network-based access control
4. **No Inheritance Model**: No parent/child folder permission relationships
5. **Flat Permission Model**: All-or-nothing for each permission type

**Storage** (`private/users.json.blank`):
```json
{
  "1": {
    "username": "admin",
    "role": "admin",
    "homedir": "/",
    "permissions": "read|write|upload|download|batchdownload|zip|chmod",
    "password": "..."
  }
}
```

### Enhancement Opportunities

1. **Path-Based ACL System**: Add per-folder permission rules
2. **IP-Based Restrictions**: Add network access control
3. **Permission Inheritance**: Implement cascading from parent folders
4. **Multiple Path Access**: Allow users access to multiple folder trees
5. **Override Mechanism**: Support explicit permissions that override inherited ones

---

## 10. Recommended Configuration Schema

### Design Goals

1. **Intuitive for PHP Configuration**: Array-based, easy to read and write
2. **Supports Cascading Inheritance**: Children inherit from parents by default
3. **Explicit Override Capability**: Specific rules override inherited ones
4. **Three-Dimensional Model**: User + IP + Path → Permissions
5. **Simple to Configure and Maintain**: Balance power with usability

### Proposed Schema (PHP Configuration)

```php
<?php

return [
    // User definitions remain mostly unchanged
    'users' => [
        'admin' => [
            'username' => 'admin',
            'name' => 'Administrator',
            'role' => 'admin',
            'password' => '$2y$10$...',
            'global_permissions' => ['read', 'write', 'upload', 'download', 'zip', 'chmod'],
            'ip_allowlist' => ['*'], // * means all IPs
            'ip_denylist' => [],
        ],
        'john' => [
            'username' => 'john',
            'name' => 'John Doe',
            'role' => 'user',
            'password' => '$2y$10$...',
            'global_permissions' => ['read', 'upload', 'download'],
            'ip_allowlist' => ['192.168.1.0/24', '10.0.0.0/16'],
            'ip_denylist' => ['192.168.1.50'],
        ],
    ],

    // New: Path-based ACL rules
    'path_acls' => [
        // Rule evaluation order: most specific path wins
        // Rules are evaluated in order; first match wins within same specificity

        // Global default (applies to all paths)
        '/' => [
            'inherit' => false, // no parent to inherit from
            'rules' => [
                [
                    'users' => ['*'],  // * means all authenticated users
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read'], // everyone can read by default
                    'priority' => 0, // lower number = lower priority
                ],
                [
                    'users' => ['admin'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete', 'zip', 'chmod'],
                    'priority' => 100,
                ],
            ],
        ],

        // Specific path rules
        '/projects' => [
            'inherit' => true, // inherit permissions from parent (/)
            'rules' => [
                [
                    'users' => ['@developers'], // @ prefix for groups/roles
                    'ip_allowlist' => ['192.168.1.0/24'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download'],
                    'priority' => 50,
                    'override_inherited' => false, // merge with inherited permissions
                ],
            ],
        ],

        '/projects/project-alpha' => [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['john', 'jane'],
                    'ip_allowlist' => ['*'],
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 75,
                    'override_inherited' => true, // this rule replaces inherited permissions
                ],
                [
                    'users' => ['@contractors'],
                    'ip_allowlist' => ['10.0.0.0/16'],
                    'ip_denylist' => [],
                    'permissions' => ['read'], // read-only for contractors
                    'priority' => 70,
                    'override_inherited' => true,
                ],
            ],
        ],

        '/hr/confidential' => [
            'inherit' => false, // don't inherit from /hr
            'rules' => [
                [
                    'users' => ['admin', '@hr-managers'],
                    'ip_allowlist' => ['192.168.1.0/24'], // only from corporate network
                    'ip_denylist' => [],
                    'permissions' => ['read', 'write', 'upload', 'download', 'delete'],
                    'priority' => 100,
                    'override_inherited' => true,
                ],
            ],
        ],
    ],

    // Group/role definitions for convenience
    'groups' => [
        'developers' => ['john', 'jane', 'bob'],
        'contractors' => ['alice', 'charlie'],
        'hr-managers' => ['susan', 'tom'],
    ],

    // ACL evaluation settings
    'acl_settings' => [
        'evaluation_mode' => 'most_specific_wins', // or 'highest_priority_wins', 'first_match'
        'default_inherit' => true, // default value for 'inherit' if not specified
        'cache_permissions' => true, // cache evaluated permissions for performance
        'cache_ttl' => 300, // 5 minutes
        'deny_overrides_allow' => true, // if user has both allow and deny for same path, deny wins
    ],
];
```

### Alternative: JSON Storage Format

For compatibility with current `users.json` approach:

```json
{
  "users": {
    "admin": {
      "username": "admin",
      "role": "admin",
      "global_permissions": ["read", "write", "upload", "download", "zip", "chmod"],
      "ip_allowlist": ["*"],
      "ip_denylist": []
    }
  },
  "path_acls": {
    "/": {
      "inherit": false,
      "rules": [
        {
          "users": ["*"],
          "ip_allowlist": ["*"],
          "permissions": ["read"],
          "priority": 0
        }
      ]
    },
    "/projects": {
      "inherit": true,
      "rules": [
        {
          "users": ["@developers"],
          "ip_allowlist": ["192.168.1.0/24"],
          "permissions": ["read", "write", "upload", "download"],
          "priority": 50
        }
      ]
    }
  },
  "groups": {
    "developers": ["john", "jane"],
    "contractors": ["alice"]
  }
}
```

### Evaluation Algorithm

```
function checkPermission(user, ip, path, requested_permission):
    # Step 1: Find all matching ACL rules
    matching_rules = findMatchingRules(user, ip, path)

    # Step 2: Check for explicit deny
    for rule in matching_rules:
        if ip in rule.ip_denylist:
            return DENY

    # Step 3: Evaluate rules by specificity and priority
    sorted_rules = sortBySpecificityAndPriority(matching_rules)

    permissions_granted = []
    for rule in sorted_rules:
        if rule.override_inherited:
            permissions_granted = rule.permissions
            break
        else:
            permissions_granted.merge(rule.permissions)

    # Step 4: Check if requested permission is granted
    if requested_permission in permissions_granted:
        return ALLOW
    else:
        return DENY

function findMatchingRules(user, ip, path):
    rules = []
    current_path = path

    # Traverse from specific path to root
    while current_path != null:
        if acl_exists_for(current_path):
            acl = get_acl(current_path)

            for rule in acl.rules:
                if user_matches(user, rule.users) and ip_matches(ip, rule):
                    rule.specificity = path_depth(current_path)
                    rules.append(rule)

            if not acl.inherit:
                break  # Stop inheritance

        current_path = parent_path(current_path)

    return rules

function sortBySpecificityAndPriority(rules):
    # Sort by:
    # 1. Path specificity (more specific = higher, e.g., /a/b/c > /a/b > /a)
    # 2. Priority value (higher = higher)
    # 3. Rule order (earlier = higher)
    return sorted(rules, key=lambda r: (r.specificity, r.priority, -r.order), reverse=True)
```

### Key Design Decisions

#### 1. Specificity-Based Evaluation

**Most specific path wins** - similar to CSS cascade:
- `/hr/confidential/file.txt` rules override `/hr` rules
- `/hr` rules override `/` rules
- Within same path, priority value determines order

**Rationale**: Intuitive for administrators, matches mental model

#### 2. Inheritance with Override

**Default inheritance enabled** - reduces configuration duplication:
- Child paths inherit parent permissions by default
- `inherit: false` stops inheritance chain
- `override_inherited: true` replaces inherited permissions

**Rationale**: Balance between DRY principle and explicit control

#### 3. Three-Dimensional Matching

**User AND IP AND Path must all match**:
- Rule only applies if user matches (username or group)
- AND IP address matches (allowlist and not in denylist)
- AND path matches (exact or ancestor)

**Rationale**: Defense in depth, supports complex security requirements

#### 4. Deny Overrides Allow

**Explicit deny always wins**:
- If IP in denylist → immediate deny
- If user has conflicting rules → deny wins

**Rationale**: Security-first approach, matches industry standards

#### 5. Wildcard Support

**Flexible matching**:
- `*` for users = all authenticated users
- `*` for IP = all IP addresses
- `@groupname` for group/role references
- CIDR notation for IP ranges

**Rationale**: Reduces configuration verbosity

#### 6. Performance Optimization

**Caching strategy**:
- Cache evaluated permissions per (user, ip, path) tuple
- TTL-based expiration
- Invalidate on ACL configuration changes

**Rationale**: Path traversal and rule evaluation can be expensive

---

## 11. Implementation Recommendations

### Phase 1: Backward-Compatible Extension

**Goal**: Add path-based ACLs without breaking existing functionality

1. **Extend User Model**:
   - Add optional `ip_allowlist` and `ip_denylist` arrays
   - Keep existing `permissions` as global default
   - Add optional `path_permissions` array for per-path rules

2. **Add ACL Service**:
   - Create `PathACL` service for rule evaluation
   - Integrate with existing `AuthInterface`
   - Fall back to global permissions if no path rules defined

3. **Migration Path**:
   - Existing users continue to work with global permissions
   - New users can opt-in to path-based ACLs
   - Provide migration script to convert global to path-based

### Phase 2: Full Path-Based ACL System

**Goal**: Implement complete three-dimensional access control

1. **Configuration Files**:
   - `private/users.json` for user definitions
   - `private/path_acls.json` for path rules (or combine into users.json)
   - `private/groups.json` for group definitions

2. **ACL Evaluation Engine**:
   - Path matching with specificity calculation
   - Inheritance resolution
   - Priority-based rule sorting
   - Permission merging logic

3. **Caching Layer**:
   - In-memory cache for evaluated permissions
   - Cache invalidation on configuration changes
   - Optional Redis/Memcached backend

4. **Admin Interface**:
   - UI for managing path-based rules
   - Visual inheritance display
   - Permission testing tool (simulate access for user/IP/path)

### Phase 3: Advanced Features

1. **Time-Based Restrictions**: Add temporal dimension (business hours, etc.)
2. **Audit Logging**: Track all permission checks and denials
3. **Policy Templates**: Pre-defined ACL patterns for common scenarios
4. **Bulk Operations**: Apply permissions to multiple paths at once
5. **Permission Inheritance Visualization**: Graph/tree view of effective permissions

### Testing Strategy

1. **Unit Tests**: Test individual rule matching, inheritance, priority sorting
2. **Integration Tests**: Test full permission evaluation flow
3. **Performance Tests**: Benchmark ACL evaluation with 1000+ rules
4. **Security Tests**: Verify deny overrides, IP restrictions, path traversal protection

### Documentation Needs

1. **Administrator Guide**: How to configure path-based ACLs
2. **Migration Guide**: Moving from global to path-based permissions
3. **API Reference**: PathACL service methods and interfaces
4. **Examples**: Common ACL configurations (department folders, project spaces, etc.)

---

## 12. Comparison Matrix

| System | Inheritance Model | Override Support | Priority Rules | IP Support | Path Wildcards | Ease of Config |
|--------|-------------------|------------------|----------------|------------|----------------|----------------|
| **AWS S3** | Explicit only | Specific > Generic | Deny > Allow | Via Conditions | Yes (* ?) | Medium |
| **NFS** | No inheritance | First match wins | First match | Native | Limited | Low |
| **Samba/CIFS** | Global > Share | Share overrides | Deny > Allow | Native (hosts) | Domain wildcards | Medium |
| **POSIX ACL** | Creation-time | Static after creation | Most specific | No | No | High |
| **Azure Blob** | RBAC hierarchy | Condition filtering | Explicit > Inherit | Via Conditions | StringLike | Medium |
| **Proposed** | Dynamic cascade | Explicit override | Specificity + Priority | Native + CIDR | Path matching | High |

---

## 13. Security Considerations

### Path Traversal Protection

- **Normalize Paths**: Always canonicalize paths before ACL lookup
- **Prevent ../**: Block parent directory references in path matching
- **Symlink Handling**: Resolve symlinks or explicitly block them
- **Case Sensitivity**: Define and enforce path case handling

### IP Spoofing Mitigation

- **Trust Reverse Proxy Headers**: Carefully validate X-Forwarded-For
- **Deny Unknown IPs**: Default deny for IP-based rules when IP detection fails
- **Multiple IP Checks**: Consider both direct connection and proxy headers

### Privilege Escalation Prevention

- **Immutable Admin**: Prevent modification of admin user ACLs via API
- **Self-Modification Limits**: Users can't grant themselves higher permissions
- **Audit Critical Changes**: Log all ACL modifications, especially privilege grants

### Performance Security

- **Rate Limiting**: Limit ACL evaluation requests to prevent DoS
- **Cache Poisoning**: Ensure cache keys include all security dimensions
- **Complexity Limits**: Limit number of rules per path, inheritance depth

---

## 14. Conclusion

### Recommended Approach

Based on industry research and FileGator's architecture, I recommend:

1. **Hybrid RBAC + Path-Based ACL Model**:
   - Roles provide base permissions (like current system)
   - Path ACLs add fine-grained control (new capability)
   - Combine both for effective permissions

2. **Explicit Override with Inheritance**:
   - Children inherit from parents by default
   - Explicit rules override inherited ones
   - Most specific path wins on conflict

3. **Three-Dimensional Evaluation**:
   - User/Group + IP Address + Path → Permissions
   - All three dimensions must match for rule to apply
   - Deny overrides allow for security

4. **PHP Array Configuration**:
   - Intuitive nested array structure
   - Optional JSON format for compatibility
   - Group definitions for convenience

5. **Performance-Conscious Design**:
   - Cache evaluated permissions
   - Optimize path matching algorithm
   - Limit rule complexity

### Expected Benefits

- **Flexibility**: Support complex organizational structures
- **Security**: Defense in depth with user + IP + path controls
- **Usability**: Intuitive configuration with inheritance reducing duplication
- **Scalability**: Efficient evaluation even with many rules
- **Maintainability**: Clear precedence rules and override behavior

### Next Steps

1. Review proposed schema with stakeholders
2. Create prototype ACL evaluation engine
3. Benchmark performance with realistic rule sets
4. Develop migration strategy for existing users
5. Implement with comprehensive testing
6. Document configuration patterns and examples

---

## References

### AWS S3
- [IAM JSON policy elements: Resource](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_resource.html)
- [Examples of Amazon S3 bucket policies](https://docs.aws.amazon.com/AmazonS3/latest/userguide/example-bucket-policies.html)
- [AWS IAM Policies: Multiple Wildcards Explained](https://www.codestudy.net/blog/aws-iam-can-you-use-multiple-wildcards-in-a-value/)

### NFS
- [The /etc/exports Configuration File - Red Hat](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/5/html/deployment_guide/s1-nfs-server-config-exports)
- [NFS exports options examples - GoLinuxCloud](https://www.golinuxcloud.com/nfs-exports-options-examples/)
- [Understanding the /etc/exports File](https://www.thegeekdiary.com/understanding-the-etc-exports-file/)

### Samba/CIFS
- [Samba securing shares](http://linux-training.be/networking/ch21.html)
- [Samba Restrict File Sharing - nixCraft](https://www.cyberciti.biz/faq/samba-user-network-file-sharing-restictions/)
- [smb.conf manual](https://www.samba.org/samba/docs/current/man-html/smb.conf.5.html)

### POSIX ACLs
- [POSIX Access Control Lists - SUSE](https://documentation.suse.com/sles/12-SP5/html/SLES-all/cha-security-acls.html)
- [Setting up a Share Using POSIX ACLs - SambaWiki](https://wiki.samba.org/index.php/Setting_up_a_Share_Using_POSIX_ACLs)
- [ACL Inheritance - Oracle](https://docs.oracle.com/cd/E19253-01/819-5461/gbaax/index.html)

### Azure Blob Storage
- [Tutorial: Add role assignment condition - Azure](https://learn.microsoft.com/en-us/azure/storage/blobs/storage-auth-abac-portal)
- [Attribute-Based Access Control For Azure Blob Storage](https://charbelnemnom.com/attribute-based-access-control-for-azure-blob-storage/)
- [Authorize access to blobs using Microsoft Entra ID](https://learn.microsoft.com/en-us/azure/storage/blobs/authorize-access-azure-active-directory)

### Best Practices
- [Understanding File Server Permissions](https://www.comparitech.com/net-admin/file-server-permissions/)
- [Right folder permissions for a website - Internal Pointers](https://www.internalpointers.com/post/right-folder-permission-website)
- [Share and NTFS Permissions](https://learn.microsoft.com/en-us/iis/web-hosting/configuring-servers-in-the-windows-web-platform/configuring-share-and-ntfs-permissions)

### ACL Patterns
- [Implementing Google Drive Style Hierarchical ACL](https://hasura.io/blog/implementing-a-google-drive-style-hierarchical-role-based-acl-system)
- [Solid ACL Inheritance Specification](https://github.com/solid/solid-spec/blob/master/acl-inheritance.md)
- [How ACL inheritance works - NetApp](https://library.netapp.com/ecmdocs/ECMP1401220/html/GUID-9320A39F-F278-4368-B8AB-561856881E5F.html)

### Design Patterns
- [Role-Based Access Control: Five Common Authorization Patterns](https://thenewstack.io/role-based-access-control-five-common-authorization-patterns/)
- [Pattern-Oriented Software Architecture - Access Control](https://transang.me/pattern-oriented-software-architecture-access-control-pattern/)
- [NIST RBAC Models](https://csrc.nist.gov/csrc/media/projects/role-based-access-control/documents/sandhu96.pdf)

---

**Document Version:** 1.0
**Last Updated:** 2025-12-09
**Author:** Research Agent
**Project:** FileGator Path-Based ACL Enhancement
