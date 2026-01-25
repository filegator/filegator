# Available Hooks

This document details all available hooks, their trigger points, and the data passed to hook scripts.

## File Operation Hooks

### onUpload

Triggered when a file upload completes successfully.

**When**: After all chunks are assembled and the file is stored in the repository.

**Data provided**:
```php
$hookData = [
    'file_path' => '/documents/file.txt',  // Destination path in repository
    'file_name' => 'file.txt',             // Original filename
    'file_size' => 1048576,                // File size in bytes
    'user' => 'john',                      // Username who uploaded
    'home_dir' => '/users/john',           // User's home directory
];
```

**Use cases**:
- Virus scanning uploaded files
- Validating file contents
- Processing/converting uploaded files
- Notifying administrators
- Creating thumbnails for images
- Indexing file content for search

**Example**:
```php
<?php
// Log upload to external service
$logEntry = sprintf(
    "[%s] File uploaded: %s by %s (%d bytes)",
    date('Y-m-d H:i:s'),
    $hookData['file_name'],
    $hookData['user'],
    $hookData['file_size']
);

file_put_contents('/var/log/uploads.log', $logEntry . "\n", FILE_APPEND);

return ['status' => 'logged'];
```

---

### onDownload

Triggered when a file download is initiated.

**When**: Before the file stream is sent to the client.

**Data provided** (single file download):
```php
$hookData = [
    'file_path' => '/documents/file.txt',  // Path being downloaded
    'file_name' => 'file.txt',             // Filename
    'file_size' => 1048576,                // File size in bytes
    'user' => 'john',                      // Username downloading
];
```

**Data provided** (batch download):
```php
$hookData = [
    'file_path' => '/documents/folder',    // Path being downloaded
    'file_name' => 'folder',               // Item name
    'type' => 'dir',                       // 'file' or 'dir'
    'batch_download' => true,              // Indicates batch download
    'user' => 'john',                      // Username downloading
];
```

**Use cases**:
- Download tracking/analytics
- Access logging for compliance
- Bandwidth throttling
- Checking download permissions

---

### onDelete

Triggered after a file or directory is deleted.

**When**: After the delete operation completes successfully.

**Data provided**:
```php
$hookData = [
    'file_path' => '/documents/file.txt',  // Full path of deleted item
    'file_name' => 'file.txt',             // Name of file/folder
    'type' => 'file',                      // 'file' or 'dir'
    'user' => 'john',                      // Username performing deletion
];
```

**Use cases**:
- Audit logging
- Cleaning up related resources
- Sync with external systems
- Notification of deletions

**Example**:
```php
<?php
// Log deletion
$logEntry = sprintf(
    "[%s] DELETED: %s '%s' by user '%s'",
    date('Y-m-d H:i:s'),
    $hookData['type'] === 'dir' ? 'Directory' : 'File',
    $hookData['file_path'],
    $hookData['user']
);

file_put_contents('/var/log/deletions.log', $logEntry . "\n", FILE_APPEND);

return ['logged' => true];
```

---

### onCreate

Triggered when a new file or directory is created.

**When**: After the file/directory creation completes.

**Data provided**:
```php
$hookData = [
    'file_path' => '/documents/newfile.txt',  // Full path of created item
    'file_name' => 'newfile.txt',             // Created item name
    'type' => 'file',                         // 'file' or 'dir'
    'user' => 'john',                         // Username
];
```

**Use cases**:
- Audit logging
- Setting default permissions
- Triggering workflows
- Notification of new content

---

### onRename

Triggered when a file or directory is renamed.

**When**: After the rename operation completes.

**Data provided**:
```php
$hookData = [
    'old_path' => '/documents/oldname.txt',   // Original full path
    'new_path' => '/documents/newname.txt',   // New full path
    'old_name' => 'oldname.txt',              // Original name
    'new_name' => 'newname.txt',              // New name
    'directory' => '/documents',              // Parent directory
    'user' => 'john',                         // Username
];
```

**Use cases**:
- Audit logging
- Updating references in databases
- Syncing with external systems

---

### onMove

Triggered when a file or directory is moved.

**When**: After the move operation completes.

**Data provided**:
```php
$hookData = [
    'source_path' => '/inbox/file.txt',       // Original path
    'destination_path' => '/archive/file.txt', // New path
    'file_name' => 'file.txt',                // Item name
    'type' => 'file',                         // 'file' or 'dir'
    'user' => 'john',                         // Username
];
```

**Use cases**:
- Audit logging
- Workflow triggers
- Sync with external systems

---

### onCopy

Triggered when a file or directory is copied.

**When**: After the copy operation completes.

**Data provided**:
```php
$hookData = [
    'source_path' => '/documents/file.txt',   // Source path
    'destination' => '/backup',               // Destination directory
    'file_name' => 'file.txt',                // Item name
    'type' => 'file',                         // 'file' or 'dir'
    'user' => 'john',                         // Username
];
```

**Use cases**:
- Audit logging
- Tracking disk usage
- Triggering backup workflows

---

## Authentication Hooks

### onLogin

Triggered when a user successfully logs in.

**When**: After credentials are verified and session is created.

**Data provided**:
```php
$hookData = [
    'username' => 'john',                     // Username
    'ip_address' => '192.168.1.100',          // Client IP
    'home_dir' => '/users/john',              // User's home directory
    'role' => 'admin',                        // User's role
];
```

**Use cases**:
- Login notifications
- Security monitoring
- Session tracking
- Two-factor authentication triggers
- IP-based access logging

**Example**:
```php
<?php
// Security logging
$logEntry = sprintf(
    "[%s] LOGIN: User '%s' (role: %s) from IP %s",
    date('Y-m-d H:i:s'),
    $hookData['username'],
    $hookData['role'],
    $hookData['ip_address']
);

file_put_contents('/var/log/auth.log', $logEntry . "\n", FILE_APPEND);

// Check for suspicious login patterns
if (in_array($hookData['ip_address'], ['10.0.0.1', '10.0.0.2'])) {
    // Send alert notification
    mail('admin@example.com', 'Security Alert', 'Login from monitored IP');
}

return true;
```

---

### onLogout

Triggered when a user logs out.

**When**: Before the logout action completes.

**Data provided**:
```php
$hookData = [
    'username' => 'john',                     // Username
    'ip_address' => '192.168.1.100',          // Client IP
];
```

**Use cases**:
- Session tracking
- Audit logging
- Analytics

---

## Hook Return Values

Hooks can return different values to control execution:

| Return Value | Behavior |
|--------------|----------|
| `true` or `null` | Success, continue to next hook |
| `false` | Stop hook chain execution |
| `array` | Custom result data (continues execution) |
| `['action' => 'stop']` | Stop hook chain execution |
| `['action' => 'continue']` | Continue to next hook |

**Example - Stopping execution**:
```php
<?php
// Prevent operation on protected files
if (strpos($hookData['file_path'], '/protected/') === 0) {
    return false; // Stops further hooks
}

return true;
```

**Example - Returning data**:
```php
<?php
return [
    'action' => 'continue',
    'status' => 'processed',
    'scan_result' => 'clean',
    'processed_at' => date('Y-m-d H:i:s'),
];
```

## Summary Table

| Hook | Trigger Event | Key Data Fields |
|------|---------------|-----------------|
| `onUpload` | File upload complete | `file_path`, `file_name`, `file_size`, `user`, `home_dir` |
| `onDownload` | File download starts | `file_path`, `file_name`, `file_size`, `user`, `batch_download` |
| `onDelete` | File/dir deleted | `file_path`, `file_name`, `type`, `user` |
| `onCreate` | File/dir created | `file_path`, `file_name`, `type`, `user` |
| `onRename` | File/dir renamed | `old_path`, `new_path`, `old_name`, `new_name`, `directory`, `user` |
| `onMove` | File/dir moved | `source_path`, `destination_path`, `file_name`, `type`, `user` |
| `onCopy` | File/dir copied | `source_path`, `destination`, `file_name`, `type`, `user` |
| `onLogin` | User logs in | `username`, `ip_address`, `home_dir`, `role` |
| `onLogout` | User logs out | `username`, `ip_address` |
